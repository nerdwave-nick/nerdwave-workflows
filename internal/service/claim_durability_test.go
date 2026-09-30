package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/store"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func disconnectClaimTest(t *testing.T, s *Server, c protocol.Client) int {
	t.Helper()
	r := httptest.NewRequest("POST", "/v1/disconnect", bytes.NewBufferString("{}"))
	r.Header.Set("X-Lit-Client-ID", c.ClientID)
	r.Header.Set("If-Match", fmt.Sprintf("\"client:%d\"", c.StateRevision))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w.Code
}
func TestClaimRestartKeepsExpiryAndCorruptionFailsClosed(t *testing.T) {
	for _, damage := range []string{"none", "missing-claim", "unknown-version", "invalid-token", "missing-owner", "unknown-field"} {
		t.Run(damage, func(t *testing.T) {
			s, c, id := claimFixture(t)
			out, e := s.ApplyClaims(protocol.ClaimOperation{Operation: "claims.acquire", OwnerClientID: c.ClientID, Selection: "explicit", Items: []protocol.ClaimItem{{IssueID: id}}}, c)
			if e != nil {
				t.Fatal(e)
			}
			saved := out.Items[0].Claim
			root := s.Store.Root
			raw, e := s.Store.Read(claimPath(id))
			if e != nil {
				t.Fatal(e)
			}
			s.Store.Close()
			if damage != "none" {
				var v map[string]any
				json.Unmarshal(raw, &v)
				switch damage {
				case "missing-claim":
					delete(v, "claim")
				case "unknown-version":
					v["schema_version"] = 2
				case "invalid-token":
					v["claim"].(map[string]any)["token"] = "bad"
				case "missing-owner":
					v["claim"].(map[string]any)["owner_client_id"] = protocol.UUID()
				case "unknown-field":
					v["unknown"] = true
				}
				raw, _ = json.Marshal(v)
				if e = os.WriteFile(filepath.Join(root, claimPath(id)), raw, 0600); e != nil {
					t.Fatal(e)
				}
			}
			st, e := store.Open(root)
			if e != nil {
				t.Fatal(e)
			}
			defer st.Close()
			reopened, e := New(st, s.Config)
			if damage != "none" {
				if e == nil {
					t.Fatal("corrupt claim accepted")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			got, e := reopened.LiveClaim(id)
			if e != nil || !same(got, saved) {
				t.Fatal(got, e)
			}
		})
	}
}
func TestClaimDisconnectJournalAtomicAcrossLiveAndExpired(t *testing.T) {
	for _, boundary := range []string{"after_journal_directory", "after_payload", "before_commit", "after_commit", "after_write:1", "after_write:2", "after_write:3", "cleanup_after_marker", "cleanup_after_payload"} {
		t.Run(boundary, func(t *testing.T) {
			s, c, id := claimFixture(t)
			issue, _ := s.Issue(id)
			second := protocol.UUID()
			executeRecordTest(t, s, c, prepareRecordTest(t, s, c, issue.ProjectID, "issue.create", protocol.ProjectInput{ID: second, Title: textPointer("Second")}))
			other := c
			other.ClientID = protocol.UUID()
			if e := s.SaveClient(other); e != nil {
				t.Fatal(e)
			}
			third := protocol.UUID()
			executeRecordTest(t, s, c, prepareRecordTest(t, s, c, issue.ProjectID, "issue.create", protocol.ProjectInput{ID: third, Title: textPointer("Other owner")}))
			held, e := s.ApplyClaims(protocol.ClaimOperation{Operation: "claims.acquire", OwnerClientID: other.ClientID, Selection: "explicit", Items: []protocol.ClaimItem{{IssueID: third}}}, other)
			if e != nil {
				t.Fatal(e)
			}
			otherClaim := held.Items[0].Claim
			// One owner has a live and an expired record; another owns separate state.
			now := time.Now().UTC()
			s.claimClock = func() time.Time { return now }
			_, e = s.ApplyClaims(protocol.ClaimOperation{Operation: "claims.acquire", OwnerClientID: c.ClientID, Selection: "explicit", Items: []protocol.ClaimItem{{IssueID: id, ExtendTo: now.Add(time.Minute).Format(time.RFC3339Nano)}, {IssueID: second}}}, c)
			if e != nil {
				t.Fatal(e)
			}
			now = now.Add(2 * time.Minute)
			root := s.Store.Root
			fired := false
			s.Store.Fault = func(point string) error {
				if point == boundary {
					fired = true
					return fmt.Errorf("interrupt")
				}
				return nil
			}
			if status := disconnectClaimTest(t, s, c); status != 503 || !fired {
				t.Fatal(status, fired)
			}
			s.Store.Close()
			committed := boundary != "after_journal_directory" && boundary != "after_payload" && boundary != "before_commit"
			var prior []byte
			for restart := 0; restart < 2; restart++ {
				st, e := store.Open(root)
				if e != nil {
					t.Fatal(e)
				}
				v, e := New(st, s.Config)
				if e != nil {
					st.Close()
					t.Fatal(e)
				}
				v.claimClock = func() time.Time { return now }
				got, e := v.Client(c.ClientID)
				if e != nil {
					t.Fatal(e)
				}
				if (got.Status == "disconnected") != committed {
					t.Fatal(got)
				}
				for _, iid := range []string{id, second} {
					claim, e := v.readClaim(iid)
					if e != nil {
						t.Fatal(e)
					}
					if (claim == nil) != committed {
						t.Fatal("partial ownership release", claim)
					}
				}
				retained, e := v.LiveClaim(third)
				if e != nil || !same(retained, otherClaim) {
					t.Fatal("other ownership changed", retained, e)
				}
				untouched, e := v.Client(other.ClientID)
				if e != nil || untouched.Status != "connected" || untouched.StateRevision != other.StateRevision {
					t.Fatal(untouched, e)
				}
				raw, e := st.Read(claimPath(second))
				if e != nil {
					t.Fatal(e)
				}
				if restart == 1 && !bytes.Equal(prior, raw) {
					t.Fatal("replayed bytes changed")
				}
				prior = raw
				st.Close()
			}
		})
	}
}
func TestClaimCloseReleaseAtomic(t *testing.T) {
	for _, boundary := range []string{"before_commit", "after_commit", "after_write:1", "after_write:2", "after_write:3", "after_write:4", "after_write:5", "cleanup_after_marker"} {
		t.Run(boundary, func(t *testing.T) {
			s, c, id := claimFixture(t)
			_, e := s.ApplyClaims(protocol.ClaimOperation{Operation: "claims.acquire", OwnerClientID: c.ClientID, Selection: "explicit", Items: []protocol.ClaimItem{{IssueID: id}}}, c)
			if e != nil {
				t.Fatal(e)
			}
			issue, _ := s.Issue(id)
			plan := prepareRecordTest(t, s, c, issue.ProjectID, "issue.close", protocol.ProjectInput{Target: id})
			root := s.Store.Root
			fired := false
			s.Store.Fault = func(point string) error {
				if point == boundary {
					fired = true
					return fmt.Errorf("interrupt")
				}
				return nil
			}
			status, _ := executeProjectTest(t, s, c, plan)
			if status != 503 || !fired {
				t.Fatal(status, fired)
			}
			s.Store.Close()
			st, e := store.Open(root)
			if e != nil {
				t.Fatal(e)
			}
			defer st.Close()
			v, e := New(st, s.Config)
			if e != nil {
				t.Fatal(e)
			}
			got, e := v.Issue(id)
			if e != nil {
				t.Fatal(e)
			}
			claim, e := v.LiveClaim(id)
			if e != nil {
				t.Fatal(e)
			}
			committed := boundary != "before_commit"
			if (got.State == "closed") != committed || (claim == nil) != committed {
				t.Fatal(got, claim)
			}
		})
	}
}

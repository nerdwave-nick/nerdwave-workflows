package service

import (
	"bytes"
	"fmt"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/store"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Exercise fixed prepared bytes across every write in the remaining initial
// mutation families, supplementing the existing creation/link/close matrices.
func TestRemainingInitialMutationRecovery(t *testing.T) {
	for _, kind := range []string{"issue-update", "issue-parent", "issue-reopen", "comment-update", "unlink", "claim-acquire", "claim-renew", "claim-release", "client-patch", "client-resume"} {
		t.Run(kind, func(t *testing.T) {
			for _, boundary := range []string{"before_commit", "after_commit", "every_write", "cleanup_after_marker", "cleanup_after_payload"} {
				t.Run(boundary, func(t *testing.T) {
					// every_write expands adaptively after the exact operation is prepared.
					max := 1
					for position := 1; position <= max; position++ {
						root := t.TempDir()
						s, c := projectTestServer(t, root)
						pid, parent, id, comment := protocol.UUID(), protocol.UUID(), protocol.UUID(), protocol.UUID()
						executeProjectTest(t, s, c, prepareProjectTest(t, s, c, "create", protocol.ProjectInput{ID: pid, Title: textPointer("feat/replay")}))
						executeRecordTest(t, s, c, prepareRecordTest(t, s, c, pid, "issue.create", protocol.ProjectInput{ID: parent, Title: textPointer("Parent")}, protocol.ProjectInput{ID: id, Title: textPointer("Child")}))
						executeRecordTest(t, s, c, prepareRecordTest(t, s, c, pid, "comment.create", protocol.ProjectInput{ID: comment, Issue: id, Content: textPointer("before\r\n雪")}))
						var writes []store.Write
						var e error
						switch kind {
						case "claim-acquire", "claim-renew", "claim-release":
							op := protocol.ClaimOperation{Operation: "claims.acquire", OwnerClientID: c.ClientID, Selection: "explicit", Items: []protocol.ClaimItem{{IssueID: id}, {IssueID: parent}}}
							if kind != "claim-acquire" {
								r, err := s.ApplyClaims(op, c)
								if err != nil {
									t.Fatal(err)
								}
								for n := range op.Items {
									op.Items[n].Token = r.Items[n].Claim.Token
									op.Items[n].ExtendTo = time.Now().UTC().Add(50 * time.Minute).Format(time.RFC3339Nano)
								}
								op.Operation = "claims.renew"
								if kind == "claim-release" {
									op.Operation = "claims.release"
									for n := range op.Items {
										op.Items[n].ExtendTo = ""
									}
								}
							}
							writes, _, e = s.claimTransaction(op, c)
						case "client-patch", "client-resume":
							if kind == "client-resume" {
								c.Status = "disconnected"
								c.StateRevision++
								s.SaveClient(c)
								c.Status = "connected"
							} else {
								c.Actor.Name = "Updated"
							}
							c.StateRevision++
							writes = []store.Write{store.JSONWrite("clients/"+c.ClientID+".json", c)}
						default:
							operation := "issue.update"
							input := protocol.ProjectInput{Target: id, Set: protocol.ProjectSet{Body: textPointer("after\r\n雪")}}
							switch kind {
							case "issue-parent":
								input.Set = protocol.ProjectSet{ParentID: nullable(&parent)}
							case "issue-reopen":
								executeRecordTest(t, s, c, prepareRecordTest(t, s, c, pid, "issue.close", protocol.ProjectInput{Target: id}))
								operation = "issue.reopen"
								input.Set = protocol.ProjectSet{}
							case "comment-update":
								operation = "comment.update"
								input.Target = comment
							case "unlink":
								link := protocol.ProjectInput{From: id, To: []string{parent}, Relation: "related"}
								executeRecordTest(t, s, c, prepareRecordTest(t, s, c, pid, "issue.link", link))
								operation = "issue.unlink"
								input = link
							}
							plan := prepareRecordTest(t, s, c, pid, operation, input)
							writes, _, e = s.recordTransaction(plan.Intent, plan.RequestHash, c)
						}
						if e != nil || len(writes) == 0 {
							t.Fatal(kind, e, len(writes))
						}
						point := boundary
						if boundary == "every_write" {
							max = len(writes)
							point = fmt.Sprintf("after_write:%d", position)
						}
						before := map[string][]byte{}
						for _, w := range writes {
							b, err := os.ReadFile(filepath.Join(root, w.Path))
							if err != nil && !os.IsNotExist(err) {
								t.Fatal(err)
							}
							before[w.Path] = b
						}
						fired := false
						s.Store.Fault = func(p string) error {
							if p == point {
								fired = true
								return fmt.Errorf("interrupt at %s", p)
							}
							return nil
						}
						if e = s.Store.Commit(writes); e == nil || !fired {
							t.Fatal("boundary not exercised", point, e)
						}
						identity := s.Store.Identity.ServiceID
						s.Store.Close()
						for restart := 0; restart < 2; restart++ {
							st, err := store.Open(root)
							if err != nil {
								t.Fatal(err)
							}
							if _, err = New(st, s.Config); err != nil {
								st.Close()
								t.Fatal(err)
							}
							if st.Identity.ServiceID != identity {
								t.Fatal("identity changed")
							}
							for _, w := range writes {
								got, err := os.ReadFile(filepath.Join(root, w.Path))
								want := w.Data
								if boundary == "before_commit" {
									want = before[w.Path]
								}
								if want == nil {
									if !os.IsNotExist(err) {
										t.Fatal("uncommitted file exists", w.Path, err)
									}
								} else if err != nil || !bytes.Equal(got, want) {
									t.Fatalf("%s restart %d changed fixed bytes %s: %v", kind, restart, w.Path, err)
								}
							}
							st.Close()
						}
					}
				})
			}
		})
	}
}
func TestCorruptLinkedJournalPreservesExactEvidence(t *testing.T) {
	root := t.TempDir()
	s, c := projectTestServer(t, root)
	p := prepareProjectTest(t, s, c, "create", protocol.ProjectInput{ID: protocol.UUID(), Title: textPointer("feat/a")}, protocol.ProjectInput{ID: protocol.UUID(), Title: textPointer("feat/b")})
	writes, _, e := s.recordTransaction(p.Intent, p.RequestHash, c)
	if e != nil {
		t.Fatal(e)
	}
	s.Store.Fault = func(p string) error {
		if p == "after_commit" {
			return fmt.Errorf("interrupted")
		}
		return nil
	}
	s.Store.Commit(writes)
	s.Store.Close()
	path := filepath.Join(root, "journal", "pending.json")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	raw = append(raw, ' ')
	os.WriteFile(path, raw, 0600)
	marker, _ := os.ReadFile(filepath.Join(root, "journal", "commit.json"))
	identity, _ := os.ReadFile(filepath.Join(root, "service.json"))
	for attempt := 0; attempt < 2; attempt++ {
		if st, e := store.Open(root); e == nil {
			st.Close()
			t.Fatal("repaired corrupt journal")
		}
		for name, want := range map[string][]byte{"journal/pending.json": raw, "journal/commit.json": marker, "service.json": identity} {
			got, _ := os.ReadFile(filepath.Join(root, name))
			if !bytes.Equal(got, want) {
				t.Fatal("lost evidence", name)
			}
		}
	}
}

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
	"strings"
	"testing"
)

func projectTestServer(t *testing.T, root string) (*Server, protocol.Client) {
	t.Helper()
	st, e := store.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	s, e := New(st, Config{Limits: protocol.DefaultLimits(), TitlePrefixes: []string{"feat", "fix"}})
	if e != nil {
		st.Close()
		t.Fatal(e)
	}
	c := protocol.Client{SchemaVersion: 1, ClientID: protocol.UUID(), StateRevision: 1, Status: "connected", Actor: protocol.Actor{Name: "Test", Kind: "human"}}
	if e = s.SaveClient(c); e != nil {
		t.Fatal(e)
	}
	return s, c
}
func prepareProjectTest(t *testing.T, s *Server, c protocol.Client, kind string, items ...protocol.ProjectInput) protocol.Prepared {
	t.Helper()
	p, e := s.PrepareProjects(protocol.PrepareRequest{Operation: "project." + kind, Items: items}, c)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func executeProjectTest(t *testing.T, s *Server, c protocol.Client, p protocol.Prepared) (int, []byte) {
	t.Helper()
	b, _ := json.Marshal(protocol.DurableRequest{SchemaVersion: 1, Intent: p.Intent, RequestHash: p.RequestHash})
	r := httptest.NewRequest("POST", "/v1/transactions", bytes.NewReader(b))
	r.Header.Set("X-Lit-Client-ID", c.ClientID)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w.Code, w.Body.Bytes()
}
func textPointer(s string) *string { return &s }
func TestProjectJournalBoundaries(t *testing.T) {
	for _, boundary := range []string{"after_journal_directory", "after_payload", "before_commit", "after_commit", "after_write:1", "after_write:2", "after_write:3", "after_write:4", "after_write:5", "after_write:6", "after_write:7", "cleanup_after_marker", "cleanup_after_payload"} {
		t.Run(boundary, func(t *testing.T) {
			root := t.TempDir()
			s, c := projectTestServer(t, root)
			ids := []string{protocol.UUID(), protocol.UUID()}
			plan := prepareProjectTest(t, s, c, "create", protocol.ProjectInput{ID: ids[0], Title: textPointer("feat/first"), Content: textPointer("first\r\n")}, protocol.ProjectInput{ID: ids[1], Title: textPointer("feat/second"), Content: textPointer("second\n")})
			fired := false
			s.Store.Fault = func(point string) error {
				if point == boundary {
					fired = true
					return fmt.Errorf("interrupted")
				}
				return nil
			}
			status, _ := executeProjectTest(t, s, c, plan)
			if !fired {
				t.Fatalf("boundary not exercised: %s (status %d)", boundary, status)
			}
			if status != 503 {
				t.Fatalf("fault status %d", status)
			}
			identity := s.Store.Identity.ServiceID
			s.Store.Close()
			committed := boundary != "after_journal_directory" && boundary != "after_payload" && boundary != "before_commit"
			var firstHistory []byte
			for restart := 0; restart < 2; restart++ {
				st, e := store.Open(root)
				if e != nil {
					t.Fatal(e)
				}
				reopened, e := New(st, s.Config)
				if e != nil {
					st.Close()
					t.Fatal(e)
				}
				if st.Identity.ServiceID != identity {
					t.Fatal("identity changed")
				}
				ps, e := reopened.Projects()
				if e != nil {
					t.Fatal(e)
				}
				want := 0
				if committed {
					want = 2
				}
				if len(ps) != want {
					t.Fatalf("partial transaction after restart: %d", len(ps))
				}
				for _, p := range ps {
					hs, e := reopened.ProjectHistory(p.ID)
					if e != nil || len(hs) != 1 || p.Revision != 1 || hs[0].RequestHash != plan.RequestHash || len(hs[0].Results) != 2 {
						t.Fatal("invalid replay", hs, e)
					}
					if p.ID == ids[0] {
						historyBytes, _ := json.Marshal(hs)
						if restart == 0 {
							firstHistory = historyBytes
						} else if !bytes.Equal(firstHistory, historyBytes) {
							t.Fatal("replay changed history bytes")
						}
					}
				}
				index, e := st.Read("indexes/titles.json")
				if e != nil {
					t.Fatal(e)
				}
				var idx TitleIndex
				if protocol.Decode(index, &idx) != nil || len(idx.Projects) != want {
					t.Fatal("partial index", string(index))
				}
				st.Close()
			}
		})
	}
}
func TestProjectCorruptionAndIndexRebuild(t *testing.T) {
	for _, damage := range []string{"body", "relationships", "history-result", "history-diff", "history-schema", "index-missing", "index-corrupt"} {
		t.Run(damage, func(t *testing.T) {
			root := t.TempDir()
			s, c := projectTestServer(t, root)
			id := protocol.UUID()
			p := prepareProjectTest(t, s, c, "create", protocol.ProjectInput{ID: id, Title: textPointer("feat/project"), Content: textPointer("preserve")})
			if code, _ := executeProjectTest(t, s, c, p); code != 200 {
				t.Fatal(code)
			}
			s.Store.Close()
			contentPath := filepath.Join(root, projectDir(id), "content.md")
			relsPath := filepath.Join(root, projectDir(id), "relationships.json")
			historyFiles, _ := filepath.Glob(filepath.Join(root, projectDir(id), "history", "*"))
			historyPath := historyFiles[0]
			switch damage {
			case "body":
				b, _ := os.ReadFile(contentPath)
				os.WriteFile(contentPath, append(b, []byte("changed")...), 0600)
			case "relationships":
				b, _ := json.Marshal(projectRelationships{1, []string{protocol.UUID()}})
				os.WriteFile(relsPath, b, 0600)
			case "history-result", "history-diff", "history-schema":
				b, _ := os.ReadFile(historyPath)
				var h ProjectHistory
				json.Unmarshal(b, &h)
				if damage == "history-result" {
					h.Results[0].Revision = 90
				} else if damage == "history-schema" {
					h.SchemaVersion = 2
				} else {
					h.Differences["title"] = FieldDifference{Before: json.RawMessage("null"), After: json.RawMessage(`"feat/changed"`)}
				}
				b, _ = json.Marshal(h)
				os.WriteFile(historyPath, b, 0600)
			case "index-missing":
				os.Remove(filepath.Join(root, "indexes/titles.json"))
			case "index-corrupt":
				os.WriteFile(filepath.Join(root, "indexes/titles.json"), []byte("broken"), 0600)
			}
			st, e := store.Open(root)
			if e != nil {
				t.Fatal(e)
			}
			defer st.Close()
			_, e = New(st, s.Config)
			if strings.HasPrefix(damage, "index-") {
				if e != nil {
					t.Fatal("derived index did not rebuild", e)
				}
				b, _ := st.Read("indexes/titles.json")
				var idx TitleIndex
				if protocol.Decode(b, &idx) != nil || idx.Projects["feat/project"] != id {
					t.Fatal("wrong rebuilt index")
				}
			} else if e == nil {
				t.Fatal("corrupt authority accepted")
			}
		})
	}
}
func TestProjectResponseBoundBeforeCommit(t *testing.T) {
	s, c := projectTestServer(t, t.TempDir())
	defer s.Store.Close()
	id := protocol.UUID()
	p := prepareProjectTest(t, s, c, "create", protocol.ProjectInput{ID: id, Title: textPointer("feat/a")})
	s.Config.Limits.SnapshotBytes = 100
	if code, _ := executeProjectTest(t, s, c, p); code != 413 {
		t.Fatal(code)
	}
	if _, e := s.Project(id); e == nil {
		t.Fatal("committed mutation rejected as too large")
	}
	hs, _ := s.ProjectHistory(id)
	if len(hs) != 0 {
		t.Fatal("rejected mutation added history")
	}
}
func TestProjectGuardAndNoopPreconditions(t *testing.T) {
	s, c := projectTestServer(t, t.TempDir())
	defer s.Store.Close()
	id, guard := protocol.UUID(), protocol.UUID()
	p := prepareProjectTest(t, s, c, "create", protocol.ProjectInput{ID: id, Title: textPointer("feat/a")}, protocol.ProjectInput{ID: guard, Title: textPointer("feat/b")})
	executeProjectTest(t, s, c, p)
	noop := prepareProjectTest(t, s, c, "update", protocol.ProjectInput{Target: id, Set: protocol.ProjectSet{Title: textPointer("feat/a")}})
	rev := int64(1)
	noop.Intent.Guards = []protocol.ObjectRef{{Type: "projects", ID: guard, ExpectedRevision: &rev}}
	_, noop.RequestHash, _ = protocol.Canonical(noop.Intent)
	update := prepareProjectTest(t, s, c, "update", protocol.ProjectInput{Target: guard, Set: protocol.ProjectSet{Description: textPointer("changed")}})
	executeProjectTest(t, s, c, update)
	if code, _ := executeProjectTest(t, s, c, noop); code != 409 {
		t.Fatal("stale guard accepted noop", code)
	}
	hs, _ := s.ProjectHistory(id)
	if len(hs) != 1 {
		t.Fatal("guard changed owner")
	}
	update = prepareProjectTest(t, s, c, "update", protocol.ProjectInput{Target: id, Set: protocol.ProjectSet{Description: textPointer("change")}})
	executeProjectTest(t, s, c, update)
	noop.Intent.Guards = nil
	_, noop.RequestHash, _ = protocol.Canonical(noop.Intent)
	if code, _ := executeProjectTest(t, s, c, noop); code != 409 {
		t.Fatal("stale noop accepted", code)
	}
}
func TestProjectRetiredPrefixAndClock(t *testing.T) {
	s, c := projectTestServer(t, t.TempDir())
	root := s.Store.Root
	id := protocol.UUID()
	p := prepareProjectTest(t, s, c, "create", protocol.ProjectInput{ID: id, Title: textPointer("feat/a")})
	executeProjectTest(t, s, c, p)
	p = prepareProjectTest(t, s, c, "update", protocol.ProjectInput{Target: id, Set: protocol.ProjectSet{Title: textPointer("fix/a")}})
	executeProjectTest(t, s, c, p)
	s.Store.Close()
	st, e := store.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	config := s.Config
	config.TitlePrefixes = []string{"fix"}
	if _, e = New(st, config); e != nil {
		t.Fatal("retired historical prefix blocks restart", e)
	}
	future := "2099-01-01T00:00:00.000000Z"
	stamp := s.nextProjectTime([]protocol.Project{{UpdatedAt: future}})
	if stamp != "2099-01-01T00:00:00.000001Z" {
		t.Fatal("clock moved backward", stamp)
	}
}

func TestProjectUpdateJournalPreservesOtherOwners(t *testing.T) {
	for _, boundary := range []string{"before_commit", "after_write:2", "after_write:6"} {
		t.Run(boundary, func(t *testing.T) {
			s, c := projectTestServer(t, t.TempDir())
			root := s.Store.Root
			ids := []string{protocol.UUID(), protocol.UUID(), protocol.UUID()}
			create := []protocol.ProjectInput{}
			for n, id := range ids {
				create = append(create, protocol.ProjectInput{ID: id, Title: textPointer(fmt.Sprintf("feat/p%d", n)), Content: textPointer("before\r\n")})
			}
			p := prepareProjectTest(t, s, c, "create", create...)
			if code, _ := executeProjectTest(t, s, c, p); code != 200 {
				t.Fatal(code)
			}
			untouched, _ := s.Store.Read(projectDir(ids[2]) + "/content.md")
			p = prepareProjectTest(t, s, c, "update", protocol.ProjectInput{Target: ids[0], Set: protocol.ProjectSet{Description: textPointer("after\n")}}, protocol.ProjectInput{Target: ids[1], Set: protocol.ProjectSet{Description: textPointer("after\n")}})
			s.Store.Fault = func(point string) error {
				if point == boundary {
					return fmt.Errorf("interrupted")
				}
				return nil
			}
			if code, _ := executeProjectTest(t, s, c, p); code != 503 {
				t.Fatal(code)
			}
			s.Store.Close()
			st, e := store.Open(root)
			if e != nil {
				t.Fatal(e)
			}
			defer st.Close()
			reopened, e := New(st, s.Config)
			if e != nil {
				t.Fatal(e)
			}
			for _, id := range ids[:2] {
				record, e := reopened.Project(id)
				if e != nil {
					t.Fatal(e)
				}
				body, revision := "after\n", int64(2)
				if boundary == "before_commit" {
					body, revision = "before\r\n", 1
				}
				if record.Description != body || record.Revision != revision {
					t.Fatal(record)
				}
				hs, _ := reopened.ProjectHistory(id)
				if len(hs) != int(revision) {
					t.Fatal("wrong history count")
				}
			}
			current, _ := st.Read(projectDir(ids[2]) + "/content.md")
			if !bytes.Equal(current, untouched) {
				t.Fatal("unrelated owner changed")
			}
		})
	}
}

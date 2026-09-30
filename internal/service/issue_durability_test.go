package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/store"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func prepareRecordTest(t *testing.T, s *Server, c protocol.Client, project, op string, items ...protocol.ProjectInput) protocol.Prepared {
	t.Helper()
	p, e := s.PrepareRecords(protocol.PrepareRequest{Operation: op, Project: project, Items: items}, c)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func executeRecordTest(t *testing.T, s *Server, c protocol.Client, p protocol.Prepared) {
	t.Helper()
	code, body := executeProjectTest(t, s, c, p)
	if code != 200 {
		t.Fatal(code, string(body))
	}
}
func TestProjectOnlyJSONFixtureRejected(t *testing.T) {
	root := t.TempDir()
	original := map[string][]byte{}
	e := filepath.WalkDir("testdata/project-only-v1", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel("testdata/project-only-v1", path)
		if e != nil {
			return e
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		original[rel] = b
		target := filepath.Join(root, rel)
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return e
		}
		return os.WriteFile(target, b, 0600)
	})
	if e != nil {
		t.Fatal(e)
	}
	st, e := store.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	if _, e = New(st, Config{Limits: protocol.DefaultLimits(), TitlePrefixes: []string{"feat", "fix"}}); e == nil {
		t.Fatal("accepted old JSON frontmatter store")
	}
	for rel, want := range original {
		got, e := st.Read(filepath.ToSlash(rel))
		if e != nil || !bytes.Equal(got, want) {
			t.Fatal("rejected store bytes changed", rel, e)
		}
	}
}

func TestIssueCommentJournalBoundaries(t *testing.T) {
	for _, kind := range []string{"issue.create", "comment.create"} {
		seed, client := projectTestServer(t, t.TempDir())
		project, parent := protocol.UUID(), protocol.UUID()
		executeRecordTest(t, seed, client, prepareProjectTest(t, seed, client, "create", protocol.ProjectInput{ID: project, Title: textPointer("feat/count")}))
		executeRecordTest(t, seed, client, prepareRecordTest(t, seed, client, project, "issue.create", protocol.ProjectInput{ID: parent, Title: textPointer("Parent")}))
		items := []protocol.ProjectInput{{ID: protocol.UUID(), Title: textPointer("A"), Parent: &parent}, {ID: protocol.UUID(), Title: textPointer("B"), Parent: &parent}}
		if kind == "comment.create" {
			items = []protocol.ProjectInput{{ID: protocol.UUID(), Issue: parent}, {ID: protocol.UUID(), Issue: parent}}
		}
		plan := prepareRecordTest(t, seed, client, project, kind, items...)
		seedWrites, _, e := seed.recordTransaction(plan.Intent, plan.RequestHash, client)
		if e != nil {
			t.Fatal(e)
		}
		seed.Store.Close()
		boundaries := []string{"after_journal_directory", "after_payload", "before_commit", "after_commit", "cleanup_after_marker", "cleanup_after_payload"}
		for n := 1; n <= len(seedWrites); n++ {
			boundaries = append(boundaries, fmt.Sprintf("after_write:%d", n))
		}
		for _, boundary := range boundaries {
			t.Run(kind+"/"+boundary, func(t *testing.T) {
				root := t.TempDir()
				s, c := projectTestServer(t, root)
				project, parent, first, second := protocol.UUID(), protocol.UUID(), protocol.UUID(), protocol.UUID()
				executeRecordTest(t, s, c, prepareProjectTest(t, s, c, "create", protocol.ProjectInput{ID: project, Title: textPointer("feat/crash")}))
				executeRecordTest(t, s, c, prepareRecordTest(t, s, c, project, "issue.create", protocol.ProjectInput{ID: parent, Title: textPointer("Parent")}))
				items := []protocol.ProjectInput{{ID: first, Title: textPointer("First"), Parent: &parent, Content: textPointer("# First\r\n雪")}, {ID: second, Title: textPointer("Second"), Parent: &parent, Content: textPointer("Second\n")}}
				if kind == "comment.create" {
					items = []protocol.ProjectInput{{ID: first, Issue: parent, Content: textPointer("# First\r\n雪")}, {ID: second, Issue: parent, Content: textPointer("Second\n")}}
				}
				plan := prepareRecordTest(t, s, c, project, kind, items...)
				writes, result, e := s.recordTransaction(plan.Intent, plan.RequestHash, c)
				if e != nil {
					t.Fatal(e)
				}
				if len(writes) != len(seedWrites) {
					t.Fatal("fault matrix does not cover actual writes", len(writes), len(seedWrites))
				}
				fired := false
				s.Store.Fault = func(point string) error {
					if point == boundary {
						fired = true
						return fmt.Errorf("interrupted")
					}
					return nil
				}
				code, _ := executeProjectTest(t, s, c, plan)
				if !fired || code != 503 {
					t.Fatal(boundary, code, fired)
				}
				identity := s.Store.Identity.ServiceID
				s.Store.Close()
				committed := boundary != "after_journal_directory" && boundary != "after_payload" && boundary != "before_commit"
				var accepted map[string][]byte
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
					p, _ := reopened.Project(project)
					owner, _ := reopened.Issue(parent)
					wantRev := int64(1)
					if committed {
						wantRev = 2
					}
					if owner.Revision != wantRev {
						t.Fatal("owner revised more than once", owner)
					}
					if kind == "issue.create" {
						want := 1
						if committed {
							want = 3
						}
						if len(p.IssueIDs) != want || len(owner.ChildIDs) != want-1 {
							t.Fatal(p, owner)
						}
					} else {
						want := 0
						if committed {
							want = 2
						}
						if len(owner.CommentIDs) != want || p.Revision != 2 {
							t.Fatal(p, owner)
						}
					}
					if committed {
						now := map[string][]byte{}
						for _, change := range result.Items {
							hs, e := reopened.RecordHistory(change.Type, change.ID)
							if e != nil {
								t.Fatal(e)
							}
							last := hs[len(hs)-1]
							if last.RequestHash != plan.RequestHash || !same(last.Results, result.Items) {
								t.Fatal(last)
							}
							path := recordPath(change.Type, change.ID) + "/history/" + last.Timestamp + "-" + last.RequestHash + ".json"
							now[path], e = st.Read(path)
							if e != nil {
								t.Fatal(e)
							}
						}
						if restart == 0 {
							accepted = now
						} else if !same(accepted, now) {
							t.Fatal("replay changed accepted bytes")
						}
					}
					st.Close()
				}
			})
		}
	}
}

func TestRecordRejectsCorruptSource(t *testing.T) {
	for _, typ := range []string{"issues", "comments"} {
		for _, mode := range []string{"body-shadow", "relationship-placement", "invalid-utf8"} {
			t.Run(typ+"/"+mode, func(t *testing.T) {
				root := t.TempDir()
				s, c := projectTestServer(t, root)
				project, issue, comment := protocol.UUID(), protocol.UUID(), protocol.UUID()
				executeRecordTest(t, s, c, prepareProjectTest(t, s, c, "create", protocol.ProjectInput{ID: project, Title: textPointer("feat/corrupt")}))
				executeRecordTest(t, s, c, prepareRecordTest(t, s, c, project, "issue.create", protocol.ProjectInput{ID: issue, Title: textPointer("Issue"), Content: textPointer("\uFFFD")}))
				executeRecordTest(t, s, c, prepareRecordTest(t, s, c, project, "comment.create", protocol.ProjectInput{ID: comment, Issue: issue, Content: textPointer("\uFFFD")}))
				s.Store.Close()
				id := issue
				if typ == "comments" {
					id = comment
				}
				path := filepath.Join(root, typ, id, "content.md")
				b, e := os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				meta := map[string]any{}
				body, e := decodeFrontmatter(b, &meta)
				if e != nil {
					t.Fatal(e)
				}
				changed := map[string][]byte{}
				switch mode {
				case "body-shadow":
					meta["body"] = "shadow body"
				case "invalid-utf8":
					body = []byte{0xff}
				case "relationship-placement":
					relPath := filepath.Join(root, typ, id, "relationships.json")
					raw, e := os.ReadFile(relPath)
					if e != nil {
						t.Fatal(e)
					}
					rel := map[string]any{}
					if e = json.Unmarshal(raw, &rel); e != nil {
						t.Fatal(e)
					}
					key := "project_id"
					if typ == "comments" {
						key = "issue_id"
					}
					meta[key] = rel[key]
					delete(rel, key)
					changed[relPath] = mustJSON(rel)
				}
				changed[path] = encodeFrontmatter(meta, string(body))
				for path, b := range changed {
					if e = os.WriteFile(path, b, 0600); e != nil {
						t.Fatal(e)
					}
				}
				st, e := store.Open(root)
				if e != nil {
					t.Fatal(e)
				}
				defer st.Close()
				if _, e = New(st, s.Config); e == nil {
					t.Fatal("silently accepted corrupt source")
				}
				for path, b := range changed {
					preserved, e := os.ReadFile(path)
					if e != nil || !bytes.Equal(b, preserved) {
						t.Fatal("startup changed corrupt evidence", e)
					}
				}
			})
		}
	}
}

package service

import (
	"bytes"
	"fmt"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/store"
	"testing"
)

func TestLinkEveryJournalWriteRecovery(t *testing.T) {
	boundaries := []string{"after_journal_directory", "after_payload", "before_commit", "after_commit", "cleanup_after_marker", "cleanup_after_payload"}
	// Two issue owners each have content, relationship, and history writes, plus index.
	for n := 1; n <= 7; n++ {
		boundaries = append(boundaries, fmt.Sprintf("after_write:%d", n))
	}
	for _, boundary := range boundaries {
		t.Run(boundary, func(t *testing.T) {
			root := t.TempDir()
			s, c := projectTestServer(t, root)
			p, q, a, b := protocol.UUID(), protocol.UUID(), protocol.UUID(), protocol.UUID()
			executeRecordTest(t, s, c, prepareProjectTest(t, s, c, "create", protocol.ProjectInput{ID: p, Title: textPointer("feat/one")}, protocol.ProjectInput{ID: q, Title: textPointer("feat/two")}))
			executeRecordTest(t, s, c, prepareRecordTest(t, s, c, p, "issue.create", protocol.ProjectInput{ID: a, Title: textPointer("One"), Content: textPointer("# exact\r\n雪")}))
			executeRecordTest(t, s, c, prepareRecordTest(t, s, c, q, "issue.create", protocol.ProjectInput{ID: b, Title: textPointer("Two")}))
			plan := prepareRecordTest(t, s, c, "", "issue.link", protocol.ProjectInput{From: a, To: []string{b}, Relation: "blocks"})
			writes, _, e := s.recordTransaction(plan.Intent, plan.RequestHash, c)
			if e != nil || len(writes) != 7 {
				t.Fatal(len(writes), e)
			}
			identity := s.Store.Identity.ServiceID
			before, _ := s.ReadRecordState()
			fired := false
			s.Store.Fault = func(point string) error {
				if point == boundary {
					fired = true
					return fmt.Errorf("interrupted")
				}
				return nil
			}
			err := s.Store.Commit(writes)
			if !fired || err == nil {
				t.Fatal(fired, err)
			}
			s.Store.Close()
			committed := boundary != "after_journal_directory" && boundary != "after_payload" && boundary != "before_commit"
			for restart := 0; restart < 2; restart++ {
				st, e := store.Open(root)
				if e != nil {
					t.Fatal(e)
				}
				s, e = New(st, s.Config)
				if e != nil {
					st.Close()
					t.Fatal(e)
				}
				if s.Store.Identity.ServiceID != identity {
					t.Fatal("identity changed")
				}
				if committed {
					for _, w := range writes {
						got, e := st.Read(w.Path)
						if e != nil || !bytes.Equal(got, w.Data) {
							t.Fatal("replay differs", w.Path, e)
						}
					}
				} else {
					after, _ := s.ReadRecordState()
					if !same(before, after) {
						t.Fatal("partial rejected link")
					}
				}
				left, _ := s.Issue(a)
				right, _ := s.Issue(b)
				if (len(left.Blocks) == 1) != committed || (len(right.BlockedBy) == 1) != committed {
					t.Fatal("partial endpoints", left, right)
				}
				st.Close()
			}
		})
	}
}

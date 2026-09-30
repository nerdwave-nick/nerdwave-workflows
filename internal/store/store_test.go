package store

import (
	"errors"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRecoveryBoundaries(t *testing.T) {
	for _, point := range []string{"after_journal_directory", "after_payload", "before_commit", "after_commit", "after_write:1", "after_write:2", "cleanup_after_marker", "cleanup_after_payload"} {
		t.Run(point, func(t *testing.T) {
			root := t.TempDir()
			s, e := Open(root)
			if e != nil {
				t.Fatal(e)
			}
			id := s.Identity.ServiceID
			s.Fault = func(p string) error {
				if p == point {
					return errors.New("interrupted")
				}
				return nil
			}
			e = s.Commit([]Write{{Path: "clients/a.json", Data: []byte("first")}, {Path: "projects/x/history/a.json", Data: []byte("second")}})
			if e == nil {
				t.Fatal("fault not exercised")
			}
			s.Close()
			s, e = Open(root)
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			if s.Identity.ServiceID != id {
				t.Fatal("identity changed")
			}
			committed := point != "before_commit" && point != "after_payload" && point != "after_journal_directory"
			for p, want := range map[string]string{"clients/a.json": "first", "projects/x/history/a.json": "second"} {
				got, e := s.Read(p)
				if committed && string(got) != want {
					t.Fatalf("wrong replay content %s: %q", p, got)
				}
				if committed && e != nil {
					t.Fatalf("committed %s missing: %v", p, e)
				}
				if !committed && !os.IsNotExist(e) {
					t.Fatalf("uncommitted %s installed", p)
				}
			}
			if _, e = os.Stat(filepath.Join(root, "journal")); !os.IsNotExist(e) {
				t.Fatal("journal not retired")
			}
			s.Close()
			again, e := Open(root)
			if e != nil {
				t.Fatal(e)
			}
			defer again.Close()
			if again.Identity.ServiceID != id {
				t.Fatal("second restart changed identity")
			}
			for p, want := range map[string]string{"clients/a.json": "first", "projects/x/history/a.json": "second"} {
				got, e := again.Read(p)
				if committed && (e != nil || string(got) != want) {
					t.Fatal("second restart changed committed bytes", p, e)
				}
				if !committed && !os.IsNotExist(e) {
					t.Fatal("second restart invented bytes", p)
				}
			}
		})
	}
}
func TestRecoveryRejectsCorruptAndUnsafeEvidence(t *testing.T) {
	t.Run("bad marker", func(t *testing.T) {
		root := t.TempDir()
		s, _ := Open(root)
		s.Fault = func(p string) error {
			if p == "after_commit" {
				return errors.New("crash")
			}
			return nil
		}
		_ = s.Commit([]Write{{Path: "clients/a.json", Data: []byte("x")}})
		s.Close()
		p := filepath.Join(root, "journal", "commit.json")
		os.WriteFile(p, []byte(`{"schema_version":2,"sha256":"bad"}`), 0600)
		if _, e := Open(root); e == nil {
			t.Fatal("accepted damaged journal")
		}
		if _, e := os.Stat(p); e != nil {
			t.Fatal("destroyed evidence")
		}
	})
	t.Run("symlink replay", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		s, _ := Open(root)
		s.Fault = func(p string) error {
			if p == "after_commit" {
				return errors.New("crash")
			}
			return nil
		}
		_ = s.Commit([]Write{{Path: "clients/a.json", Data: []byte("x")}})
		s.Close()
		os.Symlink(outside, filepath.Join(root, "clients"))
		if _, e := Open(root); e == nil {
			t.Fatal("followed symlink")
		}
		if _, e := os.Stat(filepath.Join(outside, "a.json")); !os.IsNotExist(e) {
			t.Fatal("wrote outside store")
		}
	})
	t.Run("unknown identity", func(t *testing.T) {
		root := t.TempDir()
		os.WriteFile(filepath.Join(root, "unknown"), []byte("keep"), 0600)
		if _, e := Open(root); e == nil {
			t.Fatal("initialized populated store")
		}
		if _, e := os.Stat(filepath.Join(root, "service.json")); !os.IsNotExist(e) {
			t.Fatal("invented identity")
		}
	})
	t.Run("version", func(t *testing.T) {
		root := t.TempDir()
		s, _ := Open(root)
		s.Close()
		os.WriteFile(filepath.Join(root, "service.json"), []byte(`{"schema_version":2,"service_id":"`+protocol.UUID()+`"}`), 0600)
		if _, e := Open(root); e == nil {
			t.Fatal("accepted future identity")
		}
	})
}
func TestTempsAndExclusiveOwnership(t *testing.T) {
	root := t.TempDir()
	s, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Open(root); e == nil {
		t.Fatal("second owner accepted")
	}
	s.Close()
	os.Mkdir(filepath.Join(root, "journal"), 0700)
	os.WriteFile(filepath.Join(root, "journal", ".lit-tmp-incomplete"), []byte("torn"), 0600)
	s, e = Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = os.WriteFile(filepath.Join(root, "external"), []byte("untracked"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = s.Commit([]Write{{Path: "clients/x", Data: []byte("x")}}); e == nil {
		t.Fatal("ignored external change")
	}
}
func TestInterruptedReplayPreservesExactCommit(t *testing.T) {
	root := t.TempDir()
	s, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	id := s.Identity.ServiceID
	first := []byte(`{"schema_version":1,"revision":7,"timestamp":"2026-09-29T12:00:00Z"}`)
	second := []byte("---\nschema_version: 1\n---\nExact prose\r\n")
	s.Fault = func(p string) error {
		if p == "after_commit" {
			return errors.New("crash")
		}
		return nil
	}
	if e = s.Commit([]Write{{Path: "history/a.json", Data: first}, {Path: "projects/x/content.md", Data: second}}); e == nil {
		t.Fatal("missing crash")
	}
	s.Close()
	if _, e = OpenWithFault(root, func(p string) error {
		if p == "after_write:1" {
			return errors.New("replay crash")
		}
		return nil
	}); e == nil {
		t.Fatal("missing replay interruption")
	}
	s, e = Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if s.Identity.ServiceID != id {
		t.Fatal("changed service ID")
	}
	for p, want := range map[string][]byte{"history/a.json": first, "projects/x/content.md": second} {
		got, e := s.Read(p)
		if e != nil || string(got) != string(want) {
			t.Fatal("replay changed bytes", p, string(got), e)
		}
	}
}

func TestIdentityValidationPrecedesRecovery(t *testing.T) {
	for _, identity := range []string{"future", "corrupt", "missing"} {
		for _, point := range []string{"after_commit", "before_commit"} {
			t.Run(identity+"/"+point, func(t *testing.T) {
				root := t.TempDir()
				s, err := Open(root)
				if err != nil {
					t.Fatal(err)
				}
				s.Fault = func(p string) error {
					if p == point {
						return errors.New("interrupted")
					}
					return nil
				}
				if err = s.Commit([]Write{{Path: "clients/a.json", Data: []byte("new")}}); err == nil {
					t.Fatal("fault not hit")
				}
				if err = s.Close(); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, "service.json")
				switch identity {
				case "future":
					err = os.WriteFile(path, []byte(`{"schema_version":2,"service_id":"`+protocol.UUID()+`"}`), 0600)
				case "corrupt":
					err = os.WriteFile(path, []byte(`{"broken"`), 0600)
				case "missing":
					err = os.Remove(path)
				}
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(root, ".lit-tmp-evidence"), []byte("torn"), 0600); err != nil {
					t.Fatal(err)
				}
				before := snapshotStore(t, root)
				got, err := Open(root)
				if err == nil {
					got.Close()
					t.Fatal("invalid identity accepted")
				}
				after := snapshotStore(t, root)
				if !reflect.DeepEqual(before, after) {
					t.Fatalf("failed startup modified evidence\nbefore=%v\nafter=%v", before, after)
				}
			})
		}
	}
}

func snapshotStore(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == ".lock" {
			return nil
		}
		if d.IsDir() {
			out[rel] = "directory"
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestInterruptedFirstInitialization(t *testing.T) {
	root := t.TempDir()
	tmp := filepath.Join(root, ".lit-tmp-initial-identity")
	if err := os.WriteFile(tmp, []byte(`{"schema_version":`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	id := s.Identity.ServiceID
	if !protocol.ValidUUID(id) {
		t.Fatal("invalid fresh identity")
	}
	if _, err = os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatal("initialization temporary not cleaned", err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Identity.ServiceID != id {
		t.Fatal("new identity changed on restart")
	}
}

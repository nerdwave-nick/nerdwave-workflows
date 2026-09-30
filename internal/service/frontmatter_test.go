package service

import (
	"bytes"
	"encoding/json"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/store"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrontmatterReadableAndExact(t *testing.T) {
	for _, body := range []string{"", "no newline", "\r\n雪\r\n---\n\n", "\n\ntrailing\n\n"} {
		p := protocol.Project{SchemaVersion: 1, ID: protocol.UUID(), Revision: 9007199254740993, CreatedAt: "2026-09-30T00:00:00.123456789Z", UpdatedAt: "2026-09-30T00:00:00Z", Title: "feat/test", RepositoryRefs: []string{"true", "123", "null", "x: y", "#x", "line\n---\nend"}, Description: body}
		b := projectBytes(p)
		if !bytes.Contains(b, []byte("schema_version: 1\n")) || bytes.Contains(b, []byte(`{"schema_version"`)) {
			t.Fatalf("not readable block YAML: %s", b)
		}
		var m projectMetadata
		got, err := decodeFrontmatter(b, &m)
		if err != nil || string(got) != body || m.Revision != p.Revision || m.CreatedAt != p.CreatedAt {
			t.Fatalf("roundtrip: %+v %q %v", m, got, err)
		}
		if !bytes.Equal(b, projectBytes(p)) {
			t.Fatal("nondeterministic")
		}
	}
}
func TestFrontmatterRejectsCorruption(t *testing.T) {
	for _, meta := range []string{
		"revision: 1\nrevision: 2", "title: &x hi", "title: *x", "title: !custom hi", "title: !!str hi", "<<: {title: hi}", "1: hi", "title: true", "revision: '1'", "revision: 1.5", "revision: 0x10", "title: 2026-09-30", "unknown: hi", "title: {a: 1, a: 2}", "title: hi\n...\n--- # second document\ntitle: there", "title: \xff", `{"title":"a","title":"b"}`, `{"unknown":1}`, `{"revision":"1"}`,
	} {
		t.Run(meta, func(t *testing.T) {
			var m projectMetadata
			if _, err := decodeFrontmatter([]byte("---\n"+meta+"\n---\nbody"), &m); err == nil {
				t.Fatal("accepted malformed metadata")
			}
		})
	}
}
func TestFrontmatterRejectsJSONAndFlowMappings(t *testing.T) {
	for _, raw := range []string{`{"schema_version":1,"title":"feat/test"}`, `{schema_version: 1, title: feat/test}`, "# comment\n{\n  schema_version: 1,\n  title: feat/test\n}", "!!map {title: feat/test}"} {
		var got projectMetadata
		if _, err := decodeFrontmatter([]byte("---\n"+raw+"\n---\nbody"), &got); err == nil {
			t.Fatalf("accepted JSON/flow metadata: %s", raw)
		}
	}
}

func TestFrontmatterRestartAndMutation(t *testing.T) {
	root := t.TempDir()
	s, c := projectTestServer(t, root)
	project, issue, comment := protocol.UUID(), protocol.UUID(), protocol.UUID()
	body := "\r\n雪\n---\nlast"
	executeRecordTest(t, s, c, prepareProjectTest(t, s, c, "create", protocol.ProjectInput{ID: project, Title: textPointer("feat/yaml"), Content: &body}))
	executeRecordTest(t, s, c, prepareRecordTest(t, s, c, project, "issue.create", protocol.ProjectInput{ID: issue, Title: textPointer("true"), Content: &body}))
	executeRecordTest(t, s, c, prepareRecordTest(t, s, c, project, "comment.create", protocol.ProjectInput{ID: comment, Issue: issue, Content: &body}))
	paths := []string{"projects/" + project, "issues/" + issue, "comments/" + comment}
	originals := map[string][]byte{}
	// Keep every relationship and accepted history byte for comparison after startup.
	for _, dir := range paths {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			originals[path] = b
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, dir := range paths {
		path := filepath.Join(root, dir, "content.md")
		b := originals[path]
		if !bytes.Contains(b, []byte("schema_version: 1\n")) {
			t.Fatalf("not YAML: %s", b)
		}
		var meta map[string]json.RawMessage
		got, err := decodeFrontmatter(b, &meta)
		if err != nil || string(got) != body {
			t.Fatal(string(got), err)
		}
	}
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err = New(st, s.Config)
	if err != nil {
		st.Close()
		t.Fatal(err)
	}
	for path, want := range originals {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("startup rewrote %s: %v", path, err)
		}
	}
	next := body + " updated"
	executeRecordTest(t, s, c, prepareProjectTest(t, s, c, "update", protocol.ProjectInput{Target: project, Set: protocol.ProjectSet{Description: &next}}))
	executeRecordTest(t, s, c, prepareRecordTest(t, s, c, project, "issue.update", protocol.ProjectInput{Target: issue, Set: protocol.ProjectSet{Body: &next}}))
	executeRecordTest(t, s, c, prepareRecordTest(t, s, c, project, "comment.update", protocol.ProjectInput{Target: comment, Set: protocol.ProjectSet{Body: &next}}))
	for _, dir := range paths {
		b, err := os.ReadFile(filepath.Join(root, dir, "content.md"))
		if err != nil || !bytes.Contains(b, []byte("schema_version: 1\n")) {
			t.Fatal(string(b), err)
		}
	}
	for path, want := range originals {
		if strings.HasSuffix(path, "content.md") {
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("mutation rewrote retained history/relationships %s", path)
		}
	}
	if err = s.Store.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s, err = New(st, s.Config)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Project(project)
	if err != nil || p.Description != next {
		t.Fatal(p, err)
	}
	i, err := s.Issue(issue)
	if err != nil || i.Body != next {
		t.Fatal(i, err)
	}
	co, err := s.Comment(comment)
	if err != nil || co.Body != next {
		t.Fatal(co, err)
	}
}

func TestFrontmatterStartupRejectsMalformedMetadata(t *testing.T) {
	for _, resource := range []string{"projects", "issues", "comments"} {
		for _, damage := range []string{"version", "duplicate", "unknown", "missing", "null", "string-revision", "null-timestamp", "null-author", "json-format"} {
			if damage == "null-author" && resource != "comments" {
				continue
			}
			t.Run(resource+"/"+damage, func(t *testing.T) {
				root := t.TempDir()
				s, c := projectTestServer(t, root)
				project, issue, comment := protocol.UUID(), protocol.UUID(), protocol.UUID()
				executeRecordTest(t, s, c, prepareProjectTest(t, s, c, "create", protocol.ProjectInput{ID: project, Title: textPointer("feat/yaml")}))
				executeRecordTest(t, s, c, prepareRecordTest(t, s, c, project, "issue.create", protocol.ProjectInput{ID: issue, Title: textPointer("Issue")}))
				executeRecordTest(t, s, c, prepareRecordTest(t, s, c, project, "comment.create", protocol.ProjectInput{ID: comment, Issue: issue, Content: textPointer("body")}))
				id := map[string]string{"projects": project, "issues": issue, "comments": comment}[resource]
				path := filepath.Join(root, resource, id, "content.md")
				b, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var meta map[string]json.RawMessage
				body, err := decodeFrontmatter(b, &meta)
				if err != nil {
					t.Fatal(err)
				}
				switch damage {
				case "version":
					meta["schema_version"] = json.RawMessage(`2`)
				case "unknown":
					meta["unexpected"] = json.RawMessage(`1`)
				case "missing":
					delete(meta, "created_at")
				case "null":
					meta["id"] = json.RawMessage(`null`)
				case "null-timestamp":
					meta["updated_at"] = json.RawMessage(`null`)
				case "null-author":
					meta["author"] = json.RawMessage(`null`)
				case "string-revision":
					meta["revision"] = json.RawMessage(`"1"`)
				}
				b = encodeFrontmatter(meta, string(body))
				if damage == "duplicate" {
					b = append([]byte("---\nrevision: 1\n"), b[4:]...)
				}
				if damage == "json-format" {
					raw, err := json.Marshal(meta)
					if err != nil {
						t.Fatal(err)
					}
					b = append(append([]byte("---\n"), raw...), append([]byte("\n---\n"), body...)...)
				}

				if err = s.Store.Close(); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(path, b, 0600); err != nil {
					t.Fatal(err)
				}
				st, err := store.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				defer st.Close()
				if _, err = New(st, s.Config); err == nil {
					t.Fatal("startup accepted corrupt metadata")
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(after, b) {
					t.Fatal("startup rewrote corrupt evidence", err)
				}
			})
		}
	}
}

func TestFrontmatterFinalBlockScalarPreservesWhitespace(t *testing.T) {
	for _, tc := range []struct{ name, header, want string }{
		{"strip", "|-\n  name  \n", "name  "},
		{"clip", "|\n  name\n", "name\n"},
		{"keep", "|+\n  name\n\n", "name\n\n"},
		{"folded", ">\n  first\n  second\n", "first second\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "\r\n雪\n---\nlast"
			input := []byte("---\nauthor: " + tc.header + "---\n" + body)
			var m struct {
				Author string `json:"author"`
			}
			got, err := decodeFrontmatter(input, &m)
			if err != nil || m.Author != tc.want || string(got) != body {
				t.Fatalf("author=%q want=%q body=%q err=%v", m.Author, tc.want, got, err)
			}
		})
	}
}

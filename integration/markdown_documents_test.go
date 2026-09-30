package integration

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestMarkdownRecordDocumentExports(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("LIT_ENDPOINT", "")
	t.Setenv("LIT_SESSION", "")
	state := filepath.Join(root, "state")
	s := start(t, filepath.Join(root, "data"))
	call := func(args ...string) map[string]any { return formatJSONRun(t, root, state, 0, args...) }
	call("connect", "--endpoint", s.endpoint, "--actor-name", "Export tester")
	body := " \t\r\n# Original\n\n[link](https://example.test) | *text*\n<script>raw</script>\n```\ncode\n```\n---\n tail "
	project := item(call("projects", "create", "--project-title", "test/documents", "--content", body))["id"].(string)
	call("session", "set", "--project", project)
	issue := item(call("issues", "create", "--issue", "Export issue", "--content", body))["id"].(string)
	comment := item(call("comments", "create", "--issue", issue, "--content", body))["id"].(string)
	call("claims", "acquire", issue)
	second := item(call("issues", "create", "--issue", "Second export issue"))["id"].(string)
	for _, tc := range []struct {
		name     string
		refs     []string
		document bool
	}{
		{"deduplicated", []string{issue, issue}, true},
		{"multiple", []string{issue, second}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"issues", "get"}, tc.refs...)
			args = append(args, "--format", "markdown")
			code, out, stderr := rawCLI(t, root, state, args...)
			if code != 0 || stderr != "" || strings.HasPrefix(out, "---\n") != tc.document {
				t.Fatalf("%d %s %s", code, out, stderr)
			}
		})
	}

	for _, tc := range []struct{ kind, id, bodyKey string }{{"projects", project, "description"}, {"issues", issue, "body"}, {"comments", comment, "body"}} {
		t.Run(tc.kind, func(t *testing.T) {
			snapshot := item(call(tc.kind, "get", tc.id))
			code, out, stderr := rawCLI(t, root, state, tc.kind, "get", tc.id, "--format", "markdown")
			if code != 0 || stderr != "" || !strings.HasPrefix(out, "---\n") {
				t.Fatalf("%d %s %s", code, out, stderr)
			}
			parts := strings.SplitN(out[4:], "\n---\n", 2)
			if len(parts) != 2 || parts[1] != body {
				t.Fatalf("body changed: %q", out)
			}
			var metadata map[string]any
			if err := yaml.Unmarshal([]byte(parts[0]), &metadata); err != nil {
				t.Fatal(err)
			}
			delete(snapshot, tc.bodyKey)
			want, _ := json.Marshal(snapshot)
			got, _ := json.Marshal(metadata)
			if string(got) != string(want) {
				t.Fatalf("metadata mismatch\ngot %s\nwant %s", got, want)
			}
			if tc.kind == "projects" && !strings.Contains(string(got), issue) {
				t.Fatal("missing project membership", string(got))
			}
			if tc.kind == "issues" && (metadata["claimed"] != true || metadata["claim"] == nil) {
				t.Fatal("missing live claim", metadata)
			}
		})
	}
}

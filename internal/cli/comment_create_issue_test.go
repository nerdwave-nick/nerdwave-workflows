package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommentCreateIssueBoundary(t *testing.T) {
	for _, owners := range [][]string{{"One"}, {"One", "One"}, {"One", "Two"}} {
		args := []string{"comments", "create"}
		for i, owner := range owners {
			args = append(args, "--issue", owner, "--content", []string{"first", "second"}[i], "--author", []string{"Alice", "Bob"}[i])
		}
		parsed, err := Parse(args)
		if err != nil {
			t.Fatal(err)
		}
		app := App{Args: parsed}
		items, err := app.recordInputs()
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != len(owners) {
			t.Fatalf("got %d items", len(items))
		}
		for i, item := range items {
			if item.Issue != owners[i] || item.Content == nil || *item.Content != []string{"first", "second"}[i] || item.Author == nil || *item.Author != []string{"Alice", "Bob"}[i] {
				t.Fatalf("item%d: %+v", i, item)
			}
		}
		if len(items) == 2 && items[0].ID == items[1].ID {
			t.Fatal("batch comments share generated ID")
		}
	}
}

func TestCommentCreateJSONAndUpdateBoundaryUnchanged(t *testing.T) {
	file := filepath.Join(t.TempDir(), "comments.json")
	if err := os.WriteFile(file, []byte(`[{"issue":"One","content":"body","author":"Alice"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse([]string{"comments", "create", "--file", file})
	if err != nil {
		t.Fatal(err)
	}
	app := App{Args: parsed}
	items, err := app.recordInputs()
	if err != nil || len(items) != 1 || items[0].Issue != "One" || *items[0].Content != "body" {
		t.Fatalf("%+v %v", items, err)
	}
	parsed, err = Parse([]string{"comments", "update", "--comment", "comment-id", "--content", "edited"})
	if err != nil {
		t.Fatal(err)
	}
	app.Args = parsed
	items, err = app.recordInputs()
	if err != nil || len(items) != 1 || items[0].Target != "comment-id" || *items[0].Set.Body != "edited" {
		t.Fatalf("%+v %v", items, err)
	}
}

func TestCommentCreateHelpCompletionAndOldFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("LIT_STATE_DIR", t.TempDir())
	t.Setenv("LIT_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("LIT_SESSION", "")
	t.Chdir(t.TempDir())
	var out, stderr bytes.Buffer
	if code := Run([]string{"comments", "create", "--help"}, &out, &stderr); code != 0 || !strings.Contains(out.String(), "--issue ISSUE_REF") || strings.Contains(out.String(), "--comment") || strings.Contains(out.String(), "create: new title") {
		t.Fatalf("%d %s %s", code, &out, &stderr)
	}
	out.Reset()
	stderr.Reset()
	if code := Run([]string{"__complete", "comments", "create", "--"}, &out, &stderr); code != 0 || !strings.Contains(out.String(), "--issue") || strings.Contains(out.String(), "--comment") {
		t.Fatalf("completion %d %s %s", code, &out, &stderr)
	}
	for _, old := range []string{"--comment", "--comment=One"} {
		args := []string{"comments", "create", old}
		if old == "--comment" {
			args = append(args, "One")
		}
		if _, err := Parse(args); err == nil || !strings.Contains(err.Error(), "use --issue") {
			t.Fatalf("legacy Parse: %v", err)
		}
		out.Reset()
		stderr.Reset()
		if code := Run(args, &out, &stderr); code != 2 || !strings.Contains(stderr.String(), "use --issue") {
			t.Fatalf("legacy Run %d %s", code, &stderr)
		}
	}
}

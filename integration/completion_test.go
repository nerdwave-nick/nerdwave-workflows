package integration

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSessionlessCompletionCLI(t *testing.T) {
	data, state, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	s := start(t, data)
	run(t, cwd, state, 0, "connect", "--endpoint", s.endpoint, "--format", "json")
	run(t, cwd, state, 0, "projects", "create", "--project-title", "feat/completion", "--content", "PRIVATE_PROJECT_CONTENT")
	run(t, cwd, state, 0, "issues", "create", "--project", "feat/completion", "--issue", "Fix quotes 'and' spaces", "--content", "PRIVATE_ISSUE_CONTENT")
	freshState, freshCwd := t.TempDir(), t.TempDir()
	complete := func(stateDir string, args ...string) string {
		t.Helper()
		cmd := exec.Command(cliBin, append([]string{"__complete", "--endpoint", s.endpoint}, args...)...)
		cmd.Dir = freshCwd
		cmd.Env = append(os.Environ(), "GORACE=atexit_sleep_ms=0", "LIT_SESSION=", "LIT_ENDPOINT=", "LIT_STATE_DIR="+stateDir)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("completion failed: %v %s", err, out)
		}
		return string(out)
	}
	out := complete(freshState, "projects", "get", "")
	if !strings.Contains(out, "feat/completion\t") || strings.Contains(out, "PRIVATE") {
		t.Fatal(out)
	}
	out = complete(freshState, "issues", "get", "")
	want := "feat/completion:Fix quotes 'and' spaces"
	if !strings.Contains(out, want+"\t") || strings.Contains(out, "PRIVATE") {
		t.Fatal(out)
	}
	// The candidate is a usable global reference, even without a selected project.
	run(t, cwd, state, 0, "issues", "get", want)
	out = complete(freshState, "issues", "get", "--project", "feat/completion", "Fi")
	if !strings.Contains(out, "Fix quotes 'and' spaces\t") {
		t.Fatal(out)
	}
	run(t, cwd, state, 0, "session", "set", "--project", "feat/completion")
	out = complete(state, "issues", "get", "--session", "default", "Fi")
	if !strings.Contains(out, "Fix quotes 'and' spaces\t") {
		t.Fatal(out)
	}
	for _, dir := range []string{freshState, freshCwd} {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatal("completion created state", dir, entries, err)
		}
	}
}

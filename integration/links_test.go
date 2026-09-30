package integration

import "testing"

func TestDependencyFrontierCLI(t *testing.T) {
	data, state, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	s := start(t, data)
	run(t, cwd, state, 0, "connect", "--endpoint", s.endpoint, "--format", "json")
	run(t, cwd, state, 0, "projects", "create", "--project-title", "feat/frontier", "--repository", "repo-a", "--project-title", "feat/other")
	run(t, cwd, state, 0, "session", "set", "--project", "feat/frontier")
	run(t, cwd, state, 0, "issues", "create", "--issue", "Parent", "--issue", "Blocker")
	run(t, cwd, state, 0, "issues", "create", "--issue", "Work", "--parent", "Parent", "--label", "ready", "--label", "agent")
	run(t, cwd, state, 0, "issues", "create", "--project", "feat/other", "--issue", "Foreign")
	run(t, cwd, state, 0, "issues", "link", "--from", "Blocker", "--to", "Work", "--to", "Work", "--relation", "blocks", "--from", "Work", "--to", "feat/other:Foreign", "--relation", "related")
	frontier := func(want int) {
		t.Helper()
		r := run(t, cwd, state, 0, "issues", "list", "--state", "open", "--blocked", "false", "--labels-all", "ready", "--labels-any", "agent", "--labels-none", "skip", "--parent", "Parent", "--assignee", "none")
		if len(r["items"].([]any)) != want {
			t.Fatal(r)
		}
	}
	frontier(0)
	run(t, cwd, state, 0, "issues", "close", "Parent")
	frontier(0)
	run(t, cwd, state, 0, "issues", "close", "Blocker")
	frontier(1)
	run(t, cwd, state, 0, "issues", "reopen", "Blocker")
	frontier(0)
	run(t, cwd, state, 2, "issues", "link", "--from", "Work", "--to", "Blocker", "--relation", "blocks")
	run(t, cwd, state, 0, "issues", "unlink", "--from", "Work", "--to", "Blocker", "--relation", "blocked-by")
	frontier(1)
	r := run(t, cwd, state, 0, "projects", "list", "--repository-ref", "repo-a", "--q", "FRONTIER")
	if len(r["items"].([]any)) != 1 {
		t.Fatal(r)
	}
	r = run(t, cwd, state, 0, "issues", "list", "--all-projects", "--sort", "title", "--limit", "1")
	cursor := r["next_cursor"].(string)
	run(t, cwd, state, 2, "issues", "list", "--all-projects", "--sort", "title", "--limit", "1", "--cursor", cursor, "--q", "work")
	r = run(t, cwd, state, 0, "issues", "list", "--all-projects", "--sort", "title", "--limit", "1", "--cursor", cursor)
	if len(r["items"].([]any)) != 1 {
		t.Fatal(r)
	}
	s.stop(t)
	s = start(t, data)
	run(t, cwd, state, 0, "issues", "list", "--endpoint", s.endpoint, "--all-projects", "--all")
}

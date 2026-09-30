package integration

import (
	"testing"
	"time"
)

func TestClaimsCLI(t *testing.T) {
	data, state, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	s := start(t, data)
	run(t, cwd, state, 0, "connect", "--endpoint", s.endpoint, "--format", "json", "--actor-name", "Maintainer")
	run(t, cwd, state, 0, "projects", "create", "--project-title", "feat/claims", "--project-title", "feat/other")
	run(t, cwd, state, 0, "session", "set", "--project", "feat/claims")
	run(t, cwd, state, 0, "issues", "create", "--issue", "First", "--issue", "Second")
	run(t, cwd, state, 0, "issues", "create", "--issue", "Other", "--project", "feat/other")
	r := run(t, cwd, state, 0, "claims", "get", "First", "Second")
	if len(r["items"].([]any)) != 2 || r["items"].([]any)[0].(map[string]any)["claim"] != nil {
		t.Fatal(r)
	}
	r = run(t, cwd, state, 0, "claims", "acquire", "First", "feat/other:Other")
	first := r["items"].([]any)[0].(map[string]any)["claim"].(map[string]any)
	expiry := first["expires_at"].(string)
	acquired, e := time.Parse(time.RFC3339Nano, first["acquired_at"].(string))
	if e != nil {
		t.Fatal(e)
	}
	expires, e := time.Parse(time.RFC3339Nano, expiry)
	if e != nil {
		t.Fatal(e)
	}
	if d := expires.Sub(acquired); d < 29*time.Minute || d > 30*time.Minute {
		t.Fatal(d)
	}
	r = run(t, cwd, state, 0, "claims", "list")
	if len(r["items"].([]any)) != 2 {
		t.Fatal(r)
	}
	run(t, cwd, state, 3, "claims", "acquire", "First")
	r = run(t, cwd, state, 0, "claims", "get", "First")
	if r["items"].([]any)[0].(map[string]any)["claim"].(map[string]any)["expires_at"] != expiry {
		t.Fatal("acquire renewed", r)
	}
	r = run(t, cwd, state, 0, "claims", "renew", "--all", "--for", "45m")
	if len(r["items"].([]any)) != 2 {
		t.Fatal(r)
	}
	for _, raw := range r["items"].([]any) {
		if raw.(map[string]any)["claim"].(map[string]any)["expires_at"] == expiry {
			t.Fatal(r)
		}
	}
	run(t, cwd, state, 0, "claims", "release", "First", "Second")
	run(t, cwd, state, 0, "claims", "release", "--all")
	r = run(t, cwd, state, 0, "claims", "list")
	if len(r["items"].([]any)) != 0 {
		t.Fatal(r)
	}
	run(t, cwd, state, 0, "claims", "renew", "--all")
	run(t, cwd, state, 0, "claims", "release", "First")
	run(t, cwd, state, 3, "claims", "renew", "First")
	run(t, cwd, state, 2, "claims", "acquire", "First", "--for", "2h")
}

func TestClaimsCLIOwnershipForceAndDisconnect(t *testing.T) {
	data, state, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	s := start(t, data)
	run(t, cwd, state, 0, "connect", "--endpoint", s.endpoint, "--format", "json", "--actor-kind", "agent")
	run(t, cwd, state, 0, "projects", "create", "--project-title", "feat/claimants")
	run(t, cwd, state, 0, "session", "set", "--project", "feat/claimants")
	run(t, cwd, state, 0, "issues", "create", "--issue", "Work", "--issue", "Maintenance", "--state", "closed")
	run(t, cwd, state, 0, "claims", "acquire", "Work", "Maintenance")
	run(t, cwd, state, 0, "connect", "--session", "second", "--endpoint", s.endpoint, "--format", "json", "--project", "feat/claimants", "--actor-kind", "agent")
	run(t, cwd, state, 3, "claims", "acquire", "Work", "--session", "second")
	run(t, cwd, state, 0, "session", "set", "--actor-kind", "human", "--session", "second")
	run(t, cwd, state, 0, "claims", "acquire", "Work", "--force", "--session", "second")
	run(t, cwd, state, 3, "claims", "release", "Work")
	run(t, cwd, state, 0, "disconnect", "--session", "second")
	r := run(t, cwd, state, 0, "claims", "get", "Work")
	if r["items"].([]any)[0].(map[string]any)["claim"] != nil {
		t.Fatal(r)
	}
}

func TestClaimsCLIDurableWritesAndRestart(t *testing.T) {
	data, state, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	s := start(t, data)
	run(t, cwd, state, 0, "connect", "--endpoint", s.endpoint, "--format", "json", "--actor-kind", "agent")
	run(t, cwd, state, 0, "projects", "create", "--project-title", "feat/ownership")
	run(t, cwd, state, 0, "session", "set", "--project", "feat/ownership")
	run(t, cwd, state, 0, "issues", "create", "--issue", "Work")
	acquired := run(t, cwd, state, 0, "claims", "acquire", "Work")
	original := acquired["items"].([]any)[0].(map[string]any)["claim"].(map[string]any)
	run(t, cwd, state, 0, "issues", "update", "Work", "--content", "Checkpoint")
	after := run(t, cwd, state, 0, "claims", "get", "Work")
	claim := after["items"].([]any)[0].(map[string]any)["claim"].(map[string]any)
	if claim["token"] != original["token"] || claim["expires_at"] != original["expires_at"] {
		t.Fatal(after)
	}
	s.stop(t)
	s = start(t, data)
	run(t, cwd, state, 0, "connect", "--endpoint", s.endpoint)
	resumed := run(t, cwd, state, 0, "claims", "get", "Work")
	resumedClaim := resumed["items"].([]any)[0].(map[string]any)["claim"].(map[string]any)
	if resumedClaim["token"] != original["token"] || resumedClaim["expires_at"] != original["expires_at"] {
		t.Fatal("restart changed lease", resumed)
	}
	run(t, cwd, state, 0, "connect", "--session", "rival", "--endpoint", s.endpoint, "--format", "json", "--project", "feat/ownership", "--actor-kind", "agent")
	run(t, cwd, state, 3, "issues", "update", "Work", "--content", "Conflict", "--session", "rival")
	run(t, cwd, state, 0, "comments", "create", "--issue", "Work", "--content", "A question", "--session", "rival")
	run(t, cwd, state, 3, "issues", "create", "--issue", "Child", "--parent", "Work", "--session", "rival")
	// Force is explicit; actor.kind is descriptive metadata, not an auth role.
	run(t, cwd, state, 0, "issues", "update", "Work", "--content", "Human takeover", "--force", "--session", "rival")
	run(t, cwd, state, 0, "claims", "acquire", "Work", "--session", "rival")
	run(t, cwd, state, 0, "issues", "close", "Work", "--session", "rival")
	after = run(t, cwd, state, 0, "claims", "get", "Work")
	if after["items"].([]any)[0].(map[string]any)["claim"] != nil {
		t.Fatal(after)
	}
	// Closed maintenance acquisition is allowed, independent of workflow frontier.
	run(t, cwd, state, 0, "claims", "acquire", "Work")
	run(t, cwd, state, 0, "disconnect")
	run(t, cwd, state, 0, "connect")
	after = run(t, cwd, state, 0, "claims", "list")
	if len(after["items"].([]any)) != 0 {
		t.Fatal(after)
	}
}
func TestClaimsCLIListPagesAndIssueFilters(t *testing.T) {
	data, state, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	s := start(t, data)
	run(t, cwd, state, 0, "connect", "--endpoint", s.endpoint, "--format", "json")
	run(t, cwd, state, 0, "projects", "create", "--project-title", "feat/pages")
	run(t, cwd, state, 0, "session", "set", "--project", "feat/pages")
	run(t, cwd, state, 0, "issues", "create", "--issue", "A", "--issue", "B")
	run(t, cwd, state, 0, "claims", "acquire", "title:A", "title:B")
	first := run(t, cwd, state, 0, "claims", "list", "--limit", "1")
	if len(first["items"].([]any)) != 1 || first["next_cursor"] == nil {
		t.Fatal(first)
	}
	second := run(t, cwd, state, 0, "claims", "list", "--limit", "1", "--cursor", first["next_cursor"].(string))
	if len(second["items"].([]any)) != 1 || second["next_cursor"] != nil {
		t.Fatal(second)
	}
	id := first["items"].([]any)[0].(map[string]any)["issue_id"].(string)
	one := run(t, cwd, state, 0, "claims", "list", "--issue-id", id)
	if len(one["items"].([]any)) != 1 {
		t.Fatal(one)
	}
	all := run(t, cwd, state, 0, "claims", "list", "--all")
	if len(all["items"].([]any)) != 2 {
		t.Fatal(all)
	}
}

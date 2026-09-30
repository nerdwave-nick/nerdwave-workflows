package cli

import "testing"

func TestGrepArguments(t *testing.T) {
	for _, argv := range [][]string{{"grep", "needle", "-n", "-C", "2", "--all-projects"}, {"--format", "json", "grep", "--", "--help"}, {"grep", "--", "-literal"}} {
		a, e := Parse(argv)
		if e != nil || a.Command != "grep" || len(a.Positionals) != 1 {
			t.Fatalf("%v: %+v %v", argv, a, e)
		}
	}
	for _, argv := range [][]string{{"grep", ""}, {"grep", "a", "b"}, {"grep", "a", "-C", "-1"}, {"grep", "a", "--project", "feat/a", "--all-projects"}, {"grep", "a", "--limit", "1001"}, {"grep", "a", "--cursor", ""}, {"grep", "a", "-n", "-n"}} {
		if _, e := Parse(argv); e == nil {
			t.Fatalf("accepted %v", argv)
		}
	}
}

func TestGrepCompactContext(t *testing.T) {
	for _, flag := range []string{"-C2", "-C=2"} {
		a, err := Parse([]string{"grep", "needle", flag, "--format", "json"})
		if err != nil || a.One("context") != "2" || a.One("format") != "json" {
			t.Fatalf("%s: %#v %v", flag, a, err)
		}
	}
	for _, args := range [][]string{{"grep", "needle", "-C2", "--context", "3"}, {"grep", "needle", "-C-1"}, {"grep", "needle", "-Cbad"}} {
		if _, err := Parse(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	a, err := Parse([]string{"grep", "--", "-C2"})
	if err != nil || a.Has("context") || a.Positionals[0] != "-C2" {
		t.Fatalf("literal: %#v %v", a, err)
	}
}

func TestReservedFlagContentUsesEquals(t *testing.T) {
	for _, value := range []string{"--session", "--endpoint", "--format", "--runtime-vendor", "--runtime-session-id"} {
		a, err := Parse([]string{"issues", "create", "--issue", "Test", "--content=" + value})
		if err != nil || len(a.Groups) != 1 || a.Groups[0]["content"][0] != value {
			t.Fatalf("equals %s: %#v %v", value, a, err)
		}
		if _, err := Parse([]string{"issues", "create", "--issue", "Test", "--content", value}); err == nil {
			t.Fatalf("split option-like value unexpectedly accepted: %s", value)
		}
	}
}

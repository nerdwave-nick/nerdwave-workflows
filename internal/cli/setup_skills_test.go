package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupSkillsCommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Chdir(t.TempDir())
	t.Setenv("LIT_SESSION", "does-not-exist")
	t.Setenv("LIT_ENDPOINT", "http://127.0.0.1:1")
	p := t.TempDir()
	var out, err bytes.Buffer
	if code := Run([]string{"setup-skills", "--scope", "custom", "--path", p, "--agent", "both"}, &out, &err); code != 0 {
		t.Fatalf("%d %s", code, &err)
	}
	for _, h := range []string{"codex", "claude"} {
		if _, e := os.Stat(filepath.Join(p, "."+h, "skills", "lit", "SKILL.md")); e != nil {
			t.Fatal(e)
		}
	}
}
func TestSetupSkillsFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Chdir(t.TempDir())
	for _, args := range [][]string{{}, {"--agent", "codex"}, {"--scope", "custom", "--agent", "codex"}, {"--scope", "local", "--agent", "bad"}, {"--scope", "user", "--agent", "codex", "--path", ""}, {"--scope", "custom", "--agent", "codex", "--path", "x", "extra"}, {"--scope", "local", "--agent", "codex", "--agent", "claude"}, {"--unknown"}} {
		var b bytes.Buffer
		if code := Run(append([]string{"setup-skills"}, args...), &b, &b); code != 2 {
			t.Fatalf("%v: %d %s", args, code, &b)
		}
	}
	var b bytes.Buffer
	if code := Run([]string{"setup-skills", "--help"}, &b, &b); code != 0 {
		t.Fatal(code)
	}
}
func TestSetupSkillsLocalUser(t *testing.T) {
	p := t.TempDir()
	t.Chdir(p)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CODEX_HOME", t.TempDir())
	for _, scope := range []string{"local", "user"} {
		var b bytes.Buffer
		if c := Run([]string{"setup-skills", "--scope", scope, "--agent", "codex"}, &b, &b); c != 0 {
			t.Fatal(c, b.String())
		}
	}
	for _, root := range []string{p, home} {
		if _, e := os.Stat(filepath.Join(root, ".codex", ".lit-skills.json")); e != nil {
			t.Fatal(e)
		}
	}
}

func TestGlobalHelpIncludesSkillSetup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	var b bytes.Buffer
	if code := Run([]string{"--help"}, &b, &b); code != 0 || !strings.Contains(b.String(), "setup-skills") {
		t.Fatalf("%d %s", code, &b)
	}
}

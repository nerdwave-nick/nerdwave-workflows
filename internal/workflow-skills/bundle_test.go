package workflowskills

import (
	"io/fs"
	"path"
	"strings"
	"testing"
)

func TestBundleContainsDeployableSuiteOnly(t *testing.T) {
	bundle := Bundle()
	expected := []string{"lit", "planner", "clarify", "modeling", "challenge", "consult", "research", "prototype", "to-spec", "to-tickets", "impl", "triage", "codebase", "what", "i-have-adhd"}
	for _, name := range expected {
		content, err := fs.ReadFile(bundle, "skills/"+name+"/SKILL.md")
		if err != nil || len(content) == 0 {
			t.Errorf("missing skill %s: %v", name, err)
		}
	}
	if _, err := fs.ReadFile(bundle, "AGENTS_MD_TEMPLATE.md"); err != nil {
		t.Fatal(err)
	}
	if err := fs.WalkDir(bundle, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		if name != "AGENTS_MD_TEMPLATE.md" && name != "skills" && !strings.HasPrefix(name, "skills/") {
			t.Errorf("unexpected bundle path %s", name)
		}
		if strings.Contains(name, "/tests") || strings.Contains(name, "/scripts") || strings.Contains(name, "__pycache__") {
			t.Errorf("nondeployable path %s", name)
		}
		if !entry.IsDir() && path.Ext(name) != ".md" && path.Ext(name) != ".yaml" {
			t.Errorf("unexpected asset %s", name)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestActiveBundleUsesCurrentExecutables(t *testing.T) {
	bundle := Bundle()
	err := fs.WalkDir(bundle, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, err := fs.ReadFile(bundle, name)
		if err != nil {
			return err
		}
		for _, obsolete := range []string{"lit-cli", "litd"} {
			if strings.Contains(string(content), obsolete) {
				t.Errorf("active instruction %s contains obsolete executable %q", name, obsolete)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := fs.ReadFile(bundle, "skills/lit/SKILL.md")
	if err != nil || !strings.Contains(string(content), "client executable is `lit`") || !strings.Contains(string(content), "server executable is `lit-server`") {
		t.Fatalf("missing current roles: %v: %s", err, content)
	}
}

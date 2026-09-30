package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicCommandIdentities(t *testing.T) {
	home := t.TempDir()
	for _, tc := range []struct{ binary, usage, command string }{{cliBin, "lit [", "projects"}, {litBin, "lit-server [", "--listen"}} {
		cmd := exec.Command(tc.binary, "--help")
		cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "LIT_CONFIG_FILE="+filepath.Join(home, "absent"), "LIT_STATE_DIR="+filepath.Join(home, "state"), "LIT_WORKFLOW_STATE_DIR="+filepath.Join(home, "workflow"))
		output, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(output), tc.usage) || !strings.Contains(string(output), tc.command) || strings.Contains(string(output), "lit-cli") {
			t.Fatalf("%s: %v: %s", tc.binary, err, output)
		}
	}
	for _, shell := range []string{"bash", "fish", "zsh"} {
		cmd := exec.Command(cliBin, "completion", shell)
		cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home)
		output, err := cmd.CombinedOutput()
		if err != nil || strings.Contains(string(output), "lit-cli") || !strings.Contains(string(output), "lit") {
			t.Fatalf("%s completion: %v: %s", shell, err, output)
		}
	}
}

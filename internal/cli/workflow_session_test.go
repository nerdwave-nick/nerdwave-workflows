package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowSessionHelp(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "file")
	if e := os.WriteFile(blocked, []byte("blocked"), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("LIT_STATE_DIR", filepath.Join(blocked, "state"))
	t.Setenv("LIT_WORKFLOW_STATE_DIR", filepath.Join(blocked, "workflow"))
	for _, args := range [][]string{{"--help"}, {"workflow-session", "--help"}} {
		var out, stderr bytes.Buffer
		if code := Run(args, &out, &stderr); code != 0 || !strings.Contains(out.String(), "workflow-session") {
			t.Fatalf("%v: code %d out %s err %s", args, code, &out, &stderr)
		}
	}
}

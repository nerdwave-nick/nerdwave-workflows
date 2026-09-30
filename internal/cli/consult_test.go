package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConsultIsNotCLICommand(t *testing.T) {
	state := filepath.Join(t.TempDir(), "untouched")
	t.Setenv("LIT_STATE_DIR", state)
	t.Setenv("LIT_WORKFLOW_STATE_DIR", state)
	var out, errOut bytes.Buffer
	if code := Run([]string{"consult", "--request", filepath.Join(state, "request.json")}, &out, &errOut); code == 0 {
		t.Fatal("consult unexpectedly succeeded")
	}
	var result struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(errOut.Bytes(), &result); err != nil || result.Error.Code != "invalid_arguments" {
		t.Fatalf("expected normal unsupported-command error: %s (%v)", errOut.String(), err)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("unsupported command touched state: %v", err)
	}
	out.Reset()
	errOut.Reset()
	if Run([]string{"--help"}, &out, &errOut) != 0 || strings.Contains(out.String(), "consult") {
		t.Fatal("help still advertises consultation:", out.String(), errOut.String())
	}
}

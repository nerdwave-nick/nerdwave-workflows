package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceScopedHelpIsOffline(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("LIT_CONFIG_FILE", filepath.Join(root, "missing.json"))
	t.Setenv("LIT_DATA_DIR", filepath.Join(root, "uncreated"))
	t.Setenv("LIT_LISTEN", "invalid")
	for _, args := range [][]string{{"-h"}, {"--help"}, {"help"}, {"--listen", "invalid", "help"}} {
		var out, stderr bytes.Buffer
		if code := runCLI(args, &out, &stderr); code != 0 || stderr.Len() != 0 {
			t.Fatalf("%v: %d %s", args, code, &stderr)
		}
		if strings.Contains(out.String(), "Available Commands:") {
			t.Fatalf("daemon still advertises subcommands: %s", &out)
		}
		for _, want := range []string{"--config", "--data-dir", "--listen", "127.0.0.1:7411", "optional", "loopback", "LIT_CONFIG_FILE", "socket activation", "Examples:"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%v missing %q: %s", args, want, &out)
			}
		}
	}
	var out, stderr bytes.Buffer
	if code := runCLI([]string{"--version"}, &out, &stderr); code != 0 || strings.TrimSpace(out.String()) == "" || stderr.Len() != 0 {
		t.Fatalf("version: %d %s %s", code, &out, &stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "uncreated")); !os.IsNotExist(err) {
		t.Fatalf("help created data: %v", err)
	}
}

func TestServiceArgumentErrorsBeforeStartup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("LIT_DATA_DIR", filepath.Join(root, "uncreated"))
	t.Setenv("LIT_CONFIG_FILE", filepath.Join(root, "missing"))
	for _, args := range [][]string{{"--bogus"}, {"serve"}, {"version"}, {"completion", "bash"}, {"help", "extra"}, {"--", "help"}, {"--listen", "127.0.0.1:0", "--listen", "127.0.0.1:1"}, {"unknown"}} {
		var out, stderr bytes.Buffer
		if code := runCLI(args, &out, &stderr); code != 2 || stderr.Len() == 0 || strings.Contains(stderr.String(), "config:") {
			t.Fatalf("%v: %d %s", args, code, &stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "uncreated")); !os.IsNotExist(err) {
		t.Fatalf("invalid arguments created data: %v", err)
	}
}

func TestServiceHelpTokenAsFlagValue(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("LIT_CONFIG_FILE", filepath.Join(root, "absent"))
	t.Chdir(root)
	for _, args := range [][]string{{"--config", "help"}, {"--config=help"}, {"--data-dir", "help"}} {
		var out, stderr bytes.Buffer
		if code := runCLI(args, &out, &stderr); code != 2 || out.Len() != 0 || !strings.Contains(stderr.String(), "config:") {
			t.Fatalf("%v: %d %s %s", args, code, &out, &stderr)
		}
	}
}

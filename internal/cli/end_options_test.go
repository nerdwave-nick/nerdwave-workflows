package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestDomainParsersEndOptions(t *testing.T) {
	for _, prefix := range [][]string{{"issues", "get"}, {"comments", "get"}, {"projects", "get"}, {"claims", "get"}} {
		args := append(append([]string{}, prefix...), "--session", "test", "--", "help", "--format", "--", "get")
		got, err := Parse(args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !reflect.DeepEqual(got.Positionals, []string{"help", "--format", "--", "get"}) || got.Has("format") || got.One("session") != "test" {
			t.Fatalf("%v: %#v", args, got)
		}
	}
}

func TestLiteralHelpTitleThroughFramework(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("LIT_STATE_DIR", t.TempDir())
	t.Chdir(t.TempDir())
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(503) }))
	defer srv.Close()
	common := []string{"--session", "test", "--endpoint", srv.URL}
	var out, stderr bytes.Buffer
	if code := Run(append(append([]string{}, common...), "issues", "get", "help"), &out, &stderr); code != 0 || !strings.Contains(out.String(), "Usage:") || requests != 0 {
		t.Fatalf("help: %d %s %s requests=%d", code, &out, &stderr, requests)
	}
	out.Reset()
	stderr.Reset()
	Run(append(append([]string{}, common...), "issues", "get", "--", "help"), &out, &stderr)
	if requests != 1 || strings.Contains(out.String(), "Usage:") {
		t.Fatalf("literal: %s %s requests=%d", &out, &stderr, requests)
	}
}

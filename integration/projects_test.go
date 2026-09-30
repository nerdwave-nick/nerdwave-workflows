package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectsCLI(t *testing.T) {
	data, state, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	s := start(t, data)
	run(t, cwd, state, 0, "connect", "--endpoint", s.endpoint, "--format", "json", "--actor-name", "tester")
	body := "# Exact\r\n雪 <>&\n"
	r := run(t, cwd, state, 0, "projects", "create", "--project-title", "feat/one", "--content", body, "--repository", "repo:z", "--repository", "repo:a")
	if r["outcome"] != "applied" {
		t.Fatal(r)
	}
	r = run(t, cwd, state, 0, "projects", "get", "feat/one")
	p := r["items"].([]any)[0].(map[string]any)
	id := p["id"].(string)
	if p["description"] != body {
		t.Fatal(p)
	}
	run(t, cwd, state, 0, "session", "set", "--project", "feat/one")
	run(t, cwd, state, 0, "projects", "get")
	f := filepath.Join(cwd, "batch.json")
	b, _ := json.Marshal([]any{map[string]any{"target": id, "set": map[string]any{"title": "feat/two"}}, map[string]any{"target": id, "add": map[string]any{"repository_refs": []string{"repo:b"}}}})
	os.WriteFile(f, b, 0600)
	run(t, cwd, state, 0, "projects", "update", "--file", f)
	run(t, cwd, state, 3, "projects", "update", id, "--revision", "1", "--content", "stale")
	r = run(t, cwd, state, 0, "projects", "history", id)
	if len(r["items"].([]any)) != 2 {
		t.Fatal(r)
	}
	run(t, cwd, state, 0, "projects", "update", id, "--title", "feat/two")
	r = run(t, cwd, state, 0, "projects", "history", id)
	if len(r["items"].([]any)) != 2 {
		t.Fatal("noop history", r)
	}
	s.stop(t)
	s = start(t, data)
	r = run(t, cwd, state, 0, "projects", "get", id, "--endpoint", s.endpoint)
	if r["items"].([]any)[0].(map[string]any)["revision"] != float64(2) {
		t.Fatal(r)
	}
}

func TestProjectBatchesAndLookups(t *testing.T) {
	data, state, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	s := start(t, data)
	run(t, cwd, state, 0, "connect", "--endpoint", s.endpoint, "--format", "json")
	run(t, cwd, state, 0, "projects", "create", "--project-title", "feat/a", "--content", "A", "--project-title", "feat/b", "--content", "B")
	page := run(t, cwd, state, 0, "projects", "list", "--sort", "title", "--limit", "1")
	if len(page["items"].([]any)) != 1 || page["next_cursor"] == nil {
		t.Fatal(page)
	}
	run(t, cwd, state, 2, "projects", "list", "--sort", "updated-at", "--limit", "1", "--cursor", page["next_cursor"].(string))
	page2 := run(t, cwd, state, 0, "projects", "list", "--sort", "title", "--limit", "1", "--cursor", page["next_cursor"].(string))
	if len(page2["items"].([]any)) != 1 || page2["next_cursor"] != nil {
		t.Fatal(page2)
	}
	r := run(t, cwd, state, 0, "projects", "get", "FEAT/A", "title:feat/b", "feat/a")
	if len(r["items"].([]any)) != 2 {
		t.Fatal(r)
	}
	a := r["items"].([]any)[0].(map[string]any)["id"].(string)
	b := r["items"].([]any)[1].(map[string]any)["id"].(string)
	run(t, cwd, state, 0, "projects", "get", "id:"+strings.ToUpper(strings.ReplaceAll(a, "-", "")))
	run(t, cwd, state, 0, "--project", "feat/a", "projects", "get")
	run(t, cwd, state, 0, "projects", "update", "--project-id", a, "--title", "feat/b", "--project-id", b, "--title", "feat/a")
	run(t, cwd, state, 2, "projects", "update", "--project-id", a, "--title", "feat/conflict", "--project-id", a, "--title", "feat/other")
	run(t, cwd, state, 2, "projects", "update", "--project-id", a, "--add-repository", "same", "--project-id", a, "--remove-repository", "same")
	run(t, cwd, state, 2, "projects", "update", "--project-id", a, "--clear", "repositories", "--project-id", a, "--add-repository", "same")
	run(t, cwd, state, 2, "projects", "update", a, "--title", "feat/a")
	r = run(t, cwd, state, 0, "projects", "get", a, b)
	for _, p := range r["items"].([]any) {
		if p.(map[string]any)["revision"] != float64(2) {
			t.Fatal("partial failed batch", r)
		}
	}
	run(t, cwd, state, 0, "projects", "update", a, b, "--content", "shared")
	run(t, cwd, state, 2, "projects", "get", a, "--timeout", "nonsense")
	run(t, cwd, state, 2, "projects", "list", "--file", filepath.Join(cwd, "missing"))
	r = run(t, cwd, state, 0, "projects", "list", "--all")
	if len(r["items"].([]any)) != 2 {
		t.Fatal(r)
	}
}

func TestProjectStdinAndPending(t *testing.T) {
	data, state, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	s := start(t, data)
	run(t, cwd, state, 0, "connect", "--endpoint", s.endpoint, "--format", "json")
	command := exec.Command(cliBin, "projects", "create", "--file", "-")
	command.Dir = cwd
	command.Env = append(os.Environ(), "LIT_SESSION=default", "LIT_STATE_DIR="+state, "GORACE=atexit_sleep_ms=0")
	command.Stdin = strings.NewReader(`[{"title":"feat/stdin","content":"body"}]`)
	if out, e := command.CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	run(t, cwd, state, 2, "projects", "create", "--project-title", "feat/one", "--content-file", "-", "--project-title", "feat/two", "--content-file", "-")
	// A pending-directory failure must prevent the outgoing durable mutation.
	if e := os.RemoveAll(filepath.Join(state, "pending")); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(state, "pending"), []byte("not a directory"), 0600); e != nil {
		t.Fatal(e)
	}
	run(t, cwd, state, 1, "projects", "update", "feat/stdin", "--content", "must not send")
	r := run(t, cwd, state, 0, "projects", "get", "feat/stdin")
	if r["items"].([]any)[0].(map[string]any)["description"] != "body" {
		t.Fatal(r)
	}
}

func TestProjectLostResponseRetainsExactRequest(t *testing.T) {
	data, state, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	s := start(t, data)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/transaction-status" {
			http.Error(w, "reconciliation unavailable", 503)
			return
		}
		outgoing, _ := http.NewRequest(r.Method, s.endpoint+r.URL.RequestURI(), r.Body)
		outgoing.Header = r.Header.Clone()
		response, e := http.DefaultClient.Do(outgoing)
		if e != nil {
			t.Error(e)
			return
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		if r.URL.Path == "/v1/transactions" {
			if response.StatusCode != 200 {
				t.Errorf("mutation did not commit: %d %s", response.StatusCode, body)
			}
			connection, _, e := w.(http.Hijacker).Hijack()
			if e == nil {
				connection.Close()
			}
			return
		}
		for k, v := range response.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(response.StatusCode)
		w.Write(body)
	}))
	defer proxy.Close()
	run(t, cwd, state, 0, "connect", "--endpoint", proxy.URL, "--format", "json")
	run(t, cwd, state, 4, "projects", "create", "--project-title", "feat/lost", "--content", "exact\r\n")
	files, e := filepath.Glob(filepath.Join(state, "pending", "*.json"))
	if e != nil || len(files) != 1 {
		t.Fatal("pending evidence absent", files, e)
	}
	raw, _ := os.ReadFile(files[0])
	var pending struct {
		SchemaVersion int             `json:"schema_version"`
		Body          json.RawMessage `json:"body"`
	}
	if e = json.Unmarshal(raw, &pending); e != nil {
		t.Fatal(e)
	}
	var request struct {
		Intent struct {
			Operations []struct {
				ID string `json:"id"`
			} `json:"operations"`
		} `json:"intent"`
		Hash string `json:"request_hash"`
	}
	if e = json.Unmarshal(pending.Body, &request); e != nil {
		t.Fatal(e, string(raw))
	}
	if pending.SchemaVersion != 1 || len(request.Intent.Operations) != 1 || len(request.Hash) != 64 {
		t.Fatal(string(raw))
	}
	r := run(t, cwd, state, 0, "projects", "history", request.Intent.Operations[0].ID, "--request-hash", request.Hash, "--endpoint", s.endpoint)
	if len(r["items"].([]any)) != 1 {
		t.Fatal(r)
	}
	r = run(t, cwd, state, 0, "projects", "get", "feat/lost", "--endpoint", s.endpoint)
	if r["items"].([]any)[0].(map[string]any)["description"] != "exact\r\n" {
		t.Fatal(r)
	}
}

func TestProjectFileRejectsEmptyInapplicableFields(t *testing.T) {
	data, state, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	s := start(t, data)
	run(t, cwd, state, 0, "connect", "--endpoint", s.endpoint, "--format", "json")
	run(t, cwd, state, 0, "projects", "create", "--project-title", "feat/existing")
	for _, test := range []struct{ verb, body string }{{"create", `[{"title":"feat/new","clear":[]}]`}, {"create", `[{"title":"feat/new","target":""}]`}, {"create", `[{"title":"feat/new","set":{}}]`}, {"update", `[{"target":"feat/existing","repository_refs":[]}]`}, {"update", `[{"target":"feat/existing","content":null}]`}} {
		path := filepath.Join(cwd, "invalid.json")
		os.WriteFile(path, []byte(test.body), 0600)
		run(t, cwd, state, 2, "projects", test.verb, "--file", path)
	}
	r := run(t, cwd, state, 0, "projects", "list", "--all")
	if len(r["items"].([]any)) != 1 {
		t.Fatal(r)
	}
}

func TestProjectHistoryPaginationAndReservedSegments(t *testing.T) {
	data, state, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	s := start(t, data)
	run(t, cwd, state, 0, "connect", "--endpoint", s.endpoint, "--format", "json")
	run(t, cwd, state, 0, "projects", "create", "--project-title", "feat/history", "--content", "old")
	run(t, cwd, state, 0, "projects", "get", "feat/history")
	run(t, cwd, state, 0, "projects", "update", "feat/history", "--content", "new")
	r := run(t, cwd, state, 0, "projects", "history", "feat/history", "--limit", "1")
	if len(r["items"].([]any)) != 1 || r["next_cursor"] == nil {
		t.Fatal(r)
	}
	r = run(t, cwd, state, 0, "projects", "history", "feat/history", "--limit", "1", "--cursor", r["next_cursor"].(string))
	if len(r["items"].([]any)) != 1 || r["next_cursor"] != nil {
		t.Fatal(r)
	}
}

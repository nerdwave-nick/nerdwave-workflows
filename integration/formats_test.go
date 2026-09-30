package integration

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestThreeFormatsPersistAcrossRestartAndLegacyResume(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("LIT_ENDPOINT", "")
	t.Setenv("LIT_SESSION", "")
	state, data := filepath.Join(root, "state"), filepath.Join(root, "data")
	s := start(t, data)
	for _, format := range []string{"cli", "markdown", "json", "human"} {
		t.Run(format, func(t *testing.T) {
			var id string
			if format == "human" {
				// An existing v1 client made by the former CLI must resume, not register
				// another identity. The legacy API spelling stays accepted for old peers.
				resp, err := http.Post(s.endpoint+"/v1/connect", "application/json", strings.NewReader(`{"actor":{"name":"Legacy","kind":"human"},"output_format":"human"}`))
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				var body map[string]any
				if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if resp.StatusCode != 201 {
					t.Fatal(body)
				}
				id = body["data"].(map[string]any)["client"].(map[string]any)["client_id"].(string)
				code, out, stderr := rawCLI(t, root, state, "connect", "--session", format, "--endpoint", s.endpoint, "--client-id", id)
				if code != 0 || stderr != "" || !strings.Contains(out, id) {
					t.Fatal(code, out, stderr)
				}
				c := item(formatJSONRun(t, root, state, 0, "session", "get", "--session", format))
				if c["client_id"] != id || c["output_format"] != "human" || c["state_revision"] != float64(1) {
					t.Fatal(c)
				}
			} else {
				c := item(formatJSONRun(t, root, state, 0, "connect", "--session", format, "--endpoint", s.endpoint, "--output-format", format))
				id = c["client_id"].(string)
				if c["output_format"] != format {
					t.Fatal(c)
				}
			}
			code, out, stderr := rawCLI(t, root, state, "session", "get", "--session", format)
			if code != 0 || stderr != "" || !strings.Contains(out, id) {
				t.Fatal(code, out, stderr)
			}
			if format == "json" && !json.Valid([]byte(out)) {
				t.Fatal(out)
			}
			if format != "json" && json.Valid([]byte(out)) {
				t.Fatal("unexpected JSON", out)
			}
			// One-off formatting must not overwrite the durable preference.
			c := item(formatJSONRun(t, root, state, 0, "session", "get", "--session", format, "--format", "json"))
			if c["output_format"] != format || c["client_id"] != id {
				t.Fatal(c)
			}
		})
	}
	s.stop(t)
	s = start(t, data)
	for _, format := range []string{"cli", "markdown", "json", "human"} {
		c := item(formatJSONRun(t, root, state, 0, "session", "get", "--session", format, "--endpoint", s.endpoint))
		if c["output_format"] != format || c["state_revision"] != float64(1) {
			t.Fatal("restart changed stored preference", c)
		}
	}
	// CLI public spellings reject legacy/unknown values before registration.
	for _, value := range []string{"human", "invalid"} {
		formatJSONRun(t, root, state, 2, "connect", "--session", "invalid-"+value, "--endpoint", s.endpoint, "--output-format", value)
	}
	// Update then read the durable property; all public formats round-trip.
	for _, format := range []string{"markdown", "cli", "json"} {
		c := item(formatJSONRun(t, root, state, 0, "session", "set", "--session", "cli", "--endpoint", s.endpoint, "--output-format", format))
		if c["output_format"] != format {
			t.Fatal(c)
		}
	}
	// Offline errors follow the saved format, with explicit JSON always available.
	s.stop(t)
	for _, format := range []string{"markdown", "human"} {
		code, out, stderr := rawCLI(t, root, state, "session", "get", "--session", format, "--endpoint", s.endpoint)
		if code != 1 || out != "" || !strings.Contains(html.UnescapeString(stderr), "service_unavailable") || json.Valid([]byte(stderr)) {
			t.Fatal(fmt.Sprintf("%s: %d %s %s", format, code, out, stderr))
		}
		formatJSONRun(t, root, state, 1, "session", "get", "--session", format, "--endpoint", s.endpoint, "--format", "json")
	}
}

func formatJSONRun(t *testing.T, cwd, state string, want int, args ...string) map[string]any {
	t.Helper()
	for _, arg := range args {
		if arg == "--format" {
			return run(t, cwd, state, want, args...)
		}
	}
	return run(t, cwd, state, want, append(args, "--format", "json")...)
}

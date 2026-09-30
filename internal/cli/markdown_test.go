package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestMarkdownStructuredAndSafe(t *testing.T) {
	a := App{Args: Args{Command: "issues", Verb: "history"}}
	b, e := a.markdownResult(Result{Outcome: "committed", Items: []any{map[string]any{"before": map[string]any{"body": "a|b\\c\n<script>\n```\n# heading\x1b"}, "revision": 2}}})
	if e != nil {
		t.Fatal(e)
	}
	s := string(b)
	for _, want := range []string{"# Issues history", "| Field | Value |", "Before / Body", "a&#124;b&#92;c<br>&lt;script&gt;", "&#96;&#96;&#96;", "Outcome"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q: %s", want, s)
		}
	}
	if strings.Contains(s, "\x1b") || strings.Contains(s, "<script>") || strings.Contains(s, "```") {
		t.Fatal(s)
	}
}
func TestMarkdownListNoTruncation(t *testing.T) {
	a := App{Args: Args{Command: "projects", Verb: "list"}}
	title := strings.Repeat("title", 30)
	b, e := a.markdownResult(Result{Items: []any{map[string]any{"title": title, "id": "full-id", "issue_ids": []string{"a", "b"}}}})
	if e != nil || !strings.Contains(string(b), "| TITLE | ISSUES | REPOSITORIES | ID |") || !strings.Contains(string(b), title) {
		t.Fatalf("%s %v", b, e)
	}
}
func TestCLIWidthAndRedirect(t *testing.T) {
	a := App{Args: Args{Command: "projects", Verb: "list"}, Out: &bytes.Buffer{}}
	r := Result{Items: []any{map[string]any{"title": strings.Repeat("界", 60), "id": "01234567-1234-1234-1234-012345678901"}}}
	wide, e := a.cliResultWidth(r, 120)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(wide), "TITLE") || !strings.Contains(string(wide), "…") {
		t.Fatal(string(wide))
	}
	narrow, e := a.cliResultWidth(r, 40)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(narrow), "Title:") || !strings.Contains(string(narrow), "01234567-1234-1234-1234-012345678901") {
		t.Fatal(string(narrow))
	}
	if terminalColumns(a.Out) != 0 {
		t.Fatal("redirected output treated as terminal")
	}
}
func TestMarkdownEmptyPagination(t *testing.T) {
	c := "a|b\n# bad"
	a := App{Args: Args{Command: "projects", Verb: "list"}}
	b, e := a.markdownResult(Result{NextCursor: &c, RequestHash: "h", ServerTime: "now"})
	if e != nil || !strings.Contains(string(b), "No projects found.") || !strings.Contains(string(b), "Next cursor") || strings.Contains(string(b), "\n# bad") {
		t.Fatalf("%s %v", b, e)
	}
}

func TestMarkdownStructuredKinds(t *testing.T) {
	for _, tc := range []struct {
		command, verb string
		item          any
		want          string
	}{
		{"session", "get", map[string]any{"actor": map[string]any{"name": "Alex"}, "client_id": "client"}, "Actor / Name"},
		{"claims", "acquire", map[string]any{"claim": map[string]any{"owner_client_id": "owner", "expires_at": "tomorrow"}}, "Claim / Owner client ID"},
		{"grep", "", map[string]any{"excerpts": []any{map[string]any{"text": "line one\nline two", "start_line": 7, "excerpt_truncated": true}}}, "line one<br>line two"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			a := App{Args: Args{Command: tc.command, Verb: tc.verb}}
			b, e := a.markdownResult(Result{Items: []any{tc.item}})
			if e != nil || !strings.Contains(string(b), tc.want) {
				t.Fatalf("%s %v", b, e)
			}
		})
	}
}

func TestCLITableAlignsDisplayColumns(t *testing.T) {
	a := App{Args: Args{Command: "issues", Verb: "list"}}
	r := Result{Items: []any{
		map[string]any{"title": "界界", "state": "open", "id": "cjk-id"},
		map[string]any{"title": "abcd", "state": "open", "id": "ascii-id"},
		map[string]any{"title": "e\u0301abc", "state": "open", "id": "combining-id"},
	}}
	b, e := a.cliResultWidth(r, 0)
	if e != nil {
		t.Fatal(e)
	}
	lines := strings.Split(string(b), "\n")
	column := -1
	for _, line := range lines {
		at := strings.Index(line, "open")
		if at < 0 {
			continue
		}
		got := displayColumns(line[:at])
		if column < 0 {
			column = got
		}
		if got != column {
			t.Fatalf("state at column %d, want %d:\n%s", got, column, b)
		}
	}
	if column < 0 {
		t.Fatal(string(b))
	}
}

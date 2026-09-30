package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpDiscoversNativeLinksAndQueries(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		wants []string
	}{
		{[]string{"issues", "link", "--help"}, []string{"--from", "--to", "--relation", "Repeat --from", "--file"}},
		{[]string{"issues", "list", "--help"}, []string{"--labels-all", "--labels-any", "--labels-none", "--blocked", "--assignee", "--parent", "--created-after", "--updated-before", "--all conflicts with --limit/--cursor"}},
		{[]string{"projects", "list", "--help"}, []string{"--repository-ref", "--file"}},
		{[]string{"issues", "history", "--help"}, []string{"history REF", "--limit", "--cursor", "--all"}},
	} {
		var out, err bytes.Buffer
		if code := Run(tc.args, &out, &err); code != 0 || err.Len() != 0 {
			t.Fatal(tc.args, code, err.String())
		}
		for _, want := range tc.wants {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%v help omits %q", tc.args, want)
			}
		}
	}
	// Representative examples discoverable in help must remain accepted by the parser.
	for _, args := range [][]string{{"issues", "link", "--from", "A", "--to", "B", "--to", "C", "--relation", "blocks", "--from", "B", "--to", "C", "--relation", "related"}, {"issues", "unlink", "--file", "links.json"}, {"issues", "list", "--state", "open", "--blocked", "false", "--labels-all", "ready", "--parent", "none", "--all"}, {"issues", "history", "A", "--limit", "5"}} {
		if _, e := Parse(args); e != nil {
			t.Fatal(args, e)
		}
	}
}

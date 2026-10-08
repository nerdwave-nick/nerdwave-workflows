package nwcli

import (
	"strings"
	"testing"
)

// completers records the context each dynamic completion receives.
func completers(seen *Context) map[string]Completer {
	return map[string]Completer{
		"items": func(ctx Context) ([]Candidate, Directive) {
			*seen = ctx
			return []Candidate{{Value: "item one", Description: "First"}}, NoFiles
		},
	}
}

func values(cands []Candidate) string {
	var out []string
	for _, c := range cands {
		out = append(out, c.Value)
	}
	return strings.Join(out, ",")
}

func TestCompleteSubcommands(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{""}, "adapter,help,items,search,setup"},
		{[]string{"it"}, "items"},
		{[]string{"help", ""}, "adapter,help,items,search,setup"},
		{[]string{"--session", "s", "items", ""}, "create,get"},
		{[]string{"adapter", "--host", "h", ""}, "fresh,run"},
		{[]string{"wat", ""}, ""},
	} {
		cands, d := app().Complete(tc.args, nil)
		if values(cands) != tc.want || d != NoFiles {
			t.Errorf("%v: %q %d, want %q", tc.args, values(cands), d, tc.want)
		}
	}
	if cands, _ := app().Complete([]string{"items", ""}, nil); cands[0].Description != "Create items" {
		t.Errorf("subcommand description: %+v", cands)
	}
}

func TestCompleteFlagNamesLikeCobra(t *testing.T) {
	cands, d := app().Complete([]string{"items", "get", "-"}, nil)
	if values(cands) != "--help,-h,--query,-q,--session,--verbose" || d != NoFiles || cands[2].Description != "Filter `TEXT`" {
		t.Fatalf("got %q %d %+v", values(cands), d, cands)
	}
	if cands, _ := app().Complete([]string{"items", "get", "--q"}, nil); values(cands) != "--query" {
		t.Fatalf("prefix: %q", values(cands))
	}
	if cands, _ := app().Complete([]string{"setup", "--"}, nil); values(cands) != "--help,--scope" {
		t.Fatalf("withheld flags offered: %q", values(cands))
	}
}

func TestCompleteBareTabListsFlagsByRepeatability(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"items", "create", ""}, "--item,--note,--project,--tag,--session,--verbose"},
		{[]string{"items", "create", "--project", "p", "--verbose", ""}, "--item,--note,--tag,--session"},
		{[]string{"items", "create", "--item", "a", "--note", "x", "--tag", "t", ""}, "--item,--project,--tag,--session,--verbose"},
		{[]string{"items", "create", "--item", "a", "--note", "x", "--item", "b", ""}, "--item,--note,--project,--tag,--session,--verbose"},
		{[]string{"items", "create", "--note", "x", ""}, "--item,--project,--tag,--session,--verbose"}, // fields of the implicit first item
	} {
		cands, d := app().Complete(tc.args, nil)
		if values(cands) != tc.want || d != NoFiles|KeepOrder {
			t.Errorf("%v: %q %d, want %q", tc.args, values(cands), d, tc.want)
		}
	}
}

func TestCompleteValuesByKind(t *testing.T) {
	root := app()
	create := root.Find("items", "create")
	create.Flags = append(create.Flags,
		Flag{Name: "file", Value: Value{Kind: Path}},
		Flag{Name: "dir", Value: Value{Kind: Dir}},
		Flag{Name: "state", Value: Value{Kind: Choice, Choices: []string{"open", "closed"}}},
		Flag{Name: "ref", Short: "r", Value: Value{Kind: Dynamic, Completer: "items"}},
		Flag{Name: "gone", Value: Value{Kind: Dynamic, Completer: "missing"}},
	)
	var seen Context
	for _, tc := range []struct {
		args []string
		want string
		d    Directive
	}{
		{[]string{"items", "create", "--file", ""}, "", Files},
		{[]string{"items", "create", "--dir", ""}, "", DirsOnly},
		{[]string{"items", "create", "--state", ""}, "open,closed", NoFiles},
		{[]string{"items", "create", "--state", "c"}, "closed", NoFiles},
		{[]string{"items", "create", "--state=o"}, "open", NoFiles},
		{[]string{"items", "create", "--note", ""}, "", NoFiles},
		{[]string{"items", "create", "--gone", ""}, "", NoFiles},
		{[]string{"--session", "s", "items", "create", "--item", "a", "-r", "it"}, "item one", NoFiles},
	} {
		cands, d := root.Complete(tc.args, completers(&seen))
		if values(cands) != tc.want || d != tc.d {
			t.Errorf("%v: %q %d, want %q %d", tc.args, values(cands), d, tc.want, tc.d)
		}
	}
	if strings.Join(seen.Path, " ") != "items create" || seen.Prefix != "it" || seen.Flags["session"][0] != "s" {
		t.Errorf("completer context: %+v", seen)
	}
}

func TestCompleteOperands(t *testing.T) {
	var seen Context
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"items", "get", "i"}, "item one"},
		{[]string{"items", "get", "a", "--query", "q", ""}, "item one"}, // unlimited
		{[]string{"search", ""}, ""},                                    // free-form
		{[]string{"adapter", "run", "--", ""}, ""},
		{[]string{"items", "get", "--", ""}, "item one"},
		{[]string{"items", "get", "--", "--query", ""}, "item one"}, // after -- even dashed words are operands
	} {
		cands, d := app().Complete(tc.args, completers(&seen))
		if values(cands) != tc.want || d != NoFiles {
			t.Errorf("%v: %q %d, want %q", tc.args, values(cands), d, tc.want)
		}
	}
	conflicting := app()
	conflicting.Find("items", "get").Operands.Conflicts = []string{"query"}
	if cands, _ := conflicting.Complete([]string{"items", "get", "--query", "q", ""}, completers(&seen)); len(cands) != 0 {
		t.Errorf("operands completed although --query replaces them: %v", cands)
	}
	limited := app()
	limited.Find("items", "get").Operands.Max = 1
	if cands, _ := limited.Complete([]string{"items", "get", "a", ""}, completers(&seen)); len(cands) != 0 {
		t.Errorf("operand beyond Max completed: %v", cands)
	}
}

func TestWriteCompletionProtocol(t *testing.T) {
	var b strings.Builder
	cands := []Candidate{{Value: "a", Description: "first"}, {Value: "b"}}
	WriteCompletion(&b, cands, NoFiles|KeepOrder, true)
	WriteCompletion(&b, cands, DirsOnly, false)
	if want := "a\tfirst\nb\n:36\na\nb\n:16\n"; b.String() != want {
		t.Fatalf("got %q, want %q", b.String(), want)
	}
}

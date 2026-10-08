package nwcli

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// app is a small tree with lit's shapes: root-wide flags, a group with flags
// of its own, a command opting out of inherited flags, free-form operands,
// and item flags.
func app() *Command {
	return &Command{Name: "app", Summary: "Do things", Flags: []Flag{
		{Name: "session", Usage: "Session `NAME`"},
		{Name: "verbose", Switch: true, Usage: "Talk more"},
	}, Commands: []*Command{
		{Name: "items", Summary: "Manage items", Commands: []*Command{
			{Name: "create", Summary: "Create items", Flags: []Flag{
				{Name: "item", Usage: "Item `TITLE`", Fields: []Flag{
					{Name: "note", Usage: "Item `NOTE`"},
					{Name: "tag", Repeat: Many, Usage: "Item `TAG`"},
				}},
				{Name: "project", Usage: "Project `REF`"},
			}},
			{Name: "get", Summary: "Read items", Operands: Operands{Usage: "REF...", Max: Unlimited, Value: Value{Kind: Dynamic, Completer: "items"}},
				Flags: []Flag{{Name: "query", Short: "q", Usage: "Filter `TEXT`"}}},
		}},
		{Name: "search", Summary: "Search", Operands: Operands{Usage: "PATTERN", Max: 1}, Flags: []Flag{{Name: "limit", Usage: "Page `N`"}}},
		{Name: "setup", Summary: "Install", Without: []string{"session", "verbose"}, Flags: []Flag{{Name: "scope", Usage: "`SCOPE`"}}},
		{Name: "adapter", Summary: "Adapt", Without: []string{"session"}, Flags: []Flag{{Name: "host", Usage: "`HOST`"}}, Commands: []*Command{
			{Name: "fresh", Summary: "Start fresh", Flags: []Flag{{Name: "session", Usage: "Own `NAME`"}}},
			{Name: "run", Summary: "Run", Operands: Operands{Usage: "-- COMMAND [ARGS...]", Max: Unlimited}},
		}},
	}}
}

func TestResolveMergesInheritedFlags(t *testing.T) {
	names := func(c *Command) string {
		var out []string
		c.EachFlag(func(f Flag, _ *Flag) {
			if f.Inherited {
				out = append(out, "^"+f.Name)
			} else {
				out = append(out, f.Name)
			}
		})
		return strings.Join(out, ",")
	}
	for _, tc := range []struct {
		path []string
		want string
	}{
		{nil, "session,verbose"},
		{[]string{"items"}, "^session,^verbose"},
		{[]string{"items", "create"}, "item,note,tag,project,^session,^verbose"},
		{[]string{"setup"}, "scope"},                             // opts out of both root flags
		{[]string{"adapter", "run"}, "^host,^verbose"},           // the group's opt-out applies below it
		{[]string{"adapter", "fresh"}, "session,^host,^verbose"}, // an own flag may reuse a withheld name
	} {
		c := app().Resolve(tc.path...)
		if c == nil || names(c) != tc.want {
			t.Errorf("%v: got %v, want %s", tc.path, c, tc.want)
		}
	}
	if app().Resolve("nope") != nil {
		t.Error("Resolve found a missing command")
	}
	if f, _ := app().Find("items", "create").Flag("session"); f.Name != "" {
		t.Error("Resolve changed the declared tree")
	}
}

func TestRouteFindsPathAndHelp(t *testing.T) {
	for _, tc := range []struct {
		argv    []string
		path    string
		help    bool
		unknown string
	}{
		{[]string{"items", "create", "--item", "a"}, "items create", false, ""},
		{[]string{"--session", "s", "items", "--verbose", "get", "x"}, "items get", false, ""},
		{[]string{"--project", "p", "items", "create"}, "items create", false, ""}, // a value flag declared below
		{[]string{"items", "get", "--help"}, "items get", true, ""},
		{[]string{"items", "get", "-h"}, "items get", true, ""},
		{[]string{"help", "items", "get"}, "items get", true, ""},
		{[]string{"help"}, "", true, ""},
		{[]string{"items", "get", "help"}, "items get", true, ""}, // bare alias in the command slot
		{[]string{"items", "help"}, "items", true, ""},
		{[]string{"items", "get", "x", "help"}, "items get", false, ""},  // after an operand it is an operand
		{[]string{"items", "get", "-q", "help"}, "items get", false, ""}, // a flag value
		{[]string{"items", "get", "-qx", "help"}, "items get", true, ""},
		{[]string{"items", "create", "--item", "--help"}, "items create", false, ""},
		{[]string{"search", "help"}, "search", false, ""}, // free-form operand
		{[]string{"search", "--", "--help"}, "search", false, ""},
		{[]string{"search", "x", "--help"}, "search", true, ""},
		{[]string{"adapter", "--host", "h", "run", "--", "items", "help"}, "adapter run", false, ""},
		{[]string{"items", "wat"}, "items", false, "wat"},
		{[]string{"wat"}, "", false, "wat"},
		{[]string{"help", "wat"}, "", true, "wat"},
	} {
		r := app().Route(tc.argv)
		if strings.Join(r.Path, " ") != tc.path || r.Help != tc.help || r.Unknown != tc.unknown {
			t.Errorf("%v: got path %q help %v unknown %q", tc.argv, strings.Join(r.Path, " "), r.Help, r.Unknown)
		}
	}
}

func TestRouteHelpTopicsMustExistEvenWithoutSubcommands(t *testing.T) {
	server := &Command{Name: "server", Flags: []Flag{{Name: "listen", Usage: "`ADDR`"}}}
	for _, tc := range []struct {
		argv    []string
		help    bool
		unknown string
	}{
		{[]string{"help"}, true, ""},
		{[]string{"--listen", "x", "help"}, true, ""},
		{[]string{"--listen", "help"}, false, ""}, // a flag value
		{[]string{"help", "extra"}, true, "extra"},
		{[]string{"serve"}, false, ""}, // an operand, rejected by Parse
	} {
		if r := server.Route(tc.argv); r.Help != tc.help || r.Unknown != tc.unknown {
			t.Errorf("%v: help %v unknown %q", tc.argv, r.Help, r.Unknown)
		}
	}
	if _, err := server.Parse([]string{"serve"}); err == nil || err.Error() != `unexpected argument "serve" for server` {
		t.Errorf("operand: %v", err)
	}
}

func TestParseCollectsFlagsItemsAndOperands(t *testing.T) {
	p, err := app().Parse([]string{"--session", "s", "items", "create", "--note", "n0", "--item", "a", "--tag", "x", "--tag", "y", "--item", "b", "--note", "n", "--project", "p"})
	if err != nil {
		t.Fatal(err)
	}
	want := &Parsed{Path: []string{"items", "create"}, Flags: map[string][]string{"session": {"s"}, "project": {"p"}}, Items: []Item{
		{Fields: map[string][]string{"note": {"n0"}}}, // fields before the first item flag
		{Flag: "item", Value: "a", Fields: map[string][]string{"tag": {"x", "y"}}},
		{Flag: "item", Value: "b", Fields: map[string][]string{"note": {"n"}}},
	}}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("got %+v\nwant %+v", p, want)
	}
	p, err = app().Parse([]string{"items", "get", "-q=t", "a", "--", "--b"})
	if err != nil || !reflect.DeepEqual(p.Operands, []string{"a", "--b"}) || p.Flags["query"][0] != "t" {
		t.Fatalf("operands: %+v %v", p, err)
	}
	if p, err := app().Parse([]string{"setup", "--help"}); err != nil || !p.Help {
		t.Fatalf("help short-circuits: %+v %v", p, err)
	}
}

func TestParseErrors(t *testing.T) {
	for _, tc := range []struct {
		argv []string
		kind ErrorKind
		msg  string
	}{
		{[]string{"items"}, MissingCommand, "a subcommand is required for app items; run 'app items --help'"},
		{[]string{"items", "wat"}, UnknownCommand, `unknown command "wat" for app items; run 'app items --help'`},
		{[]string{"help", "wat"}, UnknownCommand, `unknown command "wat" for app; run 'app --help'`},
		{[]string{"items", "wat", "--help"}, UnknownCommand, `unknown command "wat" for app items; run 'app items --help'`},
		{[]string{"items", "get", "--bogus"}, UnknownFlag, "unknown flag --bogus"},
		{[]string{"items", "get", "-z"}, UnknownFlag, "unknown flag -z"},
		{[]string{"setup", "--session", "s"}, NotApplicable, "--session does not apply to app setup"},
		{[]string{"adapter", "run", "--session", "s", "--", "x"}, NotApplicable, "--session does not apply to app adapter run"},
		{[]string{"items", "get", "--query", "a", "--query", "b"}, DuplicateFlag, "duplicate flag --query"},
		{[]string{"items", "create", "--item", "a", "--note", "x", "--note", "y"}, DuplicateFlag, "duplicate flag --note"},
		{[]string{"items", "get", "--query"}, MissingValue, "missing value for --query"},
		{[]string{"items", "get", "--query", "--verbose"}, MissingValue, "missing value for --query"},
		{[]string{"items", "get", "--verbose=yes"}, UnexpectedValue, "--verbose does not take a value"},
		{[]string{"setup", "x"}, UnexpectedOperand, `unexpected argument "x" for app setup`},
		{[]string{"search", "a", "b"}, UnexpectedOperand, `unexpected argument "b" for app search`},
	} {
		_, err := app().Parse(tc.argv)
		var pe *ParseError
		if !errors.As(err, &pe) || pe.Kind != tc.kind || err.Error() != tc.msg {
			t.Errorf("%v: got %v (%+v), want %q", tc.argv, err, pe, tc.msg)
		}
	}
}

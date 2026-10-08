package nwcli

import (
	"strings"
	"testing"
)

func help(path ...string) string {
	var b strings.Builder
	root := app()
	root.Find("items", "create").Description = "Create items atomically.\n\nRequirements:\n  --item per item."
	root.Find("items", "create").Example = "  app items create --item a"
	root.WriteHelp(&b, path...)
	return b.String()
}

func TestHelpForLeafCommand(t *testing.T) {
	want := `Create items atomically.

Requirements:
  --item per item.

Usage:
  app items create [flags]

Examples:
  app items create --item a

Flags:
  -h, --help          help for create
      --item TITLE    Item TITLE
      --note NOTE     Item NOTE
      --project REF   Project REF
      --tag TAG       Item TAG

Global Flags:
      --session NAME   Session NAME
      --verbose        Talk more
`
	if got := help("items", "create"); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestHelpForRootAndGroups(t *testing.T) {
	want := `Do things

Usage:
  app [flags]
  app [command]

Available Commands:
  adapter     Adapt
  help        Help about any command
  items       Manage items
  search      Search
  setup       Install

Flags:
  -h, --help           help for app
      --session NAME   Session NAME
      --verbose        Talk more

Use "app [command] --help" for more information about a command.
`
	if got := help(); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	group := help("adapter")
	for _, part := range []string{"Usage:\n  app adapter [flags]\n  app adapter [command]\n", "  fresh       Start fresh\n", "Flags:\n  -h, --help        help for adapter\n      --host HOST   HOST\n", "Global Flags:\n      --verbose   Talk more\n", `Use "app adapter [command] --help"`} {
		if !strings.Contains(group, part) {
			t.Errorf("group help lacks %q:\n%s", part, group)
		}
	}
	if strings.Contains(group, "--session") {
		t.Errorf("withheld flag listed:\n%s", group)
	}
	if run := help("adapter", "run"); !strings.Contains(run, "Usage:\n  app adapter run -- COMMAND [ARGS...] [flags]\n") || strings.Contains(run, "Available Commands") {
		t.Errorf("operand usage:\n%s", run)
	}
}

func TestHelpFlagColumnsAndPlaceholders(t *testing.T) {
	c := &Command{Name: "x", Flags: []Flag{
		{Name: "query", Short: "q", Usage: "Filter `TEXT`"},
		{Name: "all", Switch: true, Usage: "Everything"},
		{Name: "file", Usage: "Read a file"}, // no placeholder: a generic one is shown
	}}
	var b strings.Builder
	c.WriteHelp(&b)
	want := "Flags:\n      --all          Everything\n      --file value   Read a file\n  -h, --help         help for x\n  -q, --query TEXT   Filter TEXT\n"
	if !strings.Contains(b.String(), want) {
		t.Fatalf("got:\n%s\nwant section:\n%s", b.String(), want)
	}
}

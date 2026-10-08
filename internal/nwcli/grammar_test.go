package nwcli

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func sample() *Command {
	return &Command{Name: "tool", Commands: []*Command{
		{Name: "create", Flags: []Flag{
			{Name: "item", Fields: []Flag{
				{Name: "note"},
				{Name: "tag", Repeat: Many},
			}},
			{Name: "file", Value: Value{Kind: Path}},
			{Name: "force", Switch: true},
		}},
		{Name: "list", Flags: []Flag{
			{Name: "limit"},
			{Name: "id", Repeat: Many},
			{Name: "query", Short: "q"},
		}},
		{Name: "get", Operands: Operands{Usage: "REF...", Max: Unlimited, Value: Value{Kind: Dynamic, Completer: "records"}}},
	}}
}

func TestLookupFindsFieldsWithTheirItemFlag(t *testing.T) {
	create := sample().Find("create")
	if f, owner, ok := create.Lookup("tag"); !ok || owner == nil || owner.Name != "item" || f.Repeat != Many {
		t.Fatalf("field lookup: %+v %+v %v", f, owner, ok)
	}
	if f, owner, ok := create.Lookup("item"); !ok || owner != nil || len(f.Fields) != 2 {
		t.Fatalf("item flag lookup: %+v %+v %v", f, owner, ok)
	}
	var order []string
	create.EachFlag(func(f Flag, owner *Flag) { order = append(order, f.Name) })
	if strings.Join(order, ",") != "item,note,tag,file,force" || !create.HasItems() || sample().Find("list").HasItems() {
		t.Fatalf("flag order %v or HasItems wrong", order)
	}
}

func TestRepeatableFollowsItemsAndMany(t *testing.T) {
	root := sample()
	create, list := root.Find("create"), root.Find("list")
	for _, tc := range []struct {
		cmd        *Command
		flag       string
		repeatable bool
		starts     bool
	}{
		{create, "item", true, true},   // the item flag begins another item
		{create, "note", true, false},  // once per item, so again after a new boundary
		{create, "tag", true, false},   // accumulates
		{create, "file", false, false}, // an option of the whole invocation
		{create, "force", false, false},
		{create, "missing", false, false},
		{list, "limit", false, false}, // a top-level flag without fields
		{list, "id", true, false},
	} {
		if got := tc.cmd.Repeatable(tc.flag); got != tc.repeatable {
			t.Errorf("%s --%s repeatable = %v, want %v", tc.cmd.Name, tc.flag, got, tc.repeatable)
		}
		if got := tc.cmd.StartsItem(tc.flag); got != tc.starts {
			t.Errorf("%s --%s starts item = %v, want %v", tc.cmd.Name, tc.flag, got, tc.starts)
		}
	}
}

func TestFindAndWalk(t *testing.T) {
	root := sample()
	if root.Find("get") == nil || root.Find("get", "x") != nil || root.Find("nope") != nil || root.Find() != root {
		t.Fatal("Find did not follow the path")
	}
	var paths []string
	root.Walk(func(path []string, _ *Command) { paths = append(paths, strings.Join(path, " ")) })
	if strings.Join(paths, ",") != ",create,list,get" {
		t.Fatalf("walk order: %q", paths)
	}
}

func TestValidateRejectsInconsistentGrammars(t *testing.T) {
	if err := sample().Validate(); err != nil {
		t.Fatalf("valid sample: %v", err)
	}
	for name, mutate := range map[string]func(*Command){
		"duplicate flag":        func(c *Command) { c.Find("list").Flags = append(c.Find("list").Flags, Flag{Name: "limit"}) },
		"dashed flag":           func(c *Command) { c.Find("list").Flags[0].Name = "--limit" },
		"duplicate shorthand":   func(c *Command) { c.Find("list").Flags[0].Short = "q" },
		"long shorthand":        func(c *Command) { c.Find("list").Flags[0].Short = "qq" },
		"field duplicates flag": func(c *Command) { c.Find("create").Flags[0].Fields[0].Name = "file" },
		"nested fields": func(c *Command) {
			c.Find("create").Flags[0].Fields[0].Fields = []Flag{{Name: "deep"}}
		},
		"switch item flag":      func(c *Command) { c.Find("create").Flags[0].Switch = true },
		"accumulating item":     func(c *Command) { c.Find("create").Flags[0].Repeat = Many },
		"switch with kind":      func(c *Command) { c.Find("create").Flags[2].Value = Value{Kind: Path} },
		"accumulating switch":   func(c *Command) { c.Find("create").Flags[2].Repeat = Many },
		"choice without values": func(c *Command) { c.Find("list").Flags[0].Value = Value{Kind: Choice} },
		"values without choice": func(c *Command) { c.Find("list").Flags[0].Value = Value{Choices: []string{"a"}} },
		"dynamic without name":  func(c *Command) { c.Find("list").Flags[0].Value = Value{Kind: Dynamic} },
		"operands without usage": func(c *Command) {
			c.Find("get").Operands.Usage = ""
		},
		"duplicate subcommand": func(c *Command) { c.Commands = append(c.Commands, &Command{Name: "get"}) },
	} {
		root := sample()
		mutate(root)
		if err := root.Validate(); err == nil {
			t.Errorf("%s: Validate accepted it", name)
		}
	}
}

// nwcli must stay extractable into its own module: its packages import only
// the standard library and each other.
func TestImportsOnlyStandardLibrary(t *testing.T) {
	mod, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	own := strings.Fields(strings.SplitN(string(mod), "\n", 2)[0])[1] + "/internal/nwcli"
	files := 0
	err = filepath.WalkDir(".", func(name string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(name, ".go") {
			return err
		}
		files++
		src, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range f.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			if first := strings.Split(path, "/")[0]; strings.Contains(first, ".") && path != own && !strings.HasPrefix(path, own+"/") {
				t.Errorf("%s imports %s, outside the standard library and nwcli", name, path)
			}
		}
		return nil
	})
	if err != nil || files < 10 {
		t.Fatal(files, err)
	}
}

func TestExpandShortUsesDeclaredShorthands(t *testing.T) {
	grep := &Command{Name: "grep", Flags: []Flag{{Name: "context", Short: "C"}, {Name: "n", Short: "n", Switch: true}}}
	list := sample().Find("list")
	for _, tc := range []struct {
		cmd       *Command
		arg, want string
	}{
		{list, "-q", "--query"},
		{list, "-q=x", "--query=x"},
		{list, "-qx", "--query=x"},
		{list, "-q=", "--query="},
		{list, "-z", "-z"},       // undeclared: left for the parser to reject
		{list, "--q", "--q"},     // long forms are never rewritten
		{list, "query", "query"}, // operands are untouched
		{grep, "-C2", "--context=2"},
		{grep, "-n", "--n"},
		{grep, "-nx", "-nx"}, // a switch takes no attached value
		{grep, "-", "-"},
	} {
		if got := tc.cmd.ExpandShort(tc.arg); got != tc.want {
			t.Errorf("%s %q: got %q, want %q", tc.cmd.Name, tc.arg, got, tc.want)
		}
	}
}

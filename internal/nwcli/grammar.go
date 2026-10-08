// Package nwcli ("nerdwave-cli") declares command-line grammars: command trees,
// their flags and operands, and how flags repeat. It depends only on the Go
// standard library so that it can later become a module of its own.
package nwcli

import (
	"fmt"
	"strings"
)

// Repeat says how often a flag may be given.
type Repeat int

const (
	// Once allows a flag at most once per invocation, or once per item for a
	// field (see Flag.Fields).
	Once Repeat = iota
	// Many accumulates every given value.
	Many
)

// Kind says what a flag value or operand denotes; completion is derived from it.
type Kind int

const (
	Free    Kind = iota // free-form text with no candidates
	Path                // a file system path
	Dir                 // a directory
	Choice              // one of Value.Choices
	Dynamic             // candidates supplied at completion time by Value.Completer
)

// Value describes a flag value or the operands of a command.
type Value struct {
	Kind      Kind
	Choices   []string // Choice: accepted values in display order
	Completer string   // Dynamic: name of the completer supplying candidates
}

// Flag describes one --name option of a command.
type Flag struct {
	Name  string
	Short string // optional one-letter shorthand, used as -Short
	// Usage is the help text; a `quoted` word names the value in help.
	Usage  string
	Switch bool  // takes no value
	Value  Value // the value of a non-switch flag
	Repeat Repeat
	// Fields declares the flags that describe the item this flag begins, which
	// makes the flag an item flag: each occurrence begins another item, and
	// each field may be given once per item (any number of times when Many).
	// Fields given before the first occurrence describe an implicit first item,
	// such as the operands. Fields cannot have fields of their own.
	Fields []Flag
	// Inherited is set by Resolve on flags declared by an ancestor.
	Inherited bool
}

// Unlimited is an Operands.Max meaning any number of operands.
const Unlimited = -1

// Operands describes the positional arguments of a command.
type Operands struct {
	Usage string // e.g. "REF..." or "[REF]"; empty when Max is 0
	Max   int    // maximum count, or Unlimited
	Value Value
	// Conflicts names flags that replace the operands, such as --all; once
	// one is given, no operands are offered.
	Conflicts []string
}

// Command is a node of a command tree. A command with subcommands is a group.
// Flags declared on a command apply to its descendants too, except where a
// command lists them in Without. Flag names are unique within a command,
// including the fields of its item flags.
type Command struct {
	Name        string
	Summary     string // one line, shown in command lists
	Description string // help text; Summary is used when empty
	Example     string // indented example lines
	Operands    Operands
	Flags       []Flag
	Without     []string // inherited flags that do not apply here or below
	Commands    []*Command

	withheld map[string]bool // set by Resolve: inherited flags removed by Without
}

// Find returns the descendant reached by following path, or nil.
func (c *Command) Find(path ...string) *Command {
	for _, name := range path {
		var next *Command
		for _, child := range c.Commands {
			if child.Name == name {
				next = child
				break
			}
		}
		if next == nil {
			return nil
		}
		c = next
	}
	return c
}

// Lookup finds a flag by long name, at the top level or as a field. owner is
// the item flag a field belongs to, or nil for a top-level flag.
func (c *Command) Lookup(name string) (f Flag, owner *Flag, ok bool) {
	for i, top := range c.Flags {
		if top.Name == name {
			return top, nil, true
		}
		for _, field := range top.Fields {
			if field.Name == name {
				return field, &c.Flags[i], true
			}
		}
	}
	return Flag{}, nil, false
}

// Flag returns the flag with the given long name, at the top level or as a field.
func (c *Command) Flag(name string) (Flag, bool) {
	f, _, ok := c.Lookup(name)
	return f, ok
}

// EachFlag calls fn for every flag, item flags before their fields.
func (c *Command) EachFlag(fn func(f Flag, owner *Flag)) {
	for i, top := range c.Flags {
		fn(top, nil)
		for _, field := range top.Fields {
			fn(field, &c.Flags[i])
		}
	}
}

// HasItems reports whether the command declares an item flag.
func (c *Command) HasItems() bool {
	for _, f := range c.Flags {
		if len(f.Fields) > 0 {
			return true
		}
	}
	return false
}

// Repeatable reports whether a flag may be given more than once in a single
// invocation: it accumulates values, it begins items, or it is a field, which
// may be given again for each item.
func (c *Command) Repeatable(name string) bool {
	f, owner, ok := c.Lookup(name)
	return ok && (f.Repeat == Many || len(f.Fields) > 0 || owner != nil)
}

// StartsItem reports whether giving the flag begins a new item.
func (c *Command) StartsItem(name string) bool {
	f, owner, ok := c.Lookup(name)
	return ok && owner == nil && len(f.Fields) > 0
}

// Walk calls fn for c and every descendant with its path from c.
func (c *Command) Walk(fn func(path []string, cmd *Command)) {
	var walk func([]string, *Command)
	walk = func(path []string, cmd *Command) {
		fn(path, cmd)
		for _, child := range cmd.Commands {
			walk(append(append([]string{}, path...), child.Name), child)
		}
	}
	walk(nil, c)
}

// Validate reports the first inconsistency in the tree: duplicate names, an
// impossible item hierarchy, or a value description that cannot work.
func (c *Command) Validate() error {
	var err error
	c.Walk(func(path []string, cmd *Command) {
		if err != nil {
			return
		}
		where := strings.Join(append([]string{c.Name}, path...), " ")
		fail := func(format string, args ...any) { err = fmt.Errorf(where+": "+format, args...) }
		names, shorts, children := map[string]bool{}, map[string]bool{}, map[string]bool{}
		for _, child := range cmd.Commands {
			if child.Name == "" || children[child.Name] {
				fail("empty or duplicate subcommand %q", child.Name)
				return
			}
			children[child.Name] = true
		}
		cmd.EachFlag(func(f Flag, owner *Flag) {
			if err != nil {
				return
			}
			switch {
			case f.Name == "" || strings.HasPrefix(f.Name, "-") || names[f.Name]:
				fail("empty, dashed or duplicate flag %q", f.Name)
			case f.Short != "" && (len([]rune(f.Short)) != 1 || shorts[f.Short]):
				fail("--%s: shorthand %q must be one unused character", f.Name, f.Short)
			case f.Switch && (f.Value.Kind != Free || f.Repeat == Many || len(f.Fields) > 0):
				fail("--%s: a switch has no value kind, cannot accumulate and cannot begin items", f.Name)
			case owner != nil && len(f.Fields) > 0:
				fail("--%s: fields cannot have fields", f.Name)
			case len(f.Fields) > 0 && f.Repeat == Many:
				fail("--%s: an item flag repeats by beginning items, not by accumulating", f.Name)
			default:
				err = checkValue(where+": --"+f.Name, f.Value)
			}
			names[f.Name], shorts[f.Short] = true, f.Short != "" || shorts[f.Short]
		})
		if err != nil {
			return
		}
		if o := cmd.Operands; (o.Max == 0) != (o.Usage == "") || o.Max < Unlimited {
			fail("operands need a usage exactly when Max is not 0")
			return
		}
		err = checkValue(where+": operands", cmd.Operands.Value)
	})
	return err
}

func checkValue(where string, v Value) error {
	switch {
	case v.Kind == Choice && len(v.Choices) == 0:
		return fmt.Errorf("%s: choice without choices", where)
	case v.Kind != Choice && len(v.Choices) > 0:
		return fmt.Errorf("%s: choices without Choice kind", where)
	case (v.Kind == Dynamic) != (v.Completer != ""):
		return fmt.Errorf("%s: a completer is required exactly for Dynamic values", where)
	}
	return nil
}

// ExpandShort rewrites a declared shorthand to its long form: -x becomes --name,
// and for value flags -x=V and -xV become --name=V. Any other argument,
// including an undeclared shorthand, is returned unchanged. Call it only where
// a flag may appear, so that flag values and operands stay literal.
func (c *Command) ExpandShort(arg string) string {
	if len(arg) < 2 || arg[0] != '-' || arg[1] == '-' {
		return arg
	}
	short, rest := string([]rune(arg[1:])[0]), arg[1+len(string([]rune(arg[1:])[0])):]
	out := arg
	c.EachFlag(func(f Flag, _ *Flag) {
		switch {
		case f.Short != short || out != arg:
		case rest == "":
			out = "--" + f.Name
		case !f.Switch:
			out = "--" + f.Name + "=" + strings.TrimPrefix(rest, "=")
		}
	})
	return out
}

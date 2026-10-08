package nwcli

import (
	"fmt"
	"strings"
)

// Resolve returns the command at path as it applies there: its own flags,
// then the flags its ancestors declare (marked Inherited, closest first),
// except those listed in Without along the path. An own flag shadows an
// inherited one of the same name. Resolve returns nil for an unknown path and
// never changes the tree.
func (c *Command) Resolve(path ...string) *Command {
	node, ancestors := c, []*Command{}
	for _, name := range path {
		ancestors = append(ancestors, node)
		if node = node.Find(name); node == nil {
			return nil
		}
	}
	withheld := map[string]bool{}
	for _, cmd := range append(ancestors, node) {
		for _, name := range cmd.Without {
			withheld[name] = true
		}
	}
	out := *node
	out.Flags = append([]Flag{}, node.Flags...)
	seen := map[string]bool{}
	node.EachFlag(func(f Flag, _ *Flag) { seen[f.Name] = true })
	for i := len(ancestors) - 1; i >= 0; i-- {
		for _, f := range ancestors[i].Flags {
			if !withheld[f.Name] && !seen[f.Name] {
				f.Inherited, seen[f.Name] = true, true
				out.Flags = append(out.Flags, f)
			}
		}
	}
	out.withheld = withheld
	return &out
}

// Route locates the command an argument list invokes, wherever flags appear
// around the command names, and whether it asks for help: --help or -h, a
// leading help command, or a bare help where a subcommand name could appear
// (unless the command takes free-form operands, where help is an operand).
// Flag values and arguments after -- are never interpreted.
type Route struct {
	Path    []string // command names from the root
	Help    bool
	Unknown string // a word naming no subcommand where one was required
	words   map[int]bool
}

func (c *Command) Route(argv []string) Route {
	r := Route{words: map[int]bool{}}
	node, current, inPath := c, c.Resolve(), true
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			break
		}
		if a == "--help" || a == "-h" {
			r.Help = true
			continue
		}
		if len(a) > 1 && a[0] == '-' {
			a = current.ExpandShort(a)
			name, _, hasValue := strings.Cut(strings.TrimPrefix(a, "--"), "=")
			if hasValue || !strings.HasPrefix(a, "--") {
				continue
			}
			if f, ok := current.Flag(name); ok && !f.Switch || !ok && node.declaresValue(name) {
				i++
			}
			continue
		}
		if !inPath {
			continue
		}
		switch child := node.Find(a); {
		case child != nil:
			node, r.Path, r.words[i] = child, append(r.Path, a), true
			current = c.Resolve(r.Path...)
		case a == "help" && node == c && len(r.Path) == 0 && !r.Help:
			r.Help, r.words[i] = true, true // the help command: the names after it select the topic
		case a == "help" && node != c && !(node.Operands.Max != 0 && node.Operands.Value.Kind == Free):
			r.Help, r.words[i], inPath = true, true, false
		default:
			if len(node.Commands) > 0 && r.Unknown == "" {
				r.Unknown = a
			}
			inPath = false
		}
	}
	return r
}

// declaresValue reports whether the command or a descendant declares a flag
// with this name that takes a value.
func (c *Command) declaresValue(name string) bool {
	found := false
	c.Walk(func(_ []string, cmd *Command) {
		cmd.EachFlag(func(f Flag, _ *Flag) { found = found || f.Name == name && !f.Switch })
	})
	return found
}

// Parsed is the result of Parse.
type Parsed struct {
	Path     []string
	Flags    map[string][]string // flags of the whole invocation
	Items    []Item              // in order; a first item without Flag holds fields given before any item flag
	Operands []string
	Help     bool // help was requested; nothing else was validated
}

// Item is one occurrence of an item flag with the fields describing it.
type Item struct {
	Flag, Value string
	Fields      map[string][]string
}

// ErrorKind classifies a ParseError.
type ErrorKind int

const (
	UnknownCommand ErrorKind = iota
	MissingCommand
	UnknownFlag
	NotApplicable // an inherited flag withheld by Without
	DuplicateFlag
	MissingValue
	UnexpectedValue
	UnexpectedOperand
)

// ParseError reports an invalid argument list. Command is the invoked command
// path including the program name; Arg is the offending word or flag.
type ParseError struct {
	Kind         ErrorKind
	Command, Arg string
}

func (e *ParseError) Error() string {
	switch e.Kind {
	case UnknownCommand:
		return fmt.Sprintf("unknown command %q for %s; run '%s --help'", e.Arg, e.Command, e.Command)
	case MissingCommand:
		return fmt.Sprintf("a subcommand is required for %s; run '%s --help'", e.Command, e.Command)
	case NotApplicable:
		return e.Arg + " does not apply to " + e.Command
	case DuplicateFlag:
		return "duplicate flag " + e.Arg
	case MissingValue:
		return "missing value for " + e.Arg
	case UnexpectedValue:
		return e.Arg + " does not take a value"
	case UnexpectedOperand:
		return fmt.Sprintf("unexpected argument %q for %s", e.Arg, e.Command)
	}
	return "unknown flag " + e.Arg
}

// Parse routes argv and parses it against the invoked command. A value that
// starts with -- is never taken as a flag value.
func (c *Command) Parse(argv []string) (*Parsed, error) {
	r := c.Route(argv)
	p := &Parsed{Path: r.Path, Flags: map[string][]string{}, Help: r.Help}
	where := strings.Join(append([]string{c.Name}, r.Path...), " ")
	fail := func(kind ErrorKind, arg string) (*Parsed, error) { return p, &ParseError{kind, where, arg} }
	if r.Help {
		return p, nil
	}
	if r.Unknown != "" {
		return fail(UnknownCommand, r.Unknown)
	}
	node := c.Resolve(r.Path...)
	if len(node.Commands) > 0 {
		return fail(MissingCommand, "")
	}
	current := -1
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case r.words[i]:
			continue
		case a == "--":
			p.Operands = append(p.Operands, argv[i+1:]...)
			i = len(argv)
			continue
		case len(a) < 2 || a[0] != '-':
			p.Operands = append(p.Operands, a)
			continue
		}
		if a = node.ExpandShort(a); !strings.HasPrefix(a, "--") {
			return fail(UnknownFlag, a)
		}
		name, value, hasValue := strings.Cut(a[2:], "=")
		f, owner, ok := node.Lookup(name)
		switch {
		case !ok && node.withheld[name]:
			return fail(NotApplicable, "--"+name)
		case !ok:
			return fail(UnknownFlag, "--"+name)
		case f.Switch && hasValue:
			return fail(UnexpectedValue, "--"+name)
		case f.Switch:
			value = "true"
		case !hasValue:
			if i++; i >= len(argv) || strings.HasPrefix(argv[i], "--") {
				return fail(MissingValue, "--"+name)
			}
			value = argv[i]
		}
		values := p.Flags
		switch {
		case len(f.Fields) > 0:
			p.Items = append(p.Items, Item{Flag: name, Value: value, Fields: map[string][]string{}})
			current = len(p.Items) - 1
			continue
		case owner != nil:
			if current < 0 {
				p.Items = append(p.Items, Item{Fields: map[string][]string{}})
				current = 0
			}
			values = p.Items[current].Fields
		}
		if len(values[name]) > 0 && f.Repeat != Many {
			return fail(DuplicateFlag, "--"+name)
		}
		values[name] = append(values[name], value)
	}
	if max := node.Operands.Max; max != Unlimited && len(p.Operands) > max {
		return fail(UnexpectedOperand, p.Operands[max])
	}
	return p, nil
}

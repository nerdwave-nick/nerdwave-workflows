package nwcli

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Candidate is one completion; the description is optional.
type Candidate struct{ Value, Description string }

// Directive tells the shell how to treat the candidates. Its values are part
// of the completion protocol (see WriteCompletion).
type Directive int

const (
	Files     Directive = 0  // also offer file names
	NoFiles   Directive = 4  // offer only the candidates
	DirsOnly  Directive = 16 // offer directories only
	KeepOrder Directive = 32 // keep the candidates' order instead of sorting
)

// Context describes the position a dynamic completer completes: the command,
// the invocation-wide flags given so far, and the word being completed.
type Context struct {
	Path   []string
	Flags  map[string][]string
	Prefix string
}

// Completer supplies candidates for a Dynamic value, named by Value.Completer.
type Completer func(Context) ([]Candidate, Directive)

// Complete returns the candidates for the last of args, the word being
// completed, after the words before it. It follows the grammar exactly: flags
// before and between command names, shorthands, values of value flags, items
// and their fields, and --. A flag that may not be given again is not offered.
func (c *Command) Complete(args []string, completers map[string]Completer) ([]Candidate, Directive) {
	if len(args) == 0 {
		args = []string{""}
	}
	words, current := args[:len(args)-1], args[len(args)-1]
	r := c.Route(words)
	node := c.Resolve(r.Path...)
	if r.Unknown != "" || node == nil {
		return nil, NoFiles
	}
	s := scan(node, words, r.words)
	ctx := Context{Path: r.Path, Flags: s.flags, Prefix: current}
	switch {
	case s.pending != nil:
		return completeValue(s.pending.Value, ctx, completers)
	case !s.afterDash && strings.HasPrefix(current, "--") && strings.Contains(current, "="):
		name, prefix, _ := strings.Cut(current[2:], "=")
		if f, ok := node.Flag(name); ok && !f.Switch {
			ctx.Prefix = prefix
			return completeValue(f.Value, ctx, completers)
		}
		return nil, NoFiles
	case !s.afterDash && strings.HasPrefix(current, "-"):
		return flagNames(node, s, current), NoFiles
	case len(node.Commands) > 0:
		return subcommands(node, len(r.Path) == 0, current), NoFiles
	case node.Operands.Max == 0 && !s.afterDash:
		return bareFlags(node, s), NoFiles | KeepOrder
	case node.Operands.Max == 0 || node.Operands.Max != Unlimited && s.operands >= node.Operands.Max:
		return nil, NoFiles
	}
	for _, name := range node.Operands.Conflicts {
		if _, given := s.flags[name]; given {
			return nil, NoFiles
		}
	}
	return completeValue(node.Operands.Value, ctx, completers)
}

// scanned is what the words before the completed one establish.
type scanned struct {
	flags     map[string][]string // invocation-wide flags and their values
	item      map[string]int      // fields given for the current item
	operands  int
	pending   *Flag // a value flag still waiting for its value
	afterDash bool
}

func scan(node *Command, words []string, skip map[int]bool) scanned {
	s := scanned{flags: map[string][]string{}}
	pendingTop := false
	for i, a := range words {
		switch {
		case skip[i]:
			continue
		case s.pending != nil:
			if pendingTop {
				s.flags[s.pending.Name] = append(s.flags[s.pending.Name], a)
			}
			s.pending = nil
			continue
		case s.afterDash || len(a) < 2 || a[0] != '-':
			s.operands++
			continue
		case a == "--":
			s.afterDash = true
			continue
		}
		a = node.ExpandShort(a)
		name, value, hasValue := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		f, owner, ok := node.Lookup(name)
		if !ok || !strings.HasPrefix(a, "--") {
			continue
		}
		switch {
		case len(f.Fields) > 0:
			s.item = map[string]int{}
		case owner != nil:
			if s.item == nil {
				s.item = map[string]int{}
			}
			s.item[name]++
		}
		top := owner == nil && len(f.Fields) == 0
		if top && (f.Switch || hasValue) {
			s.flags[name] = append(s.flags[name], value)
		} else if _, given := s.flags[name]; top && !given {
			s.flags[name] = nil // given; its value follows in the next word
		}
		if !f.Switch && !hasValue {
			s.pending, pendingTop = &f, top
		}
	}
	return s
}

// usable reports whether a flag may still be given: it accumulates or begins
// items, or it has not been given yet (for a field: in the current item).
func usable(f Flag, owner *Flag, s scanned) bool {
	if f.Repeat == Many || len(f.Fields) > 0 {
		return true
	}
	if owner != nil {
		return s.item[f.Name] == 0
	}
	_, given := s.flags[f.Name]
	return !given
}

// flagNames completes a word starting with a dash: every usable flag sorted by
// name, each shorthand after its long form, described by its usage.
func flagNames(node *Command, s scanned, prefix string) []Candidate {
	flags := []Flag{{Name: "help", Short: "h", Switch: true, Usage: "help for " + node.Name}}
	node.EachFlag(func(f Flag, owner *Flag) {
		if usable(f, owner, s) {
			flags = append(flags, f)
		}
	})
	sort.SliceStable(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
	var out []Candidate
	for _, f := range flags {
		for _, name := range []string{"--" + f.Name, "-" + f.Short} {
			if name != "-" && strings.HasPrefix(name, prefix) {
				out = append(out, Candidate{name, f.Usage})
			}
		}
	}
	return out
}

// bareFlags lists the usable flags of a command without operands, its own
// before inherited ones, each group sorted by name.
func bareFlags(node *Command, s scanned) []Candidate {
	var own, inherited []Candidate
	node.EachFlag(func(f Flag, owner *Flag) {
		if !usable(f, owner, s) {
			return
		}
		if f.Inherited {
			inherited = append(inherited, Candidate{"--" + f.Name, f.Usage})
		} else {
			own = append(own, Candidate{"--" + f.Name, f.Usage})
		}
	})
	for _, group := range [][]Candidate{own, inherited} {
		sort.Slice(group, func(i, j int) bool { return group[i].Value < group[j].Value })
	}
	return append(own, inherited...)
}

func subcommands(node *Command, root bool, prefix string) []Candidate {
	var out []Candidate
	for _, child := range node.Commands {
		out = append(out, Candidate{child.Name, child.Summary})
	}
	if root {
		out = append(out, Candidate{"help", "Help about any command"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Value < out[j].Value })
	filtered := out[:0]
	for _, cand := range out {
		if strings.HasPrefix(cand.Value, prefix) {
			filtered = append(filtered, cand)
		}
	}
	return filtered
}

func completeValue(v Value, ctx Context, completers map[string]Completer) ([]Candidate, Directive) {
	switch v.Kind {
	case Path:
		return nil, Files
	case Dir:
		return nil, DirsOnly
	case Choice:
		var out []Candidate
		for _, choice := range v.Choices {
			if strings.HasPrefix(choice, ctx.Prefix) {
				out = append(out, Candidate{Value: choice})
			}
		}
		return out, NoFiles
	case Dynamic:
		if complete := completers[v.Completer]; complete != nil {
			return complete(ctx)
		}
	}
	return nil, NoFiles
}

// WriteCompletion writes candidates in the completion protocol: one line per
// candidate, "value" or "value<TAB>description", then ":" and the directive.
func WriteCompletion(w io.Writer, cands []Candidate, d Directive, descriptions bool) {
	var b strings.Builder
	for _, c := range cands {
		b.WriteString(c.Value)
		if descriptions && c.Description != "" {
			b.WriteString("\t" + c.Description)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, ":%d\n", d)
	io.WriteString(w, b.String())
}

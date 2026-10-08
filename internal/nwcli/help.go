package nwcli

import (
	"io"
	"sort"
	"strings"
)

// minNamePadding is the narrowest column for command names in command lists.
const minNamePadding = 11

// WriteHelp writes the help page of the command at path: its description,
// usage lines, examples, subcommands, own flags and inherited ("global")
// flags. Every command also accepts -h/--help, which the page lists; the root
// lists a help command.
func (c *Command) WriteHelp(w io.Writer, path ...string) {
	node := c.Resolve(path...)
	if node == nil {
		return
	}
	full := strings.Join(append([]string{c.Name}, path...), " ")
	var b strings.Builder
	description := node.Description
	if description == "" {
		description = node.Summary
	}
	if description = strings.TrimRight(description, " \t\n"); description != "" {
		b.WriteString(description + "\n\n")
	}
	use := full
	if node.Operands.Usage != "" {
		use += " " + node.Operands.Usage
	}
	b.WriteString("Usage:\n  " + use + " [flags]")
	if len(node.Commands) > 0 {
		b.WriteString("\n  " + full + " [command]")
	}
	if node.Example != "" {
		b.WriteString("\n\nExamples:\n" + node.Example)
	}
	if len(node.Commands) > 0 {
		b.WriteString("\n\nAvailable Commands:")
		names := map[string]string{}
		for _, child := range node.Commands {
			names[child.Name] = child.Summary
		}
		if len(path) == 0 {
			names["help"] = "Help about any command"
		}
		pad := minNamePadding
		for name := range names {
			pad = max(pad, len(name))
		}
		for _, name := range sortedKeys(names) {
			b.WriteString("\n  " + name + strings.Repeat(" ", pad-len(name)) + " " + names[name])
		}
	}
	name := c.Name
	if len(path) > 0 {
		name = path[len(path)-1]
	}
	own := []Flag{{Name: "help", Short: "h", Switch: true, Usage: "help for " + name}}
	var inherited []Flag
	node.EachFlag(func(f Flag, _ *Flag) {
		if f.Inherited {
			inherited = append(inherited, f)
		} else {
			own = append(own, f)
		}
	})
	b.WriteString("\n\nFlags:\n" + flagUsages(own))
	if len(inherited) > 0 {
		b.WriteString("\n\nGlobal Flags:\n" + flagUsages(inherited))
	}
	if len(node.Commands) > 0 {
		b.WriteString("\n\nUse \"" + full + " [command] --help\" for more information about a command.")
	}
	b.WriteString("\n")
	io.WriteString(w, b.String())
}

// flagUsages lists flags sorted by name in two aligned columns. A `quoted`
// word in the usage names the value; value flags without one show "value".
func flagUsages(flags []Flag) string {
	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
	names, usages, width := make([]string, len(flags)), make([]string, len(flags)), 0
	for i, f := range flags {
		names[i] = "      --" + f.Name
		if f.Short != "" {
			names[i] = "  -" + f.Short + ", --" + f.Name
		}
		varname, usage := unquoteUsage(f)
		if varname != "" {
			names[i] += " " + varname
		}
		usages[i], width = usage, max(width, len(names[i]))
	}
	lines := make([]string, len(flags))
	for i := range flags {
		lines[i] = strings.TrimRight(names[i]+strings.Repeat(" ", width-len(names[i])+3)+usages[i], " ")
	}
	return strings.Join(lines, "\n")
}

func unquoteUsage(f Flag) (varname, usage string) {
	if start := strings.Index(f.Usage, "`"); start >= 0 {
		if end := strings.Index(f.Usage[start+1:], "`"); end >= 0 {
			varname = f.Usage[start+1 : start+1+end]
			return varname, f.Usage[:start] + varname + f.Usage[start+2+end:]
		}
	}
	if f.Switch {
		return "", f.Usage
	}
	return "value", f.Usage
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

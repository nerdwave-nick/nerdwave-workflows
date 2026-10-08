package cli

import (
	"encoding/base64"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nerdwave-nick/nerdwave-workflows/internal/clientendpoint"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/nwcli"
	"github.com/spf13/cobra"
)

// localValues complete flag values from local state only; they never contact the service.
var localValues = map[string]func() []string{
	"sessions":  savedSessions,
	"endpoints": savedEndpoints,
	"durations": func() []string { return []string{"15m", "30m", "45m", "1h"} },
}

// runCompletion serves cobra's completion commands from a tree generated from
// the grammar.
func runCompletion(argv []string, out, errOut io.Writer) int {
	root := newCompletionTree()
	root.SetOut(out)
	root.SetErr(errOut)
	root.InitDefaultCompletionCmd()
	root.InitDefaultHelpCmd()
	root.SetArgs(argv)
	if _, err := root.ExecuteC(); err != nil {
		return argumentError(argv, out, errOut, err)
	}
	return 0
}

// newCompletionTree mirrors the grammar as cobra commands for completion.
// Every command carries its resolved flags, so inherited flags it withholds
// never appear, and each value completes according to its kind.
func newCompletionTree() *cobra.Command {
	var build func(path []string, declared *nwcli.Command) *cobra.Command
	build = func(path []string, declared *nwcli.Command) *cobra.Command {
		cmd := command(path...)
		c := &cobra.Command{Use: declared.Name, Short: declared.Summary, Run: func(*cobra.Command, []string) {}, SilenceErrors: true, SilenceUsage: true}
		cmd.EachFlag(func(f nwcli.Flag, _ *nwcli.Flag) {
			if f.Switch {
				c.Flags().BoolP(f.Name, f.Short, false, f.Usage)
				return
			}
			c.Flags().StringArrayP(f.Name, f.Short, nil, f.Usage)
			if f.Value.Kind == nwcli.Dir {
				c.MarkFlagDirname(f.Name)
			} else if f.Value.Kind != nwcli.Path {
				if err := c.RegisterFlagCompletionFunc(f.Name, completeValue(f.Value)); err != nil {
					panic(err)
				}
			}
		})
		if len(declared.Commands) == 0 {
			c.ValidArgsFunction = completeOperands(path, cmd.Operands)
		}
		for _, child := range declared.Commands {
			if child.Name != "completion" {
				c.AddCommand(build(append(append([]string{}, path...), child.Name), child))
			}
		}
		return c
	}
	return build(nil, grammar)
}

func completeValue(v nwcli.Value) cobra.CompletionFunc {
	switch {
	case v.Kind == nwcli.Choice:
		return cobra.FixedCompletions(v.Choices, cobra.ShellCompDirectiveNoFileComp)
	case v.Kind == nwcli.Dynamic && localValues[v.Completer] != nil:
		return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return localValues[v.Completer](), cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder
		}
	case v.Kind == nwcli.Dynamic:
		return completeRecords(v.Completer)
	}
	return cobra.NoFileCompletions
}

// completeOperands offers records for reference operands, nothing for
// free-form ones, and the command's flags when it takes no operands.
func completeOperands(path []string, o nwcli.Operands) cobra.CompletionFunc {
	switch {
	case o.Max == 0:
		return completeFlagNames
	case o.Value.Kind != nwcli.Dynamic:
		return cobra.NoFileCompletions
	}
	records := completeRecords(o.Value.Completer)
	return func(c *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		// claims --all replaces explicit targets.
		if all := c.Flags().Lookup("all"); o.Max != nwcli.Unlimited && len(args) >= o.Max || path[0] == "claims" && all != nil && all.Changed {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return records(c, args, prefix)
	}
}

// completeFlagNames lists the flags a command still accepts, its own before
// inherited ones, formatted like cobra's own --flag completion. A flag already
// given is left out unless the grammar lets it repeat.
func completeFlagNames(c *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	cmd := command(strings.Fields(c.CommandPath())[1:]...)
	var own, inherited []nwcli.Flag
	cmd.EachFlag(func(f nwcli.Flag, _ *nwcli.Flag) {
		if used := c.Flags().Lookup(f.Name); used != nil && used.Changed && !cmd.Repeatable(f.Name) {
			return
		}
		if f.Inherited {
			inherited = append(inherited, f)
		} else {
			own = append(own, f)
		}
	})
	out := []string{}
	for _, group := range [][]nwcli.Flag{own, inherited} {
		sort.Slice(group, func(i, j int) bool { return group[i].Name < group[j].Name })
		for _, f := range group {
			out = append(out, cobra.CompletionWithDesc("--"+f.Name, f.Usage))
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder
}

// savedSessions lists logical session names from the client state directory,
// described by their remembered endpoint.
func savedSessions() []string {
	dir, err := StateDir()
	if err != nil {
		return nil
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "sessions", "*.json"))
	out := []string{}
	for _, path := range paths {
		name, err := base64.RawURLEncoding.DecodeString(strings.TrimSuffix(filepath.Base(path), ".json"))
		if err != nil || len(name) == 0 || strings.ContainsAny(string(name), "\t\n\r") {
			continue
		}
		value := string(name)
		if m, err := readMapping(path); err == nil && m.Endpoint != "" {
			value += "\t" + m.Endpoint
		}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// savedEndpoints lists the default endpoint and every endpoint a saved session remembers.
func savedEndpoints() []string {
	seen := map[string]bool{clientendpoint.Default: true}
	for _, session := range savedSessions() {
		if _, endpoint, ok := strings.Cut(session, "\t"); ok {
			seen[endpoint] = true
		}
	}
	out := []string{}
	for endpoint := range seen {
		out = append(out, endpoint)
	}
	sort.Strings(out)
	return out
}

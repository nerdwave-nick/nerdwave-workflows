package cli

import (
	"encoding/base64"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nerdwave-nick/nerdwave-workflows/internal/clientendpoint"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// pathFlags are the only values that are file system paths; they keep the
// shell's file (or, via MarkFlagDirname, directory) completion.
var pathFlags = map[string]bool{"file": true, "content-file": true, "cli": true, "path": true}

// localValues complete flag values from local state only; they never contact the service.
var localValues = map[string]func() []string{
	"session":  savedSessions,
	"endpoint": savedEndpoints,
	"for":      func() []string { return []string{"15m", "30m", "45m", "1h"} },
}

// addLocalCompletions replaces cobra's file-name fallback: commands without
// operands offer their flags, and value flags offer known local values or nothing.
func addLocalCompletions(c *cobra.Command) {
	for _, child := range c.Commands() {
		addLocalCompletions(child)
	}
	if !c.HasSubCommands() && c.ValidArgsFunction == nil {
		// A Use with an operand (grep PATTERN, run -- COMMAND) takes free-form input.
		if strings.Contains(c.Use, " ") {
			c.ValidArgsFunction = cobra.NoFileCompletions
		} else {
			c.ValidArgsFunction = completeFlagNames
		}
	}
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.NoOptDefVal != "" || pathFlags[f.Name] {
			return
		}
		if _, ok := c.GetFlagCompletionFunc(f.Name); ok {
			return
		}
		complete := cobra.NoFileCompletions
		if values := localValues[f.Name]; values != nil {
			complete = func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
				return values(), cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder
			}
		}
		if err := c.RegisterFlagCompletionFunc(f.Name, complete); err != nil {
			panic(err)
		}
	})
}

// completeFlagNames lists the flags a command still accepts, local ones first,
// formatted like cobra's own --flag completion. Flags already given are left out
// unless their usage documents repetition; lit rejects any other duplicate flag.
func completeFlagNames(c *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	out := []string{}
	add := func(f *pflag.Flag) {
		if f.Hidden || f.Deprecated != "" || f.Name == "help" || f.Changed && !strings.Contains(f.Usage, "repeat") {
			return
		}
		out = append(out, cobra.CompletionWithDesc("--"+f.Name, f.Usage))
	}
	c.NonInheritedFlags().VisitAll(add)
	c.InheritedFlags().VisitAll(add)
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

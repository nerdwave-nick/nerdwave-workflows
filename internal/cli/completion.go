package cli

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/nerdwave-nick/nerdwave-workflows/internal/clientendpoint"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const completionTimeout = time.Second

// Completion uses its own read-only path, never runTracker: no mapping lock,
// registration, preference cache, pending request or reconnect is needed.
func completeRecords(kind string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		items, err := fetchCompletions(cmd, kind, prefix)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		out := []string{}
		for _, item := range items {
			if item.Value == "" || strings.ContainsFunc(item.Value, unicode.IsControl) {
				continue
			}
			// Values are raw arguments. The generated shell adapter owns quoting.
			description := strings.Join([]string{item.Title, item.ProjectTitle, item.State, item.ID}, " ")
			description = strings.Map(func(r rune) rune {
				if unicode.IsControl(r) {
					return ' '
				}
				return r
			}, description)
			out = append(out, item.Value+"\t"+strings.Join(strings.Fields(description), " "))
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

func fetchCompletions(cmd *cobra.Command, kind, prefix string) ([]protocol.Completion, error) {
	one := func(name string) (string, bool) {
		values := flagValues(cmd, name)
		if len(values) == 0 {
			return "", false
		}
		return values[len(values)-1], true
	}
	name, explicitSession := one("session")
	if !explicitSession {
		name = os.Getenv("LIT_SESSION")
	}
	var mapping Mapping
	if name != "" {
		dir, err := StateDir()
		if err != nil {
			return nil, err
		}
		mapping, err = readMapping(mappingPath(dir, name))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	explicit, supplied := one("endpoint")
	endpoint, err := clientendpoint.Resolve(explicit, supplied, os.Getenv("LIT_ENDPOINT"), mapping.Endpoint)
	if err != nil {
		return nil, err
	}
	q := url.Values{"prefix": {prefix}}
	if kind != "projects" {
		if project, ok := one("project"); ok {
			q.Set("project", project)
		}
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), completionTimeout)
	defer cancel()
	app := &App{Endpoint: endpoint, Mapping: mapping, HTTP: &http.Client{Timeout: completionTimeout}, Context: ctx, Out: io.Discard, Err: io.Discard}
	var result protocol.Completions
	if err := app.Call(http.MethodGet, "/v1/completions/"+kind+"?"+q.Encode(), nil, nil, &result, false); err != nil {
		return nil, err
	}
	// The completion response carries identity, saving a separate meta request
	// while preserving existing bindings even if a service changes at one URL.
	if result.APIMajor != 1 || !protocol.ValidUUID(result.ServiceID) || mapping.ServiceID != "" && result.ServiceID != mapping.ServiceID {
		return nil, protocol.E(409, "wrong_service", "invalid or unexpected completion service")
	}
	return result.Items, nil
}

// flagValues reads a flag's values from cobra's parse during completion. The
// slice interface preserves presence and exact values, unlike GetStringArray.
func flagValues(c *cobra.Command, name string) []string {
	f := c.Flags().Lookup(name)
	if f == nil || !f.Changed {
		return nil
	}
	return f.Value.(pflag.SliceValue).GetSlice()
}

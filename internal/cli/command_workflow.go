package cli

import (
	"fmt"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/workflowsession"
	"github.com/spf13/cobra"
	"io"
)

func addWorkflowCommands(root *cobra.Command, out, errOut io.Writer, code *int) {
	parent := &cobra.Command{Use: "workflow-session", Short: "Bind explicit agent runtime sessions to lit clients", Long: "Bind explicit runtime identity to durable workflow state. Never starts a service.\nFresh sessions and subagents get isolated clients; resume reuses the recorded client.\nLifecycle commands output a JSON association record. run forwards tracker output with --format json;\n--format does not apply to this adapter.", RunE: missingCommand}
	for _, k := range []string{"host", "runtime-id", "cli"} {
		addFlag(parent, parent.PersistentFlags(), k, true)
	}
	for _, action := range []string{"fresh", "subagent", "resume", "reconcile", "run", "checkout"} {
		c := &cobra.Command{Use: action, Short: map[string]string{"fresh": "Create an isolated logical client", "subagent": "Create an isolated child client", "resume": "Resume the recorded client", "reconcile": "Inspect a pending session binding", "run": "Run a tracker command with the bound client", "checkout": "Record a repository checkout path"}[action], Args: cobra.NoArgs}
		if action == "run" {
			c.Use = "run -- COMMAND [ARGS...]"
			c.Args = cobra.MinimumNArgs(1)
		}
		fields := []string{}
		switch action {
		case "fresh", "subagent":
			fields = []string{"project", "discussion-id"}
			if action == "subagent" {
				fields = append(fields, "parent-runtime-id")
			}
		case "checkout":
			fields = []string{"repository", "path"}
		}
		for _, k := range fields {
			addFlag(c, c.Flags(), k, true)
		}
		c.Long = c.Short + ".\nOutput is a JSON association record; --format does not apply."
		if action == "run" {
			c.Long = c.Short + ".\nOutput is forwarded from the tracker command; the adapter supplies --format json.\nHelp and version retain their own text output.\nUse the direct lit tracker command for CLI or Markdown output."
		}
		c.Example = "  lit workflow-session --host codex --runtime-id SESSION " + action
		switch action {
		case "run":
			c.Example += " -- issues list"
		case "checkout":
			c.Example += " --repository github.com/example/repo --path /work/repo"
			c.MarkFlagDirname("path")
			c.Flags().Lookup("path").Usage = "Required checkout `DIRECTORY` on this machine"
			c.Flags().Lookup("repository").Usage = "Required repository `REF` to associate with this checkout"
		case "subagent":
			c.Example += " --parent-runtime-id PARENT"
			c.Flags().Lookup("parent-runtime-id").Usage = "Required parent runtime session `ID`"
		}
		c.RunE = func(cmd *cobra.Command, args []string) error {
			reject := []string{"session", "format", "timeout"}
			if cmd.Name() != "fresh" && cmd.Name() != "subagent" {
				reject = append(reject, "endpoint", "project")
			}
			if err := rejectFlags(cmd, reject...); err != nil {
				return err
			}
			// Normalize the framework's flexible flag placement into the adapter grammar.
			normalized := []string{}
			for _, k := range []string{"host", "runtime-id", "cli"} {
				v := flagValues(cmd, k)
				if len(v) > 1 {
					return fmt.Errorf("duplicate flag --%s", k)
				}
				if len(v) == 1 {
					normalized = append(normalized, "--"+k, v[0])
				}
			}
			normalized = append(normalized, cmd.Name())
			selected := append([]string{}, fields...)
			if cmd.Name() == "fresh" || cmd.Name() == "subagent" {
				selected = append(selected, "endpoint")
			}
			for _, k := range selected {
				v := flagValues(cmd, k)
				if len(v) > 1 {
					return fmt.Errorf("duplicate flag --%s", k)
				}
				if len(v) == 1 {
					normalized = append(normalized, "--"+k, v[0])
				}
			}
			if cmd.Name() == "run" {
				normalized = append(normalized, "--")
				normalized = append(normalized, args...)
			}
			*code = workflowsession.Run(normalized, out, errOut)
			return nil
		}
		hidden := []string{"session", "format", "timeout"}
		if action != "fresh" && action != "subagent" {
			hidden = append(hidden, "endpoint", "project")
		}
		hideInheritedFlags(c, hidden...)
		parent.AddCommand(c)
	}
	parent.PersistentFlags().Lookup("host").Usage = "Required agent `HOST`: codex or claude"
	parent.PersistentFlags().Lookup("runtime-id").Usage = "Required explicit vendor runtime session `ID`"
	hideInheritedFlags(parent, "session", "format", "timeout", "endpoint", "project")
	root.AddCommand(parent)
}

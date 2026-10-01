package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Cobra owns routing, flag syntax, help and completion. The ordered domain
// parser still consumes the original tokens: flattening flags would destroy
// atomic item boundaries and lose duplicate scalar errors.
func Run(argv []string, out, errOut io.Writer) int {
	code := 0
	root := newCommandTree(argv, out, errOut, &code)
	root.InitDefaultCompletionCmd()
	root.InitDefaultHelpCmd()
	root.SetArgs(commandHelpAlias(root, argv))
	if _, err := root.ExecuteC(); err != nil {
		format := offlineFormat(argv)
		if format == "" {
			format = "json"
		}
		return (&App{Out: out, Err: errOut, Format: format}).Error(protocol.E(400, "invalid_arguments", err.Error()))
	}
	return code
}

func newCommandTree(argv []string, out, errOut io.Writer, code *int) *cobra.Command {
	root := &cobra.Command{Use: "lit", Short: "Track projects and issues with lit", Long: "Track projects, issues, comments and work claims. Commands never start the service.\nStart with connect [--session NAME], then select a project with session set.", SilenceErrors: true, SilenceUsage: true}
	root.SetOut(out)
	root.SetErr(errOut)
	root.RunE = missingCommand
	for _, name := range []string{"session", "endpoint", "format", "timeout", "project"} {
		addFlag(root, root.PersistentFlags(), name, true)
	}
	run := func(c *cobra.Command, args []string) { *code = runTracker(argv, out, errOut) }
	for _, name := range []string{"connect", "disconnect", "version"} {
		c := &cobra.Command{Use: name, Short: map[string]string{"connect": "Initialize or resume a client session", "disconnect": "End a connected client session", "version": "Print the binary version"}[name], Run: run, Args: cobra.NoArgs}
		for k, v := range allowedFlags(Args{Command: name}) {
			if !globalValue("--" + k) {
				addFlag(c, c.Flags(), k, v)
			}
		}
		c.Example = "  lit " + name + " --session my-session"
		if name == "version" {
			c.Example = "  lit version"
		}
		root.AddCommand(c)
	}
	for _, family := range []string{"session", "projects", "issues", "milestones", "comments", "claims", "transactions"} {
		parent := &cobra.Command{Use: family, Short: map[string]string{"session": "Inspect or change session preferences", "projects": "Create and discover project containers", "issues": "Track work, state and relationships", "milestones": "Group project issues into named worksets", "comments": "Discuss issues with durable comments", "claims": "Reserve issues with expiring work claims", "transactions": "Reconcile uncertain mutation outcomes"}[family], RunE: missingCommand}
		verbs := map[string]string{"session": "get set unset", "projects": "create get list update history", "issues": "create get list update close reopen link unlink history", "milestones": "create get list update history", "comments": "create get list update history", "claims": "acquire get list renew release", "transactions": "status"}[family]
		for _, verb := range strings.Fields(verbs) {
			c := &cobra.Command{Use: verb + commandUsage(family, verb), Short: commandSummary(family, verb), Run: run, Example: commandExample(family, verb)}
			c.Long = commandLong(family, verb)
			for k, v := range commandFlags(family, verb) {
				if !globalValue("--" + k) {
					addFlag(c, c.Flags(), k, v)
				}
			}
			customizeFlags(c, family, verb)
			parent.AddCommand(c)
		}
		root.AddCommand(parent)
	}
	grep := &cobra.Command{Use: "grep PATTERN", Short: "Search current content using a literal substring", Long: "Search the selected project, or explicitly use --all-projects. Matching is case-insensitive by default.\nUse -- before patterns beginning with a dash; grep -- --help searches for literal --help.", Example: "  lit grep --session my-session 'renewal'\n  lit grep --session my-session -- --help", Run: run, Args: cobra.ExactArgs(1)}
	for _, k := range []string{"project", "limit", "cursor", "context"} {
		addFlag(grep, grep.Flags(), k, true)
	}
	for _, k := range []string{"case-sensitive", "all-projects", "n"} {
		addFlag(grep, grep.Flags(), k, false)
	}
	root.AddCommand(grep)
	setup := &cobra.Command{Use: "setup-skills", Short: "Install embedded skills for Codex or Claude", Long: setupSkillsHelp, Example: "  lit setup-skills --scope local --agent both\n  lit setup-skills --scope custom --path /work/project --agent codex", Args: cobra.NoArgs, PreRunE: func(c *cobra.Command, args []string) error {
		return rejectFlags(c, "session", "endpoint", "format", "timeout", "project")
	}, Run: func(c *cobra.Command, args []string) {
		normalized := []string{}
		for _, k := range []string{"scope", "agent", "path"} {
			values := flagValues(c, k)
			for _, value := range values {
				normalized = append(normalized, "--"+k, value)
			}
		}
		*code = runSetupSkills(normalized, out, errOut)
	}}
	for _, k := range []string{"scope", "agent", "path"} {
		addFlag(setup, setup.Flags(), k, true)
	}
	setup.MarkFlagDirname("path")
	hideInheritedFlags(setup, "session", "endpoint", "format", "timeout", "project")
	root.AddCommand(setup)
	addWorkflowCommands(root, out, errOut, code)
	addCommandRequirements(root)
	return root
}
func missingCommand(c *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown command %q for %s; run '%s --help'", args[0], c.CommandPath(), c.CommandPath())
	}
	return fmt.Errorf("a subcommand is required for %s; run '%s --help'", c.CommandPath(), c.CommandPath())
}

// A bare trailing help is an alias only when it occupies the command slot
// before any operand/flag. Never reinterpret flag values or literal search data.
func commandHelpAlias(root *cobra.Command, args []string) []string {
	c := root
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if strings.HasPrefix(a, "-") {
			name := strings.TrimPrefix(strings.SplitN(a, "=", 2)[0], "--")
			flag := c.Flags().Lookup(name)
			if flag == nil {
				flag = c.InheritedFlags().Lookup(name)
			}
			if flag == nil {
				flag = c.PersistentFlags().Lookup(name)
			}
			if flag == nil {
				break
			}
			if flag.NoOptDefVal == "" && !strings.Contains(a, "=") {
				i++
			}
			continue
		}
		if a == "help" && c != root {
			result := append([]string{}, args...)
			result[i] = "--help"
			return result
		}
		found := false
		for _, child := range c.Commands() {
			if child.Name() == a {
				c = child
				found = true
				break
			}
		}
		if !found || c.Name() == "grep" || c.Name() == "run" {
			break
		}
	}
	return args
}

func addFlag(c *cobra.Command, f *pflag.FlagSet, name string, value bool) {
	description := flagDescriptions[name]
	if description == "" {
		panic("missing flag description: " + name)
	}
	if value {
		if name == "context" {
			f.StringArrayP(name, "C", nil, description)
		} else {
			f.StringArray(name, nil, description)
		}
	} else {
		if name == "n" {
			f.BoolP(name, "n", false, description)
		} else {
			f.Bool(name, false, description)
		}
	}
	choices := map[string][]string{"format": {"cli", "markdown", "json"}, "output-format": {"cli", "markdown", "json"}, "state": {"open", "closed"}, "actor-kind": {"human", "agent"}, "direction": {"asc", "desc"}, "relation": {"blocks", "blocked-by", "related"}, "scope": {"local", "user", "custom"}, "agent": {"codex", "claude", "both"}, "host": {"codex", "claude"}, "blocked": {"true", "false"}, "claimed": {"true", "false"}}
	if values, ok := choices[name]; ok && value {
		c.RegisterFlagCompletionFunc(name, func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return values, cobra.ShellCompDirectiveNoFileComp
		})
	}
}

// pflag GetStringArray round-trips through CSV and drops an explicitly empty
// first value. The slice interface preserves both presence and exact values.
func flagValues(c *cobra.Command, name string) []string {
	f := c.Flags().Lookup(name)
	if f == nil || !f.Changed {
		return nil
	}
	return f.Value.(pflag.SliceValue).GetSlice()
}

func rejectFlags(c *cobra.Command, names ...string) error {
	for _, name := range names {
		if f := c.Flags().Lookup(name); f != nil && f.Changed {
			return fmt.Errorf("--%s does not apply to %s", name, c.CommandPath())
		}
	}
	return nil
}

func customizeFlags(c *cobra.Command, family, verb string) {
	if family == "milestones" && verb == "create" {
		c.Flags().Lookup("issue").Usage = "Existing issue `REF` to include in this milestone (repeatable within each item)"
	}
	if family == "issues" && verb == "list" {
		c.Flags().Lookup("milestone").Usage = "Milestone `REF` whose members to include; requires project scope"
	}
	if family == "comments" && (verb == "create" || verb == "list") {
		c.Flags().Lookup("issue").Usage = "Owning issue `ISSUE_REF`"
		if verb == "create" {
			c.Flags().Lookup("issue").Usage += "; repeat to begin another atomic comment item"
			c.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
				if err.Error() == "unknown flag: --comment" {
					return fmt.Errorf("comments create: use --issue ISSUE_REF instead of --comment")
				}
				return err
			})
		}
	}
	if family == "claims" && (verb == "renew" || verb == "release") {
		c.Flags().Lookup("all").Usage = strings.ToUpper(verb[:1]) + verb[1:] + " all claims owned by this session; conflicts with explicit targets and --project"
	}
	if family == "session" && (verb == "get" || verb == "unset") {
		c.Flags().VisitAll(func(f *pflag.Flag) {
			if f.NoOptDefVal != "" {
				action := "Show"
				if verb == "unset" {
					action = "Clear"
				}
				f.Usage = action + " session property " + f.Name
			}
		})
	}
	if f := c.Flags().Lookup("sort"); f != nil {
		values := []string{"created-at", "updated-at", "title"}
		if family == "comments" {
			values = values[:2]
		}
		if family == "claims" {
			values = values[:1]
		}
		f.Usage = "Sort `FIELD`: " + strings.Join(values, ", ")
		c.RegisterFlagCompletionFunc("sort", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return values, cobra.ShellCompDirectiveNoFileComp
		})
	}
	if f := c.Flags().Lookup("clear"); f != nil {
		values := []string{"content"}
		if family == "projects" {
			values = append(values, "repositories")
		}
		if family == "issues" {
			values = append(values, "labels", "assignee", "parent")
		} else if family == "milestones" {
			values = append(values, "issues")
		}
		f.Usage = "Clear `FIELD` (repeatable): " + strings.Join(values, ", ")
		c.RegisterFlagCompletionFunc("clear", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return values, cobra.ShellCompDirectiveNoFileComp
		})
	}
}

// Shadow inherited flags with command-local hidden copies. Mutating the shared
// parent's Hidden bit would hide valid flags from sibling commands as well.
func hideInheritedFlags(c *cobra.Command, names ...string) {
	for _, name := range names {
		if c.Flags().Lookup(name) == nil {
			addFlag(c, c.Flags(), name, true)
		}
		c.Flags().MarkHidden(name)
	}
}

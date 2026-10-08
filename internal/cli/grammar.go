package cli

import (
	"strings"

	"github.com/nerdwave-nick/nerdwave-workflows/internal/nwcli"
)

// grammar is lit's command tree: the single description of which flags and
// operands each command accepts, how flags repeat, and where items begin.
// The argument parsers read it; nothing else may restate those rules.
var grammar = newGrammar()

func choice(values ...string) nwcli.Value { return nwcli.Value{Kind: nwcli.Choice, Choices: values} }
func dynamic(completer string) nwcli.Value {
	return nwcli.Value{Kind: nwcli.Dynamic, Completer: completer}
}

// flagDefaults holds how a flag behaves wherever it appears; commands override
// the exceptions explicitly. A missing entry is a single free-form value.
var flagDefaults = map[string]nwcli.Flag{
	"session": {Value: dynamic("sessions")}, "endpoint": {Value: dynamic("endpoints")}, "for": {Value: dynamic("durations")},
	"format": {Value: choice("cli", "markdown", "json")}, "output-format": {Value: choice("cli", "markdown", "json")},
	"actor-kind": {Value: choice("human", "agent")}, "state": {Value: choice("open", "closed")},
	"direction": {Value: choice("asc", "desc")}, "relation": {Value: choice("blocks", "blocked-by", "related")},
	"blocked": {Value: choice("true", "false")}, "claimed": {Value: choice("true", "false")},
	"scope": {Value: choice("local", "user", "custom")}, "agent": {Value: choice("codex", "claude", "both")}, "host": {Value: choice("codex", "claude")},
	"project": {Value: dynamic("projects")}, "project-id": {Value: dynamic("projects")},
	"issue": {Value: dynamic("issues")}, "parent": {Value: dynamic("issues")}, "from": {Value: dynamic("issues")},
	"to": {Value: dynamic("issues"), Repeat: nwcli.Many}, "add-issue": {Value: dynamic("issues"), Repeat: nwcli.Many}, "remove-issue": {Value: dynamic("issues"), Repeat: nwcli.Many},
	"comment": {Value: dynamic("comments")}, "milestone": {Value: dynamic("milestones")},
	"file": {Value: nwcli.Value{Kind: nwcli.Path}}, "content-file": {Value: nwcli.Value{Kind: nwcli.Path}}, "cli": {Value: nwcli.Value{Kind: nwcli.Path}},
	"path":  {Value: nwcli.Value{Kind: nwcli.Dir}},
	"query": {Short: "q"}, "context": {Short: "C"}, "n": {Short: "n", Switch: true},
	"label": {Repeat: nwcli.Many}, "add-label": {Repeat: nwcli.Many}, "remove-label": {Repeat: nwcli.Many},
	"repository": {Repeat: nwcli.Many}, "add-repository": {Repeat: nwcli.Many}, "remove-repository": {Repeat: nwcli.Many},
	"id": {Repeat: nwcli.Many}, "repository-ref": {Repeat: nwcli.Many},
	"labels-all": {Repeat: nwcli.Many}, "labels-any": {Repeat: nwcli.Many}, "labels-none": {Repeat: nwcli.Many},
	"session-id-only": {Switch: true}, "all": {Switch: true}, "all-projects": {Switch: true}, "force": {Switch: true},
	"session-id": {Switch: true}, "service-id": {Switch: true}, "status": {Switch: true}, "case-sensitive": {Switch: true},
}

// flags declares flags by name with their defaults.
func flags(names string) []nwcli.Flag {
	out := []nwcli.Flag{}
	for _, name := range strings.Fields(names) {
		f := flagDefaults[name]
		f.Name = name
		out = append(out, f)
	}
	return out
}

// switches declares flags that only switch something on in this command.
func switches(names string) []nwcli.Flag {
	out := flags(names)
	for i := range out {
		out[i].Switch, out[i].Value, out[i].Repeat = true, nwcli.Value{}, nwcli.Once
	}
	return out
}

// item declares an item flag: each occurrence begins an item described by fields.
func item(name string, v nwcli.Value, fields ...[]nwcli.Flag) nwcli.Flag {
	return nwcli.Flag{Name: name, Value: v, Fields: join(fields...)}
}

// invocationFlags apply to a whole invocation, never to one record item. The
// record parsers keep them out of item groups, including for commands without
// item flags, whose other flags form the single implicit group.
var invocationFlags = map[string]bool{"session": true, "endpoint": true, "format": true, "timeout": true, "project": true, "file": true, "force": true}

// with overrides one flag's value description in fs.
func with(fs []nwcli.Flag, name string, v nwcli.Value, repeat nwcli.Repeat) []nwcli.Flag {
	for i := range fs {
		if fs[i].Name == name {
			fs[i].Value, fs[i].Repeat = v, repeat
		}
	}
	return fs
}

func join(parts ...[]nwcli.Flag) []nwcli.Flag {
	out := []nwcli.Flag{}
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func refs(usage, completer string) nwcli.Operands {
	max := nwcli.Unlimited
	if !strings.HasSuffix(usage, "...") && !strings.HasSuffix(usage, "...]") {
		max = 1
	}
	return nwcli.Operands{Usage: usage, Max: max, Value: dynamic(completer)}
}

const globals = "session endpoint format timeout"

var free = nwcli.Value{}

func newGrammar() *nwcli.Command {
	root := &nwcli.Command{Name: "lit"}
	add := func(cmds ...*nwcli.Command) { root.Commands = append(root.Commands, cmds...) }
	add(
		&nwcli.Command{Name: "connect", Flags: join(flags(globals+" output-format client-id actor-name actor-kind project runtime-vendor runtime-session-id"), switches("session-id-only"))},
		&nwcli.Command{Name: "disconnect", Flags: flags(globals)},
		&nwcli.Command{Name: "version", Flags: flags(globals)},
		group("session",
			&nwcli.Command{Name: "get", Flags: join(flags(globals), switches(sessionFields+" session-id service-id status revision"))},
			&nwcli.Command{Name: "set", Flags: flags(globals + " " + sessionFields)},
			&nwcli.Command{Name: "unset", Flags: join(flags(globals), switches(sessionFields))},
		),
		recordFamily("projects"), recordFamily("issues"), recordFamily("milestones"), recordFamily("comments"),
		group("claims",
			&nwcli.Command{Name: "acquire", Operands: refs("REF...", "issues"), Flags: join(flags(globals+" for until project"), switches("force"))},
			&nwcli.Command{Name: "get", Operands: refs("REF...", "issues"), Flags: flags(globals + " project")},
			&nwcli.Command{Name: "list", Flags: join(flags(globals+" owner-client-id issue-id direction limit cursor file"), sortFlag("created-at"), switches("all"))},
			&nwcli.Command{Name: "renew", Operands: refs("[REF...]", "issues"), Flags: join(flags(globals+" for until project"), switches("all"))},
			&nwcli.Command{Name: "release", Operands: refs("[REF...]", "issues"), Flags: join(flags(globals+" project"), switches("all"))},
		),
		group("transactions", &nwcli.Command{Name: "status", Flags: flags(globals + " file")}),
		&nwcli.Command{Name: "grep", Operands: nwcli.Operands{Usage: "PATTERN", Max: 1},
			Flags: join(flags(globals+" project limit cursor context n"), switches("case-sensitive all-projects"))},
		&nwcli.Command{Name: "setup-skills", Flags: flags("scope agent path")},
		workflowSession(),
	)
	if err := root.Validate(); err != nil {
		panic(err)
	}
	return root
}

// itemFlag names the flag that begins items in a command, or "" for none.
func itemFlag(path ...string) string {
	name := ""
	if cmd := grammar.Find(path...); cmd != nil {
		cmd.EachFlag(func(f nwcli.Flag, owner *nwcli.Flag) {
			if owner == nil && len(f.Fields) > 0 {
				name = f.Name
			}
		})
	}
	return name
}

// grammarFlags maps each flag of a command, fields included, to whether it
// takes a value.
func grammarFlags(path ...string) map[string]bool {
	m := map[string]bool{}
	if cmd := grammar.Find(path...); cmd != nil {
		cmd.EachFlag(func(f nwcli.Flag, _ *nwcli.Flag) { m[f.Name] = !f.Switch })
	}
	return m
}

const sessionFields = "project output-format actor-name actor-kind runtime-vendor runtime-session-id"

func group(name string, cmds ...*nwcli.Command) *nwcli.Command {
	return &nwcli.Command{Name: name, Commands: cmds}
}

func sortFlag(values ...string) []nwcli.Flag {
	return []nwcli.Flag{{Name: "sort", Value: choice(values...)}}
}

func clearFlag(values ...string) []nwcli.Flag {
	return []nwcli.Flag{{Name: "clear", Value: choice(values...), Repeat: nwcli.Many}}
}

// recordFamily declares projects, issues, milestones or comments. Mutations
// that can describe several records do so through an item flag whose fields
// describe one record; fields given before it describe the operands.
func recordFamily(family string) *nwcli.Command {
	wide := flags(globals + " project file")
	if family == "issues" || family == "comments" {
		wide = append(wide, switches("force")...)
	}
	plain := flags(globals + " project")
	cmd := func(verb string, operands nwcli.Operands, parts ...[]nwcli.Flag) *nwcli.Command {
		return &nwcli.Command{Name: verb, Operands: operands, Flags: join(parts...)}
	}
	none := nwcli.Operands{}
	sorts := []string{"created-at", "updated-at", "title"}
	if family == "comments" {
		sorts = sorts[:2]
	}
	list := join(flags(globals+" project file direction limit cursor"), sortFlag(sorts...), switches("all"),
		flags("id query created-after created-before updated-after updated-before"))
	history := join(plain, flags("request-hash limit cursor"), switches("all"))
	body := flags("content content-file")
	g := group(family)
	switch family {
	case "projects":
		g.Commands = []*nwcli.Command{
			cmd("create", none, flags(globals+" project file"), []nwcli.Flag{item("project-title", free, body, flags("repository"))}),
			cmd("get", refs("[REF...]", "projects"), plain),
			cmd("list", none, list, flags("title repository-ref")),
			cmd("update", refs("[REF...]", "projects"), flags(globals+" project file"),
				[]nwcli.Flag{item("project-id", dynamic("projects"), body, flags("title add-repository remove-repository revision"), clearFlag("content", "repositories"))}),
			cmd("history", refs("[REF]", "projects"), history),
		}
	case "issues":
		target := func(fields ...[]nwcli.Flag) []nwcli.Flag {
			return []nwcli.Flag{item("issue", dynamic("issues"), fields...)}
		}
		g.Commands = []*nwcli.Command{
			cmd("create", none, wide, []nwcli.Flag{item("issue", free, body, flags("parent state label assignee"))}),
			cmd("get", refs("REF...", "issues"), plain),
			cmd("list", none, list, flags("title parent state labels-all labels-any labels-none assignee blocked claimed owner-client-id milestone"), switches("all-projects")),
			cmd("update", refs("[REF...]", "issues"), wide, target(body, flags("title parent state assignee add-label remove-label revision"), clearFlag("content", "labels", "assignee", "parent"))),
			cmd("close", refs("[REF...]", "issues"), wide, target(flags("revision"))),
			cmd("reopen", refs("[REF...]", "issues"), wide, target(flags("revision"))),
			cmd("link", none, wide, []nwcli.Flag{item("from", dynamic("issues"), flags("to relation"))}),
			cmd("unlink", none, wide, []nwcli.Flag{item("from", dynamic("issues"), flags("to relation"))}),
			cmd("history", refs("REF", "issues"), history),
		}
	case "milestones":
		g.Commands = []*nwcli.Command{
			cmd("create", none, flags(globals+" project file"), []nwcli.Flag{item("milestone", free, body, with(flags("issue"), "issue", dynamic("issues"), nwcli.Many))}),
			cmd("get", refs("REF...", "milestones"), plain),
			cmd("list", none, list, flags("title")),
			cmd("update", refs("[REF...]", "milestones"), flags(globals+" project file"), body, flags("title add-issue remove-issue revision"), clearFlag("content", "issues")),
			cmd("history", refs("REF", "milestones"), history),
		}
	case "comments":
		g.Commands = []*nwcli.Command{
			cmd("create", none, wide, []nwcli.Flag{item("issue", dynamic("issues"), body, flags("author"))}),
			cmd("get", refs("REF...", "comments"), plain),
			cmd("list", none, list, flags("author issue")),
			cmd("update", refs("[REF...]", "comments"), wide, []nwcli.Flag{item("comment", dynamic("comments"), body, flags("author revision"), clearFlag("content"))}),
			cmd("history", refs("REF", "comments"), history),
		}
	}
	return g
}

// workflowSession declares the adapter. Its host flags belong to the group and
// are given before the action; fresh and subagent also accept --endpoint.
func workflowSession() *nwcli.Command {
	g := group("workflow-session",
		&nwcli.Command{Name: "fresh", Flags: flags("endpoint project discussion-id")},
		&nwcli.Command{Name: "subagent", Flags: flags("endpoint project discussion-id parent-runtime-id")},
		&nwcli.Command{Name: "resume"},
		&nwcli.Command{Name: "reconcile"},
		&nwcli.Command{Name: "run", Operands: nwcli.Operands{Usage: "-- COMMAND [ARGS...]", Max: nwcli.Unlimited}},
		&nwcli.Command{Name: "checkout", Flags: with(flags("repository path"), "repository", free, nwcli.Once)},
	)
	g.Flags = flags("host runtime-id cli")
	return g
}

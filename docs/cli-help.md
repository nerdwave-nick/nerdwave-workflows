# Command help and completion

Use focused help at any command level:

```sh
lit help
lit projects help
lit projects list --help
lit projects list -h
lit projects list help
lit help projects list
```

Family help lists its commands. Leaf help explains that operation's flags,
argument grammar and examples. Each application command has a Requirements section
and Valid forms showing alternatives: brackets mean optional inputs, `|` separates
choices, and `...` means repeatable. Flag descriptions distinguish required,
conditional and optional inputs; conditional inputs depend on the chosen form or
a saved selection, so they are not unconditional Cobra requirements.

`connect` needs no flags. It uses `--session NAME`, then `LIT_SESSION`, or creates
a fresh random throwaway name when neither is supplied. The result exposes
`items[0].logical_session` (Logical session in CLI/Markdown output); reuse it
explicitly for subsequent commands. Generated sessions are saved, not automatically
expired or deleted. Other tracker commands require `--session`
or `LIT_SESSION`; `version` needs neither, and `transactions status` only uses an
explicit `--session` as a filter. Actor defaults, saved-client resume, input-file
alternatives, project selection and flag conflicts are explained in the relevant
command help.

Help and completion are offline: they do not
connect to lit, create sessions or install skills.

## Shell completion

Generate completion with the same installed `lit` that you run. These commands
print scripts; they never modify your shell configuration automatically.

For the current Bash shell:

```bash
source <(lit completion bash)
```

For the current Fish shell:

```fish
lit completion fish | source
```

For the current Zsh shell (initialize completion first if needed):

```zsh
autoload -Uz compinit
compinit
source <(lit completion zsh)
```

For the current PowerShell session:

```powershell
lit completion powershell | Out-String | Invoke-Expression
```

`lit completion SHELL --help` explains persistent installation for your shell.
Command names, flag names and fixed values such as `--format cli|markdown|json`,
`--state open|closed`, `--scope local|user|custom`, and `--agent codex|claude|both`
complete without contacting the service. Project and issue names are not fetched.

## Batch and literal arguments

The Cobra command tree preserves the ordered domain argument grammar. Repeated
item boundary flags start atomic items; scalar duplicates within one item fail.
For example:

```sh
lit projects create --session demo \
  --project-title test/one --content 'First' \
  --project-title test/two --content 'Second'
lit issues link --session demo \
  --from A --to B --to C --relation blocks \
  --from B --to C --relation related
lit grep --session demo -- --help
```

The `grep` example searches for literal `--help`. Use `issues get -- help` to fetch an issue titled `help` (bare `issues get help` opens help). Arguments after `--` remain literal operands. Likewise `grep help` searches for
`help`, and `--content help` supplies content. Workflow passthrough uses an explicit
separator, for example `workflow-session --host codex --runtime-id ID run -- issues list`.

Comment creation uses `comments create --issue ISSUE_REF --content TEXT`; repeat
`--issue` to begin another comment, even for the same owner. Comment updates use
`comments update --comment COMMENT_REF --content TEXT`. JSON creation items
retain their `issue` property.

CLI argument errors retain the JSON error contract with `--format json` and exit
code 2. Service and reconciliation outcomes retain their existing exit codes.

Implementation references: [Cobra user guide](https://github.com/spf13/cobra/blob/main/site/content/user_guide.md),
[Cobra completion guide](https://github.com/spf13/cobra/blob/main/site/content/completions/_index.md),
and [pflag](https://github.com/spf13/pflag).

## Existing record references

A reference selects an existing record; it is not the syntax for naming a new
project or issue. Title matching is case-insensitive. A UUID prefix must be
unique; use the full UUID when a prefix is ambiguous.

| Record | Accepted reference forms |
| --- | --- |
| Project | Exact title, UUID, unique hexadecimal UUID prefix, `title:TITLE`, or `id:UUID_PREFIX` |
| Issue | Title in the selected project, UUID, unique UUID prefix, `title:TITLE`, `id:UUID_PREFIX`, or `PROJECT_TITLE:ISSUE_REF` |
| Comment | UUID or unique UUID prefix, optionally preceded by `id:`; no title or project-qualified form |

`title:` forces literal title matching, and `id:` forces ID matching. These
resolve title-versus-ID ambiguity without renaming a record. For issues, a
project-qualified reference such as `feat/api:title:Review API` uses that
project's title and scopes the following title or ID prefix to it. An unqualified
issue UUID or ID prefix remains global even when a session project is selected.
A plain issue title or `title:TITLE` needs `--project` or a saved project selection.

The issue resolver treats a colon after text containing `/` as project
qualification. To address a title that itself contains slashes and colons, force
literal title matching and quote the complete argument:

```sh
lit issues get --session my-session --project test/poc 'title:Refactor a/b: cleanup'
lit issues get --session my-session 'feat/api:title:Review API'
lit comments get --session my-session id:01234567
```

The final example needs a matching unique comment ID prefix. Flags explicitly
named `--id`, `--issue-id` or `--owner-client-id` are query UUID fields, not this
reference grammar. Use each command's References section for its applicable
record types. See [recovery limitations](recovery.md) for uncertain outcomes;
selector disambiguation does not resolve mutation uncertainty.

`lit session get` shows both the selected logical session name (`logical_session`)
and its client UUID (`client_id`). The `--session-id` property selector retains its
client UUID meaning; property-filtered results contain only the selected fields.

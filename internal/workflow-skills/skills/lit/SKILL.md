---
name: lit
description: Use lit for tracker projects, issues, comments, claims, and session context; apply shared ownership and recovery rules for tracker-backed workflows.
---

# lit

The client executable is `lit`; the server executable is `lit-server`. The skill name
remains `lit`. Connections do not start the server.

Use the installed `lit --help` for the available version's syntax. Read
repository conventions in `docs/agents/issue-tracker.md`, resolving paths against
that repository. Tracker data is remote authority; do not read its storage files.

Establish an explicit logical session using [host setup](references/hosts.md).
Independent agents and subagents need distinct sessions. Resume only the same
logical working session. Pass session selection on every call; a child process
cannot export settings into its parent. Changing cwd never selects a project.
Select a project explicitly or retain the confirmed server session selection.
Ask when project context is ambiguous. Repository evidence can support a proposal,
not silently select a project.

Use the relevant short recipe:

- [Connect and select a project](references/connect.md).
- [Find, claim, and read work](references/work.md).
- [Checkpoint, recover, and finish](references/recovery.md).

The initial suite includes all fifteen agreed skills and the operations used by
these recipes. Check executable help for the installed version; if it lacks a
required capability, report the version mismatch rather than starting unclaimed
implementation.

Assignment records responsibility; a work claim grants temporary ownership.
Sustained work requires an explicit claim. Claims default to thirty minutes,
are capped at one hour, and need explicit renewal. Ordinary edits do not renew.
After expiry, reacquire and reread current context before continuing. Agents never
force takeover. Claims do not protect Git files: coordinate shared checkouts or
use separate worktrees.

Projects own shared requirements and can span repositories. Put repository
identities and roles in project content; use local checkout mappings for machine
paths. Read each affected repository's instructions and domain docs. Tickets own
their slice and acceptance criteria; they cannot override the shared specification.
Workflow stages are independently invokable, and planning does not authorize
implementation, external consultation, push, publication, or deployment.

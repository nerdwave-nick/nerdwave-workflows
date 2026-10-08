# Direct vendor CLI sessions

Use the installed vendor's help as the syntax authority: `claude --help`,
`codex exec --help`, and `codex exec resume --help`. Initial and resume commands
can support different options. Do not guess flags or copy an old invocation
without checking it. Help checks require no model call; running an actual
consultation requires the authorization in the skill.

## Prepare and invoke

1. Locate the discussion's existing notes. Match its scope and, when tracked,
   service/project/ticket identity, rather than inferring identity from the current
   checkout. Reuse the recorded provider, model, effort and explicit vendor session
   ID. A fresh approved session gets a new note while preserving previous evidence.
2. Choose a neutral working directory outside the project and record its absolute
   path. For later calls reuse that directory even if the caller changed repositories.
   Provide absolute artifact paths or relevant contents. A neutral cwd reduces
   accidental project configuration loading; it is not a sandbox.
3. Inspect supported permission and configuration controls. Select read-only tools
   or sandbox permissions, disable approval-based escalation, and isolate unrelated
   MCP integrations, hooks, plugins and configuration. Check resume supports the
   same effective restrictions. If they cannot be established, stop and report the
   limitation instead of invoking. Do not edit the user's global configuration.
4. Invoke the vendor CLI directly with the exact selected model/effort and a neutral
   briefing. Capture the returned session ID and answer using the vendor's supported
   output. Keep vendor session persistence enabled. Resume by explicit ID; never
   choose latest, continue implicitly, fork, or enable model fallback.
5. Update the discussion notes with the result or exact failure, transcript/output
   reference, and next question. Only one caller should drive a session at a time;
   coordinate ownership in the existing handoff/ticket before sending a follow-up.

Notes can be ordinary Markdown or existing tracker comments. No prescribed JSON
schema, new database, lock service, or separate helper is required. Vendor CLIs
own their session transcripts; the notes connect those sessions to the discussion.
Model availability, authentication and actual permission enforcement depend on
the installed vendor environment. Successful help parsing does not prove them.

## Interrupted turns and earlier helper records

Preserve existing consultation JSON records, lock directories and vendor
transcripts unchanged. Earlier installations kept records beneath `consultations/`
in `LIT_WORKFLOW_STATE_DIR` (or the platform workflow-state default); they remain
useful evidence, not input for an automatic migration. Read a matching record to
recover its approved question, provider/model/effort, explicit session ID and turn
outcomes, then reference it from ordinary notes. The former helper used
`<workflow-state>/consultations/runs/<record_id>` as its invocation directory;
verify that directory and the vendor transcript instead of inventing or recreating
missing session state.

An `in_flight` or `uncertain` turn, an old lock, a timeout, or a missing response
requires investigating the vendor transcript and whether a process is still active
before another caller proceeds. Do not delete locks, replay an uncertain turn, or
infer a new session on failure. When the outcome cannot be established, report it
and retain the evidence. Any fresh replacement consultation needs explicit approval.

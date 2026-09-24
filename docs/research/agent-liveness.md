# Agent liveness evidence for lit

Investigated 2026-09-24. This resolves an evidence question, not the choice of lease policy.

## Conclusion

A local helper is viable. It need not launch every agent itself: current Claude Code explicitly supports external session-state queries, and the installed Codex exposes a shared app-server integration. However, `{type, session_id}` only identifies the conversation within a runtime. The helper must know which host/configuration/runtime owns it, be able to query that runtime, and associate it with the specific lit claim. Process existence is not evidence that useful work continues.

## Version and verification scope

Read-only local inspection found `codex-cli 0.153.4` and `Claude Code 2.1.281`. Inspected help only; generated public Codex schemas without starting a service. No live agent registry, private transcript, credentials, or session contents were read. Public documentation is rolling; version-gate adapters and test them against the actual installed versions. This is interface research, not end-to-end validation of a helper.

Local evidence can be reproduced with:

```sh
codex --version
codex agents --help
codex app-server proxy --help
codex app-server generate-json-schema --out /tmp/lit-codex-schema
claude --version
claude agents --help
```

## Codex

The official app-server API supports metadata-only `thread/read` without resuming or subscribing. Its runtime statuses distinguish unloaded, idle, error, and active threads. `thread/status/changed`, turn boundaries, and item lifecycle notifications provide additional signals when integrated with the owning server. `thread/loaded/list` means loaded in memory, not actively working. Unsubscribing is not immediate shutdown: an inactivity grace period can retain the thread. [OpenAI app-server documentation](https://learn.chatgpt.com/docs/app-server#read-a-stored-thread-without-resuming)

Installed-version evidence: `codex agents --help` describes a shared local app-server daemon. `codex app-server proxy --help` provides `--sock` for its control socket. Generated `v2/ThreadReadResponse.json` defines `ThreadActiveFlag` values `waitingOnApproval` and `waitingOnUserInput`; `v2/ThreadReadParams.json` accepts `threadId` and optional `includeTurns`. Therefore an adapter can investigate polling the owning runtime with `includeTurns: false`; no transcript parsing or synthetic resume should be needed for a status snapshot. This path was not connected/tested here. The command group labels app-server experimental.

Hooks offer an alternative: session start/resume, user submission, tool boundaries, permission requests, stop, interruption, and session end. Common input carries `session_id`; subagent hooks use the parent's session ID, with separate subagent identity where supplied. Hook tool coverage is incomplete: hosted tools can bypass it and polling `write_stdin` does not repeat the pre-tool event. Transcripts are explicitly not a stable hook interface. `SessionEnd` applies to main threads and can occur after the disconnected idle grace period, rather than when a terminal merely switches away. [OpenAI hooks documentation](https://learn.chatgpt.com/docs/hooks)

Inference: a helper needs endpoint discovery/configuration or hook registration. A session running in another app-server instance cannot safely be declared dead from this server's unloaded status. Hook start/stop pairs alone do not prove continuous progress during silent tools or recover missed events.

## Claude Code

`claude agents --json --all` is the documented external state interface. It covers live interactive sessions and background sessions; the TUI has narrower display scope. Match full `sessionId`, not the background job's short `id`. Live entries expose `status` (`busy`, `waiting`, `idle`) and waiting reasons. Background entries additionally expose `state` (`working`, `blocked`, `done`, `failed`, `stopped`). `working` can include a scheduled wait between autonomous steps, not just current computation. Completed background sessions require `--all`; exited interactive sessions are not durable rows promised by this interface. Separate configuration directories have separate runtime scope. Internal jobs files are not stable interfaces; agent view is a research preview. [Claude Code agent-view documentation](https://code.claude.com/docs/en/agent-view#read-session-state-from-a-script)

Hooks carry `session_id` and provide start/resume, user prompt, tool start/success/failure, permission request, stop, and session end signals. `SessionStart` identifies resume versus fresh startup. Notifications are not universal immediate state transitions: idle notification is delayed and depends on user inactivity; use permission hooks for immediate permission requests. End hooks have limited execution time. Subagents have their own IDs in lifecycle events. [Claude Code hook reference](https://code.claude.com/docs/en/hooks)

Installed `claude agents --help` independently confirms `--json` includes interactive and background sessions and `--all` includes completed background sessions. Top-level help confirms resume preserves a conversation while `--fork-session` creates another session ID; minimal/safe invocation modes can disable hooks.

Inference: a helper can observe independently started compatible sessions without a wrapper or hooks, provided it runs in the correct local configuration scope. Hooks remain useful for registration and finer transitions. Older installations or unavailable agent-view support require another adapter or explicit renewal; do not silently assume the command exists.

## Consequences for the still-open lease decision

These are design implications, not accepted product requirements:

- Keep agent-specific adapters near the agents. Future server-hosted lit receives renewal requests over its normal API; it need not inspect remote PIDs, files, or vendor runtimes.
- Register the claim association explicitly: agent type, conversation ID, observer/runtime scope, and a distinct claimant/run identity. A resumed conversation must not resurrect an expired ownership token.
- Distinguish observed activity, waiting for a person, autonomous waiting, idle, stopped, and unknown/unreachable. Unknown is not proof of either death or continued work.
- A fresh active-state snapshot proves runtime state, not business progress or that this issue is still being worked. Bind observation to a particular claim and choose a separate stall policy.
- Polling can cover a long silent tool if the owning runtime still reports active. A helper that only remembers the last start hook could renew forever after a crash; it needs fresh observation or a bounded fallback.
- Decide whether awaiting human input retains a lease and for how long. Do not inherit that decision accidentally from vendor `active` or `working` enums.
- Bound renewal when the observer, vendor runtime, tunnel, or lit is unavailable. Service-authoritative expiry and fencing remain necessary regardless of helper sophistication.
- Generous explicit renewals are an alternative with fewer integrations, but do not guarantee survival of longer tool calls. A helper and explicit renewal can share one generic server lease API.

Before implementing automatic renewal, a small compatibility probe should check snapshot freshness, runtime loss, long tools, approval waits, resume/fork, and child-agent identity for each supported version. That validation and the lease policy remain open decisions.

# Agent launcher and claim supervisor

Status: deferred idea, not approved for implementation (2026-09-30).

Potential interface: `lit run --session NAME --project REF -- COMMAND...`;
`lit agent` is another name under consideration. An omitted session would be
fresh and printed for reuse. Launch the interactive agent with explicit tracker
context, optionally supervising enrolled claims with a background renewal loop.
This does not require a permanent daemon.

The agent chooses work, acquires claims, records checkpoints and completes or
hands off work. A supervisor could handle setup and renewal timing. It must not
automatically acquire, force takeover, reacquire expired claims or close issues.

Questions to resolve before specification:

- Process liveness does not establish progress. Define renewal while idle or
  waiting for approval, explicit pause, and the duration of authorization.
- Lost ownership must reach the agent and stop new claimed work. A warning on
  stderr alone is insufficient; tracker leases do not fence filesystem writes.
- Reusing one logical session concurrently risks cross-renewal and cleanup.
  Consider one active supervisor per session; subagents need separate identities
  and explicit enrollment, not accidental inherited ownership.
- Main-agent skills must adopt launch context; the existing workflow adapter
  deliberately ignores inherited session/endpoint environment variables.
- Renew exact owner/token pairs. Expiry or reassignment requires agent-led
  reconciliation and rereading, never automatic reacquisition.
- On crashes, allow expiry. Release only when the relevant work and descendants
  are known to have stopped. Keep session reuse distinct from vendor chat resume.

Validation should cover long commands, idle approval, signal/exit handling,
supervisor crashes, concurrent reuse, subagents, dropped replies, and sleep or
network loss followed by another owner acquiring the issue.

This records the discussion only; automatic lease helpers remain deferred.

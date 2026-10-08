# Interrupt and resume implementation

Read this when work spans repositories, stops unfinished, or resumes from a
handoff. A handoff is evidence of a previous observation, not current permission
to work. Use lit's [recovery policy](../../lit/references/recovery.md) for retained
requests and claims; this reference covers the Git and workflow side.

## Preserve an interruption

Stop starting new behavior changes. Inspect each affected repository separately:
its applicable instructions, repository identity/role, actual checkout or worktree,
branch, HEAD, staged and unstaged ticket-owned changes, unrelated changes, and
checks already run. Tracker ownership does not lock files or establish Git
isolation. Keep each repository's test commands and outcomes distinct.

Where hooks permit, commit unfinished ticket-owned work as an explicitly marked
WIP checkpoint with expected failures and the current TDD phase. Respect hooks:
a rejected WIP commit is a failed checkpoint, not authorization to bypass hooks,
change hook configuration, or describe the work as committed. Record its actual
HEAD, staged/unstaged/untracked paths, expected failing checks, and how the next
session can find the uncommitted work. Preserve the checkout. A patch may help
preserve evidence but does not replace untracked files, binary files, or Git
objects unless those were actually captured and verified. Do not sweep unrelated
changes into a checkpoint.

Multi-repository checkpoints are not atomic. Record which commits succeeded and
which failed before posting a combined handoff. Reference unchanged repositories'
existing commits. Preserve cited commits reachable on their working branches;
never amend, rebase, squash, reset away, or prune them merely to tidy the handoff.
An explicitly authorized later rewrite needs an appended old-to-new commit map,
including repository identity and the verification performed on replacements.
Do not silently reinterpret an old hash as a new one.

Store a handoff on the ticket when available, and keep a local copy when updates
are pending. Workflow-owned persisted handoffs belong under the workflow state
directory (explicit `LIT_WORKFLOW_STATE_DIR`, otherwise the host's documented
workflow default), separate from CLI private pending/session files and output
cache. Use Markdown front matter `schema_version: 1`; identify service, project,
issue, and logical client IDs explicitly. This is a prose record, not a second
transaction engine or automatic replay queue. Include:

- Per repository: role/identity, checkout/worktree, branch, base and current full
  commit hashes, successful checkpoints and remaining uncommitted state.
- Actual checks and results, current TDD phase, expected failures, acceptance
  criteria still outstanding, and next action.
- Tracker updates known stored versus not attempted versus uncertain; reference
  exact retained pending files/request hashes without rewriting or copying their
  secret claim tokens into prose. Record evidence for an acknowledged update.
- Ownership last observed and release result; never describe failed or uncertain
  release as successful. Release ownership on orderly handoff when available.

Git hashes identify objects; they do not prove a remote copy or authorize a push.
A hard crash may leave no handoff. Inspect surviving Git state and retained CLI
requests rather than filling gaps with guessed history.

## Resume from current evidence

1. Reestablish the explicitly identified logical session and service. Do not use
   a global latest session, a new client to escape pending requests, or a matching
   project title on a different service. Reuse that logical client across the
   affected repositories while retaining each checkout's identity and isolation.
2. Reconcile uncertain tracker requests read-only before issuing replacements.
   Use `transactions status` with the retained request and executable help.
   `committed` establishes durable history; `already_satisfied` establishes only
   the reported current-state proof. `uncertain` is not permission to retry.
   An `uncommitted` result still requires fresh requirements/revisions/ownership
   before deliberately constructing a new command; never alter and replay the
   retained request. Inspect comments/history and current closure as appropriate.
3. Fetch the current ticket, comments, specification, blockers, and claims. If
   closure already committed, verify closed state and ended ownership; do not
   reacquire or reopen solely to repeat completion. For still-open work, a
   returning session reacquires after release/expiry/loss and rereads after that
   acquisition before changing work. A live competing claim stops implementation;
   report the conflict without force. If lit remains unavailable, preserve existing
   work but do not resume implementation. Reconnection never renews a claim.
4. Check every named repository and Git object against the handoff. Verify the
   actual path, worktree branch, HEAD, status, and cited commits (for example,
   `git cat-file -e HASH^{commit}` and `git show HASH`). Read each repository's
   current instructions. Do not reset a differing checkout to match stale notes.
   An unavailable checkout blocks the affected scope; do not substitute a similarly
   named directory or claim its checks passed. Independent already-authorized work
   may continue only when its prerequisites and isolation are established.
5. Distinguish a missing path from lost work. An existing local worktree, branch,
   or object database may provide a recoverable checkpoint: verify the commit and
   tree before creating a new isolated worktree. Reflog/object inspection can find
   surviving commits without rewriting history. If required objects or uncommitted
   files cannot be found, retain the evidence and report exactly what is missing.
   A hash or comment alone cannot recreate them. Do not invent completion or
   silently recreate a claimed checkpoint; reconstruction is new work that needs
   an explicit scope decision and fresh verification.
6. Re-run relevant checks on the recovered checkout and inspect the diff. Resume
   the recorded TDD phase using current requirements. Keep partial success visible
   until all changed repositories have verified commits and completion evidence
   is stored. If closure already committed, verify state and ended ownership;
   do not reacquire and reopen solely to repeat completion. If only the evidence
   comment committed, avoid duplicating it and finish remaining verification and
   closure under current ownership. Stop after this ticket.

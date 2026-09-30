# Uncertain tracker outcomes

An interrupted request may have committed even when the CLI did not receive its
response. The client preserves pending request evidence under its private
`LIT_STATE_DIR` (or the platform default). This is separate from service data.
`lit transactions status --format json` inspects retained requests and never
replays them. Only an explicit `--session NAME` filters the result; neither
`LIT_SESSION` nor the working directory supplies that filter. `--file PATH`
selects one existing retained request envelope, not a mutation input file.

An `uncertain` result retains evidence and exits 4. Repeating status cannot
always establish an outcome:

- If the response to a first connection is lost before the client identity is
  received, the pending record has no client ID. Status reports
  `registration_identity_not_received` without querying the service for that
  request. There is no initial-scope registration lookup/repair operation.
- A durable request can be proven committed from immutable history. When that
  history evidence is absent and original preconditions no longer hold, such as
  after another edit, the service reports `original_preconditions_no_longer_valid`.
  It cannot conclude that replay is safe merely from current content.
- Disconnected clients and operational effects without durable history can also
  leave uncertainty. Reconnecting or obtaining another claim does not supply
  missing proof about an earlier request.

These are supported conservative outcomes, not promises that retrying status
will eventually clear the record. There is currently no supported command to
retire unresolved evidence, repair a registration identity, or override an
uncertain result. Do not delete/move pending files, create a replacement client,
or replay the operation as a workaround.

For a persistent uncertain outcome, stop dependent mutations and escalate to the
human owner. Preserve the pending file, any returned proof/error, request and
service/client identifiers, repository changes, and an explicit handoff describing
what is confirmed and what remains unknown. Record the intended operation and
whether claims or ticket updates might have partially succeeded. Keep unrelated
work separate from the unresolved operation; do not claim completion or resume
claimed work by assuming it failed.

A human should inspect the preserved evidence and accepted history before
choosing a next step. If those do not prove an outcome, retain the unresolved
state and seek an explicit repair design or maintainer assistance. Human
escalation alone does not authorize evidence deletion, bypass, or replay; the
initial delivery provides no verified manual retirement procedure. The workflow
adapter's `reconcile` only adopts a confirmed existing binding, and does not
repair these cases.

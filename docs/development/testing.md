# Verification

Run the active Go checks from the repository root:

```sh
go test -count=1 -race ./...
go vet ./...
go test -count=1 -v ./integration -run '^TestGoDistributionWalkthrough$'
```

The integration harness uses temporary service data, private client state, and
working directories. The distribution walkthrough installs the public binaries
with `go install`, extracts embedded skills offline, and exercises sessions,
claims, checkpoints, cross-checkout resume, and closure against a temporary service.
It does not invoke paid models or start a persistent service.

Installer and workflow-session regression tests live in
`internal/workflow-skills` and `internal/workflowsession`. Protocol fixtures in
`internal/protocol/testdata` pin canonical request bytes and hashes.

Keep generated logs, agent transcripts, and session snapshots out of version
control. These checks do not establish native agent behavior or macOS/Windows
runtime compatibility.

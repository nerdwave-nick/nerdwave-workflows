# Go behavior changes

Use the repository's Go version, module/workspace boundaries, test commands, and
format/lint conventions. Check go.mod, go.work, and task scripts before inventing
commands. Keep behavior tests at the existing public or package seam; table-driven
subtests help distinguish inputs and failure cases when cases share an invariant.
Do not assert internal call sequences merely to mirror the implementation.

Observe the intended failing subtest before implementation, then rerun it green.
Run affected packages and required broader checks. For concurrency/lifecycle
changes, exercise competing operations, cancellation, ownership, and shutdown as
applicable and run those paths with the race detector (normally `go test -race`).
A passing race run only covers paths actually exercised. Use controlled time or
synchronization where existing seams permit; sleeps alone are weak evidence of
ordering. Follow the repository's formatting and vet/static-analysis requirements.

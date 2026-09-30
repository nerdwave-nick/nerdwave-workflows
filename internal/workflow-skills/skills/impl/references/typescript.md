# TypeScript behavior changes

Use the repository's package manager, lockfile, scripts, test runner, and TS
configuration. Runtime tests and type checking establish different things: run
both applicable runtime tests and the existing typecheck/build command. A type
assertion or successful compilation does not validate data received at runtime.

Test observable behavior at existing boundaries. Where the changed behavior
accepts untrusted JSON, storage, or network input, exercise malformed and missing
values and the established validation/error path. Do not add a schema library or
weaken strictness just for this workflow. Observe the intended runtime failure,
then the passing behavior; use established type-test conventions for changes whose
contract is specifically compile-time. Await asynchronous work and assert its
result or rejection rather than only that a mock was called.

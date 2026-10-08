<!-- BEGIN lit managed -->
# Workflow

These conventions apply to each repository. All paths below are relative to the current repository root, never to the directory containing this global file.

## Issue tracker

When using the tracker, load the installed `lit` skill. Read repository conventions in docs/agents/issue-tracker.md relative to that repository.
Use lit/references/hosts.md for explicit runtime session setup. Hooks are optional.

## Triage labels

Default to `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, and `wontfix`. Repository-specific overrides belong in `docs/agents/triage-labels.md`.

## Domain docs

Default to a single-context layout in each repository: one root `CONTEXT.md` and ADRs in `docs/adr/`. Repository-specific layout and reading rules belong in `docs/agents/domain.md`.
<!-- END lit managed -->

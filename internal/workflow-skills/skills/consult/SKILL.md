---
name: consult
description: Consult an independent Claude or Codex CLI session with approved scope, exact model and effort, and reusable read-only follow-ups. Also supplies the workflow for explicitly requested external challenge.
---

# Consult

Invoke the selected vendor CLI directly. This skill requires no consultation
command, helper, or tracker connection. Read [session and invocation guidance](references/interface.md)
when starting, resuming, or handing off a consultation.

An explicit user request or approval is required before the first invocation,
a fresh consultation, or a provider/model/effort change. A proposal to consult
is not approval. Relevant follow-ups within the approved question need no repeated
permission. Stop when useful or disagreement is understood. Never initiate paid
smoke tests merely to validate this skill.

- `claude` / `fable` selects Claude CLI model `fable`.
- `codex` / `astra` selects Codex CLI model `gpt-6-astra`.
- Pass explicit model and effort values unchanged; effort defaults to high.
  Do not substitute models, lower effort, or start a fresh conversation after a
  failed resume. Unsupported selections are failures to report.

Find the existing discussion notes and reuse a clear matching vendor session by
its explicit ID, including across repositories. Do not use vendor latest/continue
selection. If the provider or session is ambiguous, resolve that ambiguity before
invoking; when questions are disallowed, report the blocked consultation and
continue independent work. Keep one caller responsible for a session and wait for
each turn to finish before sending another. These are agent coordination rules;
this skill does not enforce locks or serialize processes.

First/fresh briefings contain the task, original proposal, requirements,
constraints, relevant artifacts, and observed facts with provenance. Label
assumptions. Exclude your verdict, preferred answer, persuasive summary, and
leading framing. Supply applicable repository rules and absolute artifact paths
or relevant contents so a consultant can work from a neutral directory.

Consultants inspect and advise only. Before every invocation, select the installed
CLI's supported read-only permissions and isolate unrelated integrations, hooks,
plugins and configuration. Verify these choices for resume as well as initial
calls. Stop if the environment cannot honor them; a read-only prompt alone is
insufficient. Never bypass permissions or relax restrictions to make a call work.
Consultants receive no ticket-write, editing, or commit authority.

Record the approved scope, provider, exact model/effort, explicit vendor session
ID, invocation directory, response/transcript location, findings, and unresolved
questions in the existing ticket or standalone discussion notes. Preserve enough
context for another caller to resume. The caller decides how to incorporate
advice; advice does not reverse accepted decisions or authorize implementation.
On interruption or failure, retain evidence and investigate whether a turn ran
before retrying. Never treat a lost response as proof nothing happened.

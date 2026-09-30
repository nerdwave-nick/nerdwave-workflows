---
name: clarify
description: Interview the user to settle a plan or decision, recording accepted answers and warranted domain documentation as the discussion proceeds.
---

Build a shared understanding of the decision, its constraints, and the questions
that depend on it. Read applicable repository instructions and domain docs first.
Use [modeling](../modeling/SKILL.md) by default when terms or relationships change;
it does not require creating a glossary or ADR for every conversation.

Investigate discoverable facts from the actual artifacts rather than asking the
user to supply them. Separate observed behavior, proposals, assumptions, and
accepted decisions. A code/decision mismatch is evidence to discuss, not authority
to overwrite the user's decision. Delegate bounded fact gathering when useful
and supported by the host; pending evidence blocks only dependent questions.

Ask the currently answerable questions in manageable numbered rounds. Explain
tradeoffs and give a reasoned recommendation where evidence supports one. Wait
for the user's decisions before asking questions dependent on those answers.
Recompute the remaining questions after each round; do not demand exhaustive
answers to irrelevant hypothetical branches.

Record accepted answers during the discussion, before moving to the next round.
Use the existing decision record or shared tracker discussion when one is in
scope. For a standalone conversation, use the agreed repository-local discussion
file (or a small Markdown decision note when no destination exists); no running
tracker is required. Capture the answer, rationale, unresolved points, and evidence
links without presenting your recommendation as accepted. If the user requests
read-only discussion, retain a proposed note in the response instead of writing.
Application-owned workflow notes use `schema_version: 1` front matter; do not add
format markers to user-owned prose or existing repository domain documents.

Shared project decisions belong in the tracker when working in that workflow;
repository-specific glossary/ADR changes belong in each owning repository. Read
that repository's rules and resolve its paths from its own root. Reuse the lit
skill's ownership policy for tracked work; do not demand a tracker connection for
an unrelated decision conversation. If an in-scope tracker write fails, preserve
pending notes locally and report the missing update without claiming it succeeded.

At the end, summarize settled decisions and remaining uncertainties and obtain
confirmation of shared understanding. This closes the design discussion; it does
not authorize implementation. Existing implementation authorization remains valid
within its scope. Use [challenge](../challenge/SKILL.md) when deliberate scrutiny
would help, without silently reopening settled choices.

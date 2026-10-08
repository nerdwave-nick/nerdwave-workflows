---
name: modeling
description: Refine domain terms and relationships, recording resolved vocabulary and warranted architectural decisions in the owning repository.
---

Read the owning repository's instructions, `docs/agents/domain.md` when present,
and its existing glossary/decision records. Repository conventions override these
defaults. Resolve paths from that repository root, including in cross-repository
work. Default to root `CONTEXT.md` and `docs/adr/`; follow a context map or another
layout only when local conventions call for it.

Clarify overloaded terms using concrete scenarios. Compare claims about existing
behavior to code where available, separating observed behavior from intended
changes. Surface disagreements with accepted terms and ask about material domain
choices; do not impose this skill's terminology or silently rename local concepts.
Hypothetical edge cases are questions, not evidence of actual defects.

Once a term or relationship is resolved, update the owning glossary during the
conversation. Preserve its established format and unrelated content. If there is
no glossary, create one lazily when there is a resolved domain concept to record;
[CONTEXT-FORMAT.md](CONTEXT-FORMAT.md) offers a fallback. The default glossary
records domain meaning rather than implementation details. Local document purpose
remains authoritative. Read-only scope permits proposed edits only.

Create or update an ADR only for an accepted decision whose real alternatives,
cost of reversal, and otherwise surprising rationale warrant durable explanation.
An unresolved choice stays a proposal. Use the local template and numbering, or
[ADR-FORMAT.md](ADR-FORMAT.md) as a fallback. Preserve prior decision history using
the repository's supersession convention. Do not create empty scaffolding or an
ADR for an obvious, easily reversed choice.

Keep shared project decisions in their existing tracker discussion and local
vocabulary/ADRs with their owning code. A standalone modeling request needs no
tracker. Summarize actual edits and open questions; recording a decision grants
no additional implementation authority.

# Triage briefs

A brief records triage evidence and makes an issue actionable. It is not a competing
specification: the project's shared specification owns common behavior, and the
ticket owns its slice and acceptance criteria. Link those sources. Do not declare
old discussion irrelevant or silently overwrite accepted decisions. A standalone
ticket may contain its entire specification.

Use only the fields needed for this issue:

- Category and requested/agreed disposition.
- Current behavior, reproduction commands/environment, and actual results.
- Desired behavior and applicable shared-spec/decision references.
- Affected repository identities and roles; relevant interfaces and source evidence.
- Concrete acceptance criteria for this slice, consistent with the specification.
- Scope exclusions, blockers, missing access, and unresolved questions.

Prefer durable behavior/interface descriptions over implementation recipes. Concrete
paths, commits, and commands are useful evidence when labeled with their repository
and observed revision; do not treat them as permanent implementation mandates.
Distinguish observed facts from assumptions and proposals. When evidence contradicts
requirements, expose the disagreement for a decision instead of creating a new
contract in a comment. A quick explicit label override does not require a brief.

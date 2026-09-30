---
name: to-tickets
description: Turn requirements or an agreed specification into reviewed, verifiable implementation tickets and real blocking links in lit.
disable-model-invocation: true
---

# To tickets

Accept a plan, specification, existing discussion, or issue directly. No planner
prerequisite is needed. Read [lit guidance](../lit/SKILL.md), repository tracker and
triage conventions, and each affected repository's instructions, CONTEXT.md, and
relevant ADRs. Resolve repository-relative references against their owning repo.
Use tracker context and grep to discover earlier decisions and overlapping work,
then read the current full records and comments, including continuation pages.

Identify the project's shared specification and accepted decisions. It owns common
behavior; each ticket owns its slice and acceptance criteria. A standalone ticket
may include its complete specification. Flag contradictions instead of silently
changing shared requirements. Record repository identities and roles on the project;
keep checkout paths in client-local mappings. Missing access blocks affected slices,
not independent drafting.

Draft narrow, complete, verifiable slices. A ticket may span repositories when that
is the smallest coherent behavior change; do not split mechanically by repository.
Each ticket should fit a fresh working context and be demonstrable on its own.
Prefactor only when needed. For mechanical changes that cannot land independently,
use expand–migrate–contract or name a shared integration branch and explicit final
verification gate; explain where intermediate green checks are possible.

Present a new breakdown for human review before publication. Show each title,
delivery, acceptance criteria, affected repositories, and true prerequisite tickets.
Parentage and related context do not imply a blocker. Review granularity and edges;
await approval where the breakdown is new. Explicit direction to publish an already
agreed breakdown is sufficient: do not ask again. Carry prior authorization forward
within scope, and return changed material choices for review.

Publish the approved breakdown through installed lit operations:

1. Create a separate issue for each slice, recording returned stable IDs. Use the
   designated shared-spec reference, repository roles, expected outcome, testable
   acceptance criteria, scope boundaries, and relevant accepted contracts. Apply
   ready-for-agent unless the user's direction or repository conventions differ.
   Mutation output can include changed parent/project records: identify the newly
   created issue by its returned type/identity and fetch it before using its ID;
   never assume the first result item is the new ticket.
2. After issues exist, wire native `blocked-by` relationships using their real IDs
   and executable help. Body references alone do not establish the frontier. Add
   parentage only within one project; cross-project context uses related/blocking
   links. Do not confuse hierarchy with dependencies.
3. Fetch the issues and relationships to verify the published graph and criteria.
   Report the stable references, available frontier, and any partial publication.
   Reconcile uncertain outcomes before retrying; do not blindly duplicate tickets.

Preserve the parent's existing status and content. Creating or finishing children
never completes the parent implicitly. Publication does not start implementation;
hand off to impl only when the user has authorized execution.

# ADR 0011: Compensation Applies Only To Completed Nodes

- `Status`: `Accepted`
- `Date`: `2026-03-26`
- `Deciders`: `Project maintainers`
- `Related docs`: `docs/V1_CANONICAL_BASELINE.md`, `docs/UAT_MATRIX.md`, `docs/SPRINT_4_PLAN.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

Compensation is one of the most semantically dangerous features in a workflow engine.

If the system attempts to compensate nodes that:

- never completed
- were skipped
- were cancelled before completion
- timed out before completing their intended work

then compensation becomes ambiguous and can actively make system behavior less trustworthy.

We need one clear rule for what enters the compensation set.

---

## Decision

In Grael v1, compensation applies only to nodes that reached `COMPLETED`.

That means:

- only completed nodes may enter the compensation stack
- failed, skipped, cancelled, or not-yet-completed nodes do not enter the compensation stack
- compensation ordering is based on the reverse order of completed compensable work

---

## Alternatives Considered

### Allow partially completed or in-flight nodes into compensation heuristically

Rejected because heuristic compensation is not trustworthy enough for v1.

### Allow cancellation or timeout to imply compensability automatically

Rejected because those states do not guarantee that the forward action completed in a compensable way.

### Skip compensation entirely in v1

Rejected because basic saga-style unwind is part of the intended v1 reliability story.

---

## Consequences

Benefits:

- clearer compensation semantics
- lower risk of compensating work that never truly completed
- more honest operator expectations

Tradeoffs:

- some partial side effects remain outside guaranteed compensation
- compensation remains intentionally conservative

---

## Guardrails

The following rules now follow from this decision:

- do not push non-completed nodes into the compensation stack
- do not treat timeout, skip, or cancellation alone as proof of compensable completion
- do not widen compensation semantics through convenience shortcuts

---

## Validation

This decision is holding if:

- [UAT-C7-02-failure-triggers-compensation.md](../uat/UAT-C7-02-failure-triggers-compensation.md) passes
- [UAT-C7-03-compensation-resumes-after-restart.md](../uat/UAT-C7-03-compensation-resumes-after-restart.md) remains explainable through persisted completed-work history only

---

## Notes

This ADR is intentionally conservative.
Conservative compensation is better than misleading compensation.

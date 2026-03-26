# ADR 0010: Spawn Validation Must Happen Before Graph Mutation

- `Status`: `Accepted`
- `Date`: `2026-03-26`
- `Deciders`: `Project maintainers`
- `Related docs`: `docs/V1_CANONICAL_BASELINE.md`, `docs/UAT_MATRIX.md`, `docs/SPRINT_3_PLAN.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

The living DAG behavior in Grael allows workers to introduce new nodes at runtime.

This is powerful, but it also creates one of the highest-risk correctness boundaries in the system.

If invalid spawn definitions are accepted first and validated later, Grael can record:

- cycles
- missing dependency references
- inconsistent graph state

Once such graph state is durably recorded, recovery becomes harder and correctness becomes ambiguous.

We need to define the ordering of validation relative to graph mutation.

---

## Decision

In Grael, spawn validation must happen before graph mutation becomes active.

That means:

- spawned node definitions are validated before they are accepted into active graph state
- cycle-producing or otherwise invalid spawn submissions are rejected before they can corrupt the run graph

Validation is therefore a gate on mutation, not a cleanup step after mutation.

---

## Alternatives Considered

### Record spawn first, validate later

Rejected because it allows invalid graph state to enter durable history before the system knows whether it is safe.

### Accept invalid spawn and rely on later recovery logic

Rejected because recovery should not be a substitute for mutation safety.

### Limit spawn expressiveness instead of validating

Rejected because validation is the correct answer to dynamic structure, not arbitrary restriction alone.

---

## Consequences

Benefits:

- safer graph mutation semantics
- lower risk of unrecoverable or confusing graph corruption
- clearer failure behavior at the mutation boundary

Tradeoffs:

- spawn handling path becomes more explicit
- invalid spawn attempts fail earlier and more visibly

---

## Guardrails

The following rules now follow from this decision:

- do not let spawned nodes enter active graph state before validation finishes
- do not rely on later repair logic for invalid graph mutations
- do not weaken cycle detection into a warning-only mechanism

---

## Validation

This decision is holding if:

- [UAT-C6-03-cycle-spawn-rejected.md](../uat/UAT-C6-03-cycle-spawn-rejected.md) passes
- invalid spawn attempts fail safely without creating active cyclical graph state

---

## Notes

This ADR protects one of the most important correctness boundaries in Grael.

# ADR 0006: Timed Out Is Not A Separate V1 Node State

- `Status`: `Accepted`
- `Date`: `2026-03-26`
- `Deciders`: `Project maintainers`
- `Related docs`: `docs/V1_CANONICAL_BASELINE.md`, `docs/V1_SCOPE.md`, `docs/UAT_MATRIX.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

One design option is to model timeout as its own terminal node state, such as `TIMED_OUT`.

That approach can improve observability, but it also adds:

- extra node-state complexity
- more transitions to reason about
- more implementation branching in v1

For Grael v1, we need to decide whether timeout is a distinct node state or a failure reason within the simplified state model.

---

## Decision

In Grael v1, timeout will not be a separate node state.

Instead:

- timeout is represented through failure semantics
- nodes use the v1 state set:
  - `PENDING`
  - `READY`
  - `RUNNING`
  - `AWAITING_APPROVAL`
  - `COMPLETED`
  - `FAILED`
  - `SKIPPED`
- timeout-specific meaning is carried through failure reason and event semantics, not through a separate terminal state

This keeps the v1 state machine smaller while preserving externally visible timeout behavior.

---

## Alternatives Considered

### Keep `TIMED_OUT` as a separate node state in v1

Rejected because it adds real implementation and reasoning complexity without being necessary for the first trustworthy v1 product.

### Hide timeout entirely as an internal implementation detail

Rejected because operators still need visible timeout semantics, even if timeout is not a separate state.

### Delay timeout handling until later

Rejected because deadline enforcement is part of the v1 trust contract.

---

## Consequences

Benefits:

- smaller v1 node state machine
- less branching in scheduler and state application
- timeout still remains externally meaningful through failure semantics

Tradeoffs:

- some observability nuance is carried in failure reasons rather than terminal state names
- future versions may revisit the state model if a distinct timeout state becomes worth the complexity

---

## Guardrails

The following rules now follow from this decision:

- do not add `TIMED_OUT` as a v1 node state without an explicit superseding decision
- do not make timeout invisible just because it is modeled as failure semantics
- timeout behavior must still remain externally observable through event and API surfaces

---

## Validation

This decision is holding if:

- [UAT-C5-03-execution-deadline-timeout.md](../uat/UAT-C5-03-execution-deadline-timeout.md)
- [UAT-C5-04-absolute-deadline-during-approval.md](../uat/UAT-C5-04-absolute-deadline-during-approval.md)
- [UAT-C8-03-checkpoint-timeout.md](../uat/UAT-C8-03-checkpoint-timeout.md)

all remain satisfiable without introducing a separate `TIMED_OUT` node state into the v1 runtime model.

---

## Notes

This ADR is explicitly about v1.
It does not forbid a future reconsideration if observability value clearly outweighs complexity in a later version.

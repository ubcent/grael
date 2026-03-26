# ADR 0009: Checkpoints Must Not Block Unrelated Work

- `Status`: `Accepted`
- `Date`: `2026-03-26`
- `Deciders`: `Project maintainers`
- `Related docs`: `docs/V1_CANONICAL_BASELINE.md`, `docs/UAT_MATRIX.md`, `docs/SPRINT_4_PLAN.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

Checkpoint approval is one of Grael's important operator-facing features.

The naive implementation risk is that once one node requests approval:

- the whole run stalls
- unrelated runnable work stops progressing
- approval becomes a global pause instead of a local gate

That would significantly weaken the product value and make checkpoints feel expensive and intrusive.

We need to decide whether approval waiting is local or global.

---

## Decision

In Grael v1, checkpoints block only the node that requested approval and the work that depends on that node.

They must not block unrelated runnable work in the same run.

Checkpoint waiting is therefore a local graph-control mechanism, not a global execution freeze.

---

## Alternatives Considered

### Pause the entire run during approval

Rejected because it turns checkpoints into a blunt instrument and weakens one of Grael's most visible differentiators.

### Allow workers to decide whether approval blocks globally or locally

Rejected because it makes checkpoint semantics inconsistent and harder to trust.

### Avoid checkpoints in v1 entirely

Rejected because selective approval is part of the intended v1 operator story.

---

## Consequences

Benefits:

- stronger product behavior during human-in-the-loop pauses
- better throughput in mixed automated/manual flows
- clearer graph semantics

Tradeoffs:

- node readiness and scheduling around checkpoints require more careful implementation
- timeout and deadline handling must remain precise while unrelated work continues

---

## Guardrails

The following rules now follow from this decision:

- do not freeze the whole run when one node enters `AWAITING_APPROVAL`
- do not couple unrelated runnable work to checkpoint wait state
- ensure that only dependency-related work is blocked by a waiting node

---

## Validation

This decision is holding if:

- [UAT-C8-01-checkpoint-pauses-one-node.md](../uat/UAT-C8-01-checkpoint-pauses-one-node.md) passes
- `GetRun` makes it visible that one node is waiting while unrelated nodes continue to progress

---

## Notes

This ADR captures a product behavior that users should feel immediately in demos.

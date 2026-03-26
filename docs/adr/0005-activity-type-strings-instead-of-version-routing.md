# ADR 0005: Activity Type Strings Instead Of Version Routing In V1

- `Status`: `Accepted`
- `Date`: `2026-03-26`
- `Deciders`: `Project maintainers`
- `Related docs`: `docs/V1_CANONICAL_BASELINE.md`, `docs/V1_SCOPE.md`, `docs/SPRINT_5_PLAN.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

A richer worker-routing model could include capability version ranges and version-aware dispatch logic.

That approach can be useful, but it creates additional concerns:

- version parsing and comparison
- compatibility negotiation
- queueing behavior when no compatible worker exists
- wider worker registration semantics

For v1, we need to decide whether worker routing includes version semantics or stays minimal.

---

## Decision

Grael v1 will route work by activity type string only.

The v1 rule is:

- workers register activity type strings
- tasks target activity type strings
- breaking changes require a new activity type name

Grael v1 will not implement capability version ranges or version-aware worker routing.

---

## Alternatives Considered

### Add capability version ranges in v1

Rejected because the routing complexity is real, and the product does not need it to prove the core value of Grael v1.

### Infer compatibility dynamically from worker metadata

Rejected because it creates ambiguous contracts and hidden routing rules.

### Keep version fields but ignore them operationally

Rejected because fake flexibility is worse than a small honest contract.

---

## Consequences

Benefits:

- simpler worker registration model
- simpler dispatch semantics
- lower risk in early protocol implementation

Tradeoffs:

- activity breaking changes must be expressed through naming discipline
- richer compatibility management is postponed to a later version

---

## Guardrails

The following rules now follow from this decision:

- do not add version-range routing logic to v1 worker matching
- do not widen worker registration to imply compatibility semantics the engine does not enforce
- if a worker behavior changes incompatibly, introduce a new activity type name instead of soft-routing around it

---

## Validation

This decision is holding if:

- [UAT-C4-01-worker-success.md](../uat/UAT-C4-01-worker-success.md) and [UAT-C10-02-go-sdk-worker-seam.md](../uat/UAT-C10-02-go-sdk-worker-seam.md) remain explainable through activity-type-only routing
- worker registration and task dispatch stay small and honest in v1 implementation

---

## Notes

This ADR protects v1 from premature platform complexity.

# ADR 0007: GetRun And ListEvents Are The Only V1 Read Surfaces

- `Status`: `Accepted`
- `Date`: `2026-03-26`
- `Deciders`: `Project maintainers`
- `Related docs`: `docs/V1_CANONICAL_BASELINE.md`, `docs/UAT_MATRIX.md`, `docs/V1_TASK_BACKLOG.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

There are many possible ways to present workflow state:

- compact projections
- operational projections
- forensic projections
- specialized dashboards
- derived status APIs with different collapse rules

Those can be useful, but they also widen the surface area and increase the risk of semantic drift between what the engine knows and what operators are shown.

For v1, we need to decide how much read surface to support.

---

## Decision

Grael v1 will expose two primary read surfaces only:

- `GetRun` for current derived run state
- `ListEvents` for raw recorded execution history

Grael v1 will not introduce multiple projection modes or specialized read models beyond these two surfaces.

This is the v1 read contract.

---

## Alternatives Considered

### Add multiple projection modes in v1

Rejected because it expands surface area and makes it easier to drift away from one coherent source-of-truth model.

### Build a richer dashboard-oriented projection system first

Rejected because it optimizes presentation before the engine contract is fully proven.

### Expose only `GetRun` and hide raw event history

Rejected because Grael's auditability and forensic value depend on raw event visibility.

---

## Consequences

Benefits:

- smaller and more honest v1 API surface
- stronger alignment between source of truth and what operators see
- lower risk of projection drift

Tradeoffs:

- some richer read UX is postponed
- clients may need to do a bit more work if they want specialized presentation views

---

## Guardrails

The following rules now follow from this decision:

- do not add alternate projection modes to v1 without a superseding decision
- do not make any projection primary state
- do not hide raw execution history behind aggregated-only views
- keep `GetRun` derived and `ListEvents` raw

---

## Validation

This decision is holding if:

- [UAT-C9-01-get-run-coherent-view.md](../uat/UAT-C9-01-get-run-coherent-view.md) passes
- [UAT-C9-02-list-events-causal-history.md](../uat/UAT-C9-02-list-events-causal-history.md) passes
- operators can inspect both current state and causal history without additional read models

---

## Notes

This ADR is not anti-observability.
It is pro-honesty and anti-premature read-model complexity in v1.

# ADR 0002: Scheduler Must Remain Pure

- `Status`: `Accepted`
- `Date`: `2026-03-26`
- `Deciders`: `Project maintainers`
- `Related docs`: `docs/V1_CANONICAL_BASELINE.md`, `docs/V1_TASK_BACKLOG.md`, `AGENTS.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

Grael's orchestration layer determines what the engine should do next based on derived execution state.

If scheduler behavior depends on:

- direct I/O
- wall-clock reads
- hidden caches
- goroutine timing
- random process-local state

then recovery, replay, testing, and reasoning about the engine become unreliable.

For Grael, this is not just an implementation quality issue. It cuts directly into the product promise of determinism, auditability, and crash-safe continuation.

We need to make an explicit architectural decision about the scheduler boundary.

---

## Decision

The Grael scheduler must remain pure.

That means:

- it accepts derived execution state as input
- it produces commands as output
- it performs no I/O
- it reads no wall-clock time
- it depends on no hidden mutable state

In concrete terms, the scheduler contract is:

`Scheduler.Decide(state) -> []Command`

All side effects belong outside the scheduler, in components that execute commands and persist resulting events.

---

## Alternatives Considered

### Let the scheduler perform direct I/O for convenience

Rejected because convenience at the decision layer destroys clear ownership and makes orchestration behavior harder to reason about under failure.

### Allow wall-clock access inside the scheduler for timeout checks

Rejected because time-dependent transitions must remain event-driven and durable. Direct time reads would make behavior process-dependent and weaken restart correctness.

### Let the scheduler consult shared in-memory caches

Rejected because hidden state would create non-reconstructable behavior and break deterministic reasoning.

---

## Consequences

Benefits:

- scheduler behavior is easy to test
- orchestration remains reconstructable from persisted state
- runtime ownership boundaries stay sharp
- restart behavior stays honest

Tradeoffs:

- some logic that feels convenient to place in the scheduler must live elsewhere
- command processing becomes more explicit and slightly more verbose

---

## Guardrails

The following rules now follow from this decision:

- do not call network, file, or worker APIs from scheduler code
- do not call wall-clock time functions from scheduler code
- do not read global mutable state from scheduler code
- do not sneak side effects into helper functions called by the scheduler
- if logic requires I/O to decide what happens next, it belongs before state derivation or after command creation, not inside the scheduler

---

## Validation

This decision is holding if:

- [UAT-C2-02-dependency-unblocking-from-recorded-history.md](../uat/UAT-C2-02-dependency-unblocking-from-recorded-history.md) remains explainable through recorded state
- [UAT-C3-01-linear-run-loop.md](../uat/UAT-C3-01-linear-run-loop.md) progresses through state and command transitions without hidden orchestration side effects
- scheduler-focused tests can be written against state input alone

---

## Notes

Purity here does not mean "small."
It means "deterministic and side-effect free."

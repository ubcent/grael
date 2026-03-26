# ADR 0003: Time Enters Only Through Persisted Events

- `Status`: `Accepted`
- `Date`: `2026-03-26`
- `Deciders`: `Project maintainers`
- `Related docs`: `docs/V1_CANONICAL_BASELINE.md`, `docs/UAT_MATRIX.md`, `AGENTS.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

Grael has multiple forms of time-sensitive behavior:

- retry backoff
- lease expiry
- execution deadlines
- absolute deadlines
- checkpoint timeout

If time enters the system through ad hoc checks such as direct clock reads in orchestration logic, then behavior becomes:

- process-local
- restart-fragile
- harder to test
- harder to audit

We need one clear answer to the question: how is time allowed to affect workflow state?

---

## Decision

In Grael, wall-clock time may affect workflow state only through persisted events written by designated time-owning components.

In practice, this means:

- timers are scheduled and fired through persisted timer events
- lease expiry is represented through persisted lease-expiry events
- scheduler logic reacts to persisted time-related events rather than observing time directly

No other component is allowed to create time-driven orchestration state transitions by directly consulting the clock and mutating state.

---

## Alternatives Considered

### Let each component read time directly if convenient

Rejected because convenience here creates inconsistent semantics and undermines restart correctness.

### Use in-memory timers without persisted timer state

Rejected because timer intent and timer firing would disappear across crash and restart boundaries.

### Let the scheduler inspect deadlines by comparing state to `now`

Rejected because this would make time-driven transitions non-durable and non-reconstructable.

---

## Consequences

Benefits:

- time-dependent behavior survives restart
- timeout and retry paths are externally visible in event history
- timer-driven semantics are easier to test and audit

Tradeoffs:

- more explicit timer plumbing is required
- timer and lease components become semantically important parts of the architecture

---

## Guardrails

The following rules now follow from this decision:

- do not read wall-clock time inside scheduler logic
- do not implement correctness-sensitive timers as in-memory-only behavior
- do not bypass persisted event flow for retry, timeout, or expiry behavior
- if a time-based transition matters to correctness, it must be represented in durable history

---

## Validation

This decision is holding if:

- [UAT-C5-01-retry-backoff-success.md](../uat/UAT-C5-01-retry-backoff-success.md) is explainable through persisted retry timing
- [UAT-C5-02-overdue-retry-after-restart.md](../uat/UAT-C5-02-overdue-retry-after-restart.md) survives restart correctly
- [UAT-C5-03-execution-deadline-timeout.md](../uat/UAT-C5-03-execution-deadline-timeout.md) and [UAT-C5-04-absolute-deadline-during-approval.md](../uat/UAT-C5-04-absolute-deadline-during-approval.md) do not depend on process uptime luck

---

## Notes

This ADR is one of the core honesty rules of the system.
If time affects state without leaving a durable trail, the product promise weakens immediately.

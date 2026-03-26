# ADR 0004: No Admission Queue In V1

- `Status`: `Accepted`
- `Date`: `2026-03-26`
- `Deciders`: `Project maintainers`
- `Related docs`: `docs/V1_CANONICAL_BASELINE.md`, `docs/V1_SCOPE.md`, `docs/V1_TASK_BACKLOG.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

One possible product behavior for overload is an admission queue:

- run creation succeeds
- the run sits in a queued state
- execution begins later when capacity is available

This is a real feature, but it introduces additional semantics:

- new run states
- queue timeout behavior
- admission ordering
- operator expectations around delayed start

For Grael v1, we need to decide whether that complexity is part of the product or explicitly excluded.

---

## Decision

Grael v1 will not implement an admission queue.

If the engine is at hard capacity:

- `StartRun` rejects immediately
- no hidden queued run is created
- no delayed automatic start is implied

This is the honest v1 overload contract.

---

## Alternatives Considered

### Add an admission queue in v1

Rejected because it adds meaningful product semantics and implementation complexity that are not on the critical path to a trustworthy v1 engine.

### Silently queue internally without exposing a formal queued state

Rejected because hidden queuing is worse than explicit queuing. It weakens operator trust and makes system behavior harder to reason about.

### Accept every run and rely on eventual drain

Rejected because it hides overload instead of expressing it honestly.

---

## Consequences

Benefits:

- simpler and more honest product contract
- less overload-related state complexity
- easier operational reasoning in v1

Tradeoffs:

- clients must retry explicitly if they want admission behavior
- v1 may feel less smooth under load than a fuller platform product

---

## Guardrails

The following rules now follow from this decision:

- do not introduce queued-run semantics into v1 APIs or state without a new explicit decision
- do not create hidden internal queues that contradict immediate-reject behavior
- do not make `StartRun` appear successful if execution has not actually been admitted

---

## Validation

This decision is holding if:

- [UAT-C9-03-start-run-capacity-reject.md](../uat/UAT-C9-03-start-run-capacity-reject.md) passes
- no runtime path creates a hidden queued run at hard capacity
- operator expectations under overload remain clear and explicit

---

## Notes

This is a deliberate product cut, not an oversight.
It keeps v1 smaller and more honest.

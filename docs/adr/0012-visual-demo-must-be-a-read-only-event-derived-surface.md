# ADR 0012: Visual Demo Must Be A Read-Only Event-Derived Surface

- `Status`: `Accepted`
- `Date`: `2026-03-27`
- `Deciders`: `Grael core team`
- `Related docs`: `docs/V1_CLOSEOUT.md`, `docs/POST_V1_CAPABILITY_MAP.md`, `docs/POST_V1_TASK_BACKLOG.md`, `docs/SPRINT_6_PLAN.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

Grael v1 is complete at the committed runtime scope, but the product now needs a flagship visual demo that shows why the engine is special.

The strongest demonstration path is a live UI that shows:

- the workflow graph as it grows
- node states changing over time
- the causal event history beside the graph
- retry, approval, restart, and completion becoming visible as one coherent story

The main design tension is that a visual demo can easily become a second runtime with its own hidden state, transport shortcuts, or demo-only semantics. That would weaken the same operator trust that the core engine worked hard to earn.

The decision therefore needs to lock:

- where the visual demo lives
- what data it is allowed to trust
- what it must not introduce into the runtime contract

---

## Decision

We will build the visual demo as a separate read-only surface adjacent to the engine, not inside the engine core.

The visual demo must:

- derive its model from `GetRun` and `ListEvents`
- treat persisted event history and read APIs as the only source of truth
- begin with polling-based refresh rather than requiring a new correctness-critical realtime transport
- live in the same repository as a demo application, not as engine-core code

The visual demo must not:

- introduce authoritative state outside the engine event log
- depend on hidden hooks or demo-only runtime shortcuts
- widen the core runtime contract just to make the UI easier
- become a control plane before it proves itself as a read surface

---

## Alternatives Considered

### 1. Build the UI directly into the engine binary as product-core behavior

This would have made packaging simple.

It was rejected because it would mix demo UX concerns with core execution concerns and create pressure to widen the runtime surface for presentation convenience.

### 2. Build the visual demo on a separate backend-specific projection database first

This would have given more flexibility for animation and querying.

It was rejected because the first demo must prove that Grael is legible directly from its honest read surfaces, not from a second interpretation layer that could drift from the runtime.

### 3. Require WebSocket or SSE push before building the first demo

This would have made the UI feel more realtime immediately.

It was rejected because push transport is not necessary to prove the product story. Polling from stable read APIs is enough for the first honest slice and keeps scope smaller.

---

## Consequences

Benefits:

- preserves engine honesty and determinism boundaries
- keeps the demo tightly aligned with operator-visible truth
- allows a compelling product story without mutating core runtime semantics
- reduces scope by making polling sufficient for the first slice

Costs and tradeoffs:

- the first demo may feel slightly less fluid than a push-based experience
- a UI-friendly read adapter may still be needed to simplify graph rendering
- later, if a richer transport is added, it must still remain non-authoritative

---

## Guardrails

- the visual demo must never become an authoritative state source
- demo rendering must be reproducible from persisted events and read APIs
- any demo-specific backend adapter must remain a read-only translation layer
- no demo requirement may weaken runtime guarantees or introduce hidden engine behavior
- review should ask: "Would the demo still be correct after restart if all we had were persisted events and public reads?"

---

## Validation

This decision is holding if:

- the post-v1 visual demo UATs pass from normal read surfaces
- Sprint 6 tasks can be implemented without new correctness-sensitive runtime semantics
- the flagship `core-demo` workflow becomes visibly understandable through the demo UI

Relevant next-step validation docs:

- `UAT-C12-01`
- `UAT-C12-02`
- `UAT-C12-03`
- Sprint 6 tasks `T36` to `T40`

---

## Notes

This ADR is intentionally post-v1. It does not change the completed v1 runtime contract. It defines how the next product-facing demo layer should be built without eroding v1 trust guarantees.

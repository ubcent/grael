# ADR 0013: gRPC Transport Must Remain A Thin Layer Over `api.Service`

- `Status`: `Accepted`
- `Date`: `2026-03-27`
- `Deciders`: `Grael core team`
- `Related docs`: `docs/V1_SCOPE.md`, `docs/V1_CANONICAL_BASELINE.md`, `docs/V1_CAPABILITY_MAP.md`, `docs/SPRINT_7_PLAN.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

Grael v1 has an in-process `api.Service` surface over the runtime engine, but the intended product shape has always included workers connecting over gRPC.

The next v1 slice needs to make the existing orchestration and worker surfaces reachable over the network so:

- TypeScript workers can poll and complete tasks remotely
- a separate API server can start runs and observe progress remotely
- the visual and operator-facing story can consume live committed events without inventing a second source of truth

The main risk is that transport work can accidentally become a second runtime:

- by duplicating orchestration logic in the server
- by translating state too aggressively and drifting from runtime semantics
- by inventing a streaming or subscription layer that is not tied to committed WAL order

For a correctness-sensitive workflow engine, that would weaken operator trust instead of strengthening the product.

---

## Decision

We will implement Grael's v1 gRPC layer as a thin transport over `api.Service`, not as a second orchestration surface.

The gRPC layer must:

- delegate all business behavior to `api.Service`
- map proto request and response types to existing runtime types with minimal transformation
- expose the worker protocol, orchestration calls, and read calls over the network without changing their semantics
- emit `StreamEvents` only from committed events that already exist in the WAL-backed execution flow

The gRPC layer must not:

- contain scheduler logic, orchestration policy, or retry behavior
- become a second source of truth for run state
- reorder or synthesize event history that is not already committed
- widen v1 scope into auth, TLS, RBAC, multi-tenancy, or cloud-control-plane concerns

For v1, local plaintext gRPC is sufficient.

---

## Alternatives Considered

### 1. Move the public API boundary from `api.Service` down into `engine.Engine`

This would have removed one layer.

It was rejected because `api.Service` already defines the intended Go-facing boundary, and bypassing it would create unnecessary churn in a correctness-sensitive area with little product value.

### 2. Build a richer gateway that normalizes or projects data specifically for remote clients

This could have made TypeScript and UI integration easier initially.

It was rejected because projection logic belongs in read-side consumers unless it is already part of the honest public runtime surface. The first transport slice should preserve, not reinterpret, Grael semantics.

### 3. Stream events from an in-memory bus that is not tied to committed WAL append order

This could have simplified implementation.

It was rejected because live visibility must remain subordinate to committed event history. A stream that can outpace or diverge from persisted history is not trustworthy enough for Grael.

---

## Consequences

Benefits:

- preserves the semantic center of the system in one place
- keeps transport work reviewable as mapping rather than runtime behavior
- gives remote workers and remote orchestration clients the same contract as local callers
- keeps `StreamEvents` honest by tying it to committed history

Costs and tradeoffs:

- transport mapping code will be somewhat mechanical and repetitive
- event streaming must be designed carefully enough to preserve committed order
- some payloads will require explicit conversion for durations, structs, and event payload transport

---

## Guardrails

- `api.Service` remains the business boundary
- committed event order remains the truth for `StreamEvents`
- transport code may translate types but may not reinterpret semantics
- transport work must not weaken stale-result rejection, lease semantics, retry semantics, or restart semantics
- local no-auth gRPC is acceptable in v1; auth and TLS are explicitly out of scope for this slice

---

## Validation

This decision is holding if:

- remote orchestration calls behave the same as in-process service calls
- a networked worker can register, poll, heartbeat, and complete tasks without semantic drift
- `StreamEvents` can replay from `from_seq` and then emit new committed events in order
- reviewers can still point to one semantic implementation path inside `api.Service` and `engine.Engine`

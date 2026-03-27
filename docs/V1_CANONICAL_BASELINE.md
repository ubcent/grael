# Grael v1 Canonical Baseline

This document defines the practical source of truth for Grael v1 planning.

It exists because the current repository contains a broad runtime specification and a narrower v1 scope definition. For v1 planning and backlog generation, this document resolves that tension by defining what Grael v1 keeps, simplifies, and cuts.

This is a planning baseline, not a replacement for runtime semantics. Its purpose is to constrain implementation work to the realistic v1 product.

---

## Source Priority

For v1 planning, documents should be interpreted in this order:

1. `docs/V1_SCOPE.md`
2. `docs/ARCHITECTURE_CORRECTIONS.md` for semantic corrections
3. `docs/GRAEL_RUNTIME_SPEC.md` for reusable details that still fit v1
4. `docs/ARCHITECTURE.md` and `docs/ARCHITECTURE_ADDENDUM.md` as background context

If `docs/GRAEL_RUNTIME_SPEC.md` conflicts with `docs/V1_SCOPE.md`, v1 planning follows `docs/V1_SCOPE.md`.

---

## Product Definition

Grael v1 is a single-binary durable execution engine for long-running, branching AI agent pipelines where the graph can grow at runtime.

The v1 product promise is:

- append-only WAL with crash recovery
- deterministic orchestration logic
- worker-based execution over gRPC
- orchestration and inspection access over gRPC
- persisted retries, leases, and timers
- dynamic node spawn at runtime
- human approval on specific nodes without blocking the whole run
- graceful cancellation
- basic sequential compensation
- minimal read APIs for current state and raw event history

Grael v1 is not:

- a generic workflow platform
- a multi-tenant control plane
- a memory system
- a workflow migration platform
- a full-blown policy engine

---

## Keep, Simplify, Cut

| Area | Decision | v1 Canonical Position |
|---|---|---|
| WAL and rehydration | Keep | Append-only WAL per run, CRC32, scan/replay, snapshot + delta recovery |
| Scheduler purity | Keep | `Scheduler.Decide(state) -> []Command` must stay pure and deterministic |
| Command/event split | Keep | Commands are ephemeral; state changes only through persisted events |
| Node state machine | Simplify | Use 7 node states: `PENDING`, `READY`, `RUNNING`, `AWAITING_APPROVAL`, `COMPLETED`, `FAILED`, `SKIPPED` |
| Timeout modeling | Simplify | Do not keep `TIMED_OUT` as a separate node state in v1; model timeout as `FAILED` with timeout reason |
| Run state machine | Simplify | Keep only states needed for normal execution, cancellation, compensation, and terminal outcomes |
| Worker protocol | Simplify | Public v1 worker surface is `PollTask`, `CompleteTask`, `FailTask`, `Heartbeat` |
| Lease model | Keep | Lease grant and lease expiry are required for correctness |
| Lease renewal | Open decision | Renewal semantics are needed, but not yet locked as a required public v1 RPC |
| Heartbeats | Keep | Worker heartbeat timeout should bulk-expire active leases |
| Retries and backoff | Keep | Retry policy and persisted backoff timers are core |
| Deadlines | Keep | v1 keeps both execution deadline and absolute deadline |
| Living DAG spawn | Keep | Completed nodes may spawn new nodes at runtime |
| Cycle detection | Keep | Mandatory at spawn time |
| Guardrails | Simplify | Use soft warnings and hard caps, not a full policy system |
| Graceful cancellation | Keep | This is the only v1 cancellation mode |
| Revoke-and-abandon | Cut | Move to v1.1+ |
| Compensation | Keep | Sequential, basic saga-style compensation only |
| Compensation on cancel | Keep | Optional config, but within v1 |
| Checkpoints | Keep | Included in late v1; minimal approval flow only |
| Memory layer | Cut | Entirely out of v1 |
| Memory refresh modes | Cut | Entirely out of v1 |
| Sub-workflows | Cut | Entirely out of v1 |
| Activity version ranges | Cut | Use activity type strings only |
| Error handler branch | Cut | No `HANDLING_ERROR`, no handler event family |
| Admission queue | Cut | `StartRun` either succeeds immediately or rejects immediately |
| Projection modes | Cut | Replace with `GetRun` and `ListEvents` |
| Workflow version migration | Cut | No migration API in v1 |
| Workflow pinning | Simplify | Record definition hash at start time |
| External event ingestion | Simplify | If implemented in v1, keep only simple dedup semantics |
| Security model | Cut | No mTLS, no RBAC, no multi-tenancy; optional shared secret only |
| Conformance matrix artifact | Cut | Express invariants as tests, not as a separate deliverable |

---

## Canonical v1 Surface

The following capabilities define the actual Grael v1 implementation surface:

### Storage and Recovery

- WAL append
- WAL scan
- CRC32 corruption detection
- snapshot write and load
- rehydration from snapshot + delta

### Deterministic Runtime Core

- `ExecutionState`
- `Apply(event)`
- pure scheduler
- command ordering
- graph materialization from events

### Worker Execution

- worker registry by activity type
- task polling
- success/failure completion
- gRPC worker transport
- heartbeat-driven liveness
- lease tracking and expiry

### Time-Dependent Reliability

- retry backoff timers
- node execution deadline
- node absolute deadline
- checkpoint timeout
- overdue timer catch-up on restart

### Dynamic Orchestration

- runtime node spawn
- dependency resolution
- cycle rejection

### Control Flow

- graceful cancellation
- compensation stack and unwind
- checkpoint approval flow

### Read Surface

- `StartRun`
- `CancelRun`
- `ApproveCheckpoint`
- `GetRun`
- `ListEvents`
- `StreamEvents`
- gRPC orchestration/read transport

### Test Surface

- crash recovery
- lease expiry
- retries
- timer recovery
- dynamic spawn
- checkpoints
- cancellation
- compensation

---

## Explicit v1 Exclusions

The following areas are intentionally excluded from v1 planning and should not generate first-wave backlog:

- memory store and memory profile injection
- embeddings, HNSW, BM25, relationship scoring
- sub-workflows and parent-child propagation
- activity capability version matching
- admission queues and admission timeouts
- error handler branches
- multiple projection modes
- workflow migration API
- mTLS, RBAC, and tenant isolation
- dashboard and platform features

---

## Open Decisions To Lock Before Backlog Finalization

These questions do not block planning, but they should be frozen before implementation-heavy backlog generation:

### 1. Lease Renewal API Shape

Need to decide whether lease renewal is:

- a required public worker RPC in v1, or
- an internalized/simplified mechanism that can be added explicitly in v1.1

The semantics themselves are required. The unresolved part is the API shape.

### 2. Checkpoints as Strict v1 Scope

Current documents imply checkpoints are expensive but still part of the intended v1 demo. For planning purposes, checkpoints should be treated as late-v1, not optional and not first-wave core.

### 3. External Events

External event ingestion is currently under-specified for the real v1 critical path. Unless it is needed for the first end-to-end demo, it should remain off the critical path.

### 4. Workflow Definition Contract

Need one minimal v1 contract for how workflows are defined, hashed, and submitted through `StartRun`.

---

## Planning Rule

If a proposed task does not strengthen one of the canonical v1 surfaces above, it should be assumed out of scope unless there is an explicit reason to include it.

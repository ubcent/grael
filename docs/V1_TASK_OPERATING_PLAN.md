# Grael v1 Task Operating Plan

This document is the operational companion to `docs/V1_TASK_BACKLOG.md`.

The backlog remains the canonical spec-driven task definition. This file adds delivery-oriented planning fields so the work can be prioritized, sequenced, estimated, and assigned.

---

## Field Definitions

- `Priority`
  - `P0`: blocks the core durable execution path
  - `P1`: required for intended v1, but not the first critical path
  - `P2`: important polish, integration, or final proof layer
- `Estimate`
  - `XS`: less than 0.5 day
  - `S`: 0.5 to 1.5 days
  - `M`: 2 to 4 days
  - `L`: 4 to 7 days
  - `XL`: more than 1 week
- `Risk`
  - `Low`: straightforward implementation or bounded blast radius
  - `Medium`: meaningful correctness or integration risk
  - `High`: high uncertainty, durability risk, or concurrency/correctness complexity
- `Owner`
  - left blank for now; assign when execution starts

---

## Operating Table

| Task | Capability | Priority | Estimate | Risk | Owner | Notes |
|---|---|---|---|---|---|---|
| `T1` | `C1` | `P0` | `S` | `Medium` |  | WAL format is foundational; mistakes cascade into all recovery paths |
| `T2` | `C1` | `P0` | `M` | `High` |  | Tail corruption handling is easy to get subtly wrong |
| `T3` | `C1` | `P0` | `M` | `Medium` |  | Snapshot restore path is critical for recovery performance and correctness |
| `T4` | `C2` | `P0` | `M` | `High` |  | State model becomes the semantic center of the system |
| `T5` | `C2` | `P0` | `S` | `High` |  | Terminal-state and readiness bugs create silent orchestration corruption |
| `T6` | `C2` | `P0` | `M` | `Medium` |  | Scheduler purity is conceptually simple but central to correctness |
| `T7` | `C3` | `P0` | `M` | `Medium` |  | First end-to-end engine loop |
| `T8` | `C4` | `P0` | `S` | `Low` |  | Worker registry is conceptually simple but must stay minimal |
| `T9` | `C4` | `P0` | `M` | `Medium` |  | Public worker RPC shape is a stable external contract |
| `T10` | `C4` | `P0` | `M` | `High` |  | Attempt tracking errors show up later as duplicate or stale execution bugs |
| `T11` | `C4` | `P0` | `M` | `High` |  | Liveness and expiry paths are concurrency-sensitive |
| `T12` | `C4` | `P0` | `S` | `High` |  | Stale-result rejection is a correctness-critical guardrail |
| `T13` | `C5` | `P0` | `M` | `High` |  | Timers are the only legal time-entry path and must survive restart |
| `T14` | `C5` | `P0` | `S` | `Medium` |  | Retry logic is bounded but visible everywhere in agent workloads |
| `T15` | `C5` | `P0` | `S` | `Medium` |  | Execution deadline is narrow in scope but high-value |
| `T16` | `C5` | `P1` | `S` | `Medium` |  | Smaller than it looks, but easy to conflate with checkpoint timeout semantics |
| `T17` | `C6` | `P0` | `M` | `Medium` |  | First living-DAG milestone |
| `T18` | `C6` | `P0` | `M` | `High` |  | Dynamic graph durability is one of the signature correctness risks |
| `T19` | `C6` | `P0` | `S` | `High` |  | Cycle rejection must fail safely before bad graph state is recorded |
| `T20` | `C7` | `P1` | `S` | `Low` |  | API/event entry point for cancellation |
| `T21` | `C7` | `P1` | `M` | `Medium` |  | Cancellation propagation has many node-state branches |
| `T22` | `C7` | `P1` | `S` | `Medium` |  | Compensation stack rules are simple but semantically important |
| `T23` | `C7` | `P1` | `M` | `Medium` |  | Sequential compensation touches failure semantics and worker execution |
| `T24` | `C7` | `P1` | `M` | `High` |  | Mid-unwind restart handling is easy to get wrong |
| `T25` | `C8` | `P1` | `M` | `Medium` |  | First checkpoint waiting-state slice |
| `T26` | `C8` | `P1` | `S` | `Medium` |  | Approval API is small but tightly coupled to waiting-state correctness |
| `T27` | `C8` | `P1` | `S` | `Medium` |  | Checkpoint timeout builds on timer infrastructure cleanly |
| `T28` | `C8` | `P1` | `M` | `High` |  | Restarted approval flows need careful rehydration semantics |
| `T29` | `C9` | `P0` | `S` | `Low` |  | Minimal start contract is essential and relatively bounded |
| `T30` | `C9` | `P0` | `S` | `Low` |  | `GetRun` is a thin but important product surface |
| `T31` | `C9` | `P0` | `S` | `Low` |  | `ListEvents` should be mechanically simple once WAL access exists |
| `T32` | `C9` | `P1` | `XS` | `Low` |  | Small policy slice with clear product value |
| `T33` | `C10` | `P1` | `M` | `Medium` |  | Definition contract should stay minimal to avoid over-design |
| `T34` | `C10` | `P2` | `M` | `Low` |  | Thin SDK seam should follow stable worker protocol, not precede it |
| `T35` | `C11` | `P2` | `M` | `Medium` |  | Demo harness is composition work after lower-level slices are real |

---

## Recommended Execution Buckets

### Bucket A: Non-Negotiable Core

- `T1` to `T15`
- `T17` to `T19`
- `T29` to `T31`

These tasks establish:

- durable storage and recovery
- deterministic orchestration
- real worker execution
- retries and deadlines
- living DAG
- minimal product-visible APIs

### Bucket B: Intended v1 Control Flow

- `T16`
- `T20` to `T28`
- `T32`
- `T33`

These tasks establish:

- hard deadline behavior during approval
- cancellation
- compensation
- checkpoint flows
- explicit capacity contract
- minimal authoring contract

### Bucket C: Final Product Layer

- `T34`
- `T35`

These tasks establish:

- thin SDK seam
- flagship demo path

---

## Risk Hotspots

These tasks deserve extra design review, tighter acceptance discipline, or smaller sub-slicing before implementation:

- `T2` WAL scan and corruption-boundary recovery
- `T4` ExecutionState and event application core
- `T10` lease/attempt tracking
- `T11` heartbeat timeout and lease expiry monitor
- `T12` stale worker result rejection
- `T13` timer scheduling and firing engine
- `T18` dynamic graph scheduling and persisted rehydration
- `T19` spawn validation and cycle rejection
- `T24` compensation recovery after restart
- `T28` checkpoint recovery across restart

---

## Suggested Owner Strategy

If this work is done mostly sequentially by one person, ownership can still be used as a focus flag:

- `owner-runtime`: `T1` to `T7`
- `owner-worker`: `T8` to `T12`
- `owner-time`: `T13` to `T16`
- `owner-graph`: `T17` to `T19`
- `owner-control`: `T20` to `T28`
- `owner-surface`: `T29` to `T35`

If the work is parallelized later, these groupings minimize semantic overlap.

---

## Practical Recommendation

If the immediate goal is momentum, start with:

1. `T1`
2. `T4`
3. `T6`
4. `T7`
5. `T29`
6. `T8`
7. `T9`
8. `T10`
9. `T13`
10. `T14`

This sequence gets the project to a real, testable execution skeleton quickly without over-investing in late-v1 flows too early.

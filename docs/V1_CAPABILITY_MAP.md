# Grael v1 Capability Map

This document breaks Grael v1 into capability slices that can later be turned into spec-driven backlog items.

The goal is to plan by verifiable system capability, not by package ownership or abstract architecture layers.

---

## Capability Overview

| ID | Capability | Goal |
|---|---|---|
| C1 | Storage and Recovery Foundation | Make WAL the only source of truth and guarantee restart recovery |
| C2 | Execution State and Deterministic Runtime Core | Define state semantics and pure orchestration behavior |
| C3 | Command Processing and Run Loop | Connect deterministic decisions to persisted execution progress |
| C4 | Worker Dispatch, Registry, and Leases | Give the engine a real task execution model |
| C5 | Timers, Retries, and Deadlines | Make time-driven behavior durable and crash-safe |
| C6 | Living DAG and Graph Mutation | Support runtime graph growth safely |
| C7 | Cancellation and Basic Compensation | Stop and unwind runs in a controlled way |
| C8 | Checkpoints and Human Approval | Pause specific nodes for approval without stopping the whole run |
| C9 | Public API and Minimal Read Surface | Expose the smallest useful client interface for v1 |
| C10 | Workflow Definition and SDK Seam | Define the minimum authoring/execution contract |
| C11 | Integration Reliability Test Suite | Prove v1 guarantees through executable tests |

---

## C1. Storage and Recovery Foundation

### Goal

Guarantee that Grael can recover run state after process crash using only persisted event history and snapshots.

### Scope

- append-only WAL per run
- event encoding and integrity checks
- WAL scanning and replay
- offset index rebuild
- snapshot persistence
- rehydration from snapshot + delta

### Acceptance Shape

- identical WAL input always rehydrates to identical `ExecutionState`
- corrupt WAL tail is detected and handled without invalidating valid prior events
- rehydration is read-only

### Key Risks

- hidden mutable state outside WAL
- incorrect snapshot/wal boundary handling
- tail corruption recovery bugs

---

## C2. Execution State and Deterministic Runtime Core

### Goal

Represent all runtime state transitions through event application and make orchestration decisions fully deterministic.

### Scope

- `ExecutionState`
- `Apply(event)`
- node states and transitions
- run states and transitions
- graph materialization from events
- scheduler purity
- deterministic command ordering

### Acceptance Shape

- equal event streams produce equal state
- equal state produces equal command sequences
- terminal node semantics are enforced consistently

### Key Risks

- state transitions hidden in command execution
- non-deterministic ordering
- node lifecycle drift between docs and implementation

---

## C3. Command Processing and Run Loop

### Goal

Provide the engine loop that continuously derives commands from state and turns them back into persisted events.

### Scope

- `RunLoop`
- `WaitNext`
- command processor
- command-to-event append flow
- initial run bootstrap after `StartRun`

### Acceptance Shape

- a run can progress end-to-end through repeated event application and command execution
- crash between decision and append is recoverable because commands are re-derivable

### Key Risks

- duplicate side effects around retries or restart
- run loop blocking on non-essential work

---

## C4. Worker Dispatch, Registry, and Leases

### Goal

Enable workers to poll, execute, and complete tasks under explicit lease ownership.

### Scope

- worker registry by activity type
- long-poll dispatch
- `PollTask`
- `CompleteTask`
- `FailTask`
- `Heartbeat`
- lease grant
- lease expiry
- stale result rejection

### Acceptance Shape

- workers can execute real tasks end-to-end
- lost workers do not leave tasks stuck forever
- late results after lease expiry are rejected deterministically

### Key Risks

- task duplication around expiry races
- under-specified worker liveness semantics

---

## C5. Timers, Retries, and Deadlines

### Goal

Persist and recover all time-based behavior needed for correctness.

### Scope

- timer manager
- persisted timer scheduling
- persisted timer firing
- retry backoff
- execution deadline
- absolute deadline
- checkpoint timeout
- overdue timer catch-up

### Acceptance Shape

- retries survive restart
- overdue timers fire after recovery
- timeouts do not depend on in-memory process continuity

### Key Risks

- hidden wall-clock checks in scheduler
- deadline bugs around approval pauses

---

## C6. Living DAG and Graph Mutation

### Goal

Allow completed nodes to grow the graph at runtime while preserving graph validity.

### Scope

- spawned node payloads
- runtime node insertion
- dependency linking
- cycle detection
- graph rebuild on rehydration
- spawn caps and warnings

### Acceptance Shape

- a node can complete and add new `PENDING` nodes
- graph shape after restart matches pre-crash event history
- invalid spawn attempts are rejected cleanly

### Key Risks

- cycles recorded in WAL
- dependency references to missing nodes

---

## C7. Cancellation and Basic Compensation

### Goal

Allow a run to be cancelled and, when configured, unwound through sequential compensation.

### Scope

- `GracefulCancel`
- cancel propagation
- node cancellation handling by state
- compensation stack construction
- sequential compensation execution
- optional compensation on cancel

### Acceptance Shape

- cancellation reaches a terminal run outcome without stranded nodes
- only completed nodes are compensable
- compensation resumes safely after crash

### Key Risks

- compensating nodes that never completed
- ambiguous interaction between cancel and compensation

---

## C8. Checkpoints and Human Approval

### Goal

Support approval gates on individual nodes while the rest of the run continues.

### Scope

- checkpoint request result shape
- `CheckpointReached`
- `AWAITING_APPROVAL`
- approve/reject APIs
- checkpoint timeout
- redispatch after approval

### Acceptance Shape

- one node can wait for approval while unrelated nodes continue
- approval state survives restart
- deadlines still behave correctly while awaiting approval

### Key Risks

- approval path accidentally blocking the whole run
- deadline bypass through checkpoints

---

## C9. Public API and Minimal Read Surface

### Goal

Expose a small but sufficient API for starting runs, controlling them, and observing current state plus raw history.

### Scope

- `StartRun`
- `CancelRun`
- `ApproveCheckpoint`
- `GetRun`
- `ListEvents`
- immediate reject on capacity

### Acceptance Shape

- users can launch, inspect, and control runs without extra projection systems
- `GetRun` exposes derived current state
- `ListEvents` exposes forensic raw history

### Key Risks

- accidental introduction of duplicate source-of-truth read models

---

## C10. Workflow Definition and SDK Seam

### Goal

Freeze the smallest workable authoring contract for workflows and worker handlers.

### Scope

- workflow definition shape
- node definition shape
- dependency declaration
- retry/deadline/checkpoint/compensation config
- definition hash captured at run start
- thin Go SDK seam for worker registration and task handling

### Acceptance Shape

- workflows are expressive enough for the core v1 demos
- activity compatibility is handled by naming, not version routing

### Key Risks

- over-designing the definition registry before end-to-end execution works

---

## C11. Integration Reliability Test Suite

### Goal

Turn runtime guarantees into executable tests rather than leaving them as prose.

### Scope

- crash recovery tests
- retry tests
- lease expiry tests
- timer recovery tests
- dynamic spawn tests
- checkpoint tests
- cancellation tests
- compensation tests

### Acceptance Shape

- critical v1 guarantees are enforced by repeatable automated tests
- regressions in recovery and orchestration behavior are caught early

### Key Risks

- relying on specification text without executable coverage

---

## Dependency Order

Recommended critical-path dependency order:

1. `C1` Storage and Recovery Foundation
2. `C2` Execution State and Deterministic Runtime Core
3. `C3` Command Processing and Run Loop
4. `C4` Worker Dispatch, Registry, and Leases
5. `C5` Timers, Retries, and Deadlines
6. `C6` Living DAG and Graph Mutation
7. `C7` Cancellation and Basic Compensation
8. `C8` Checkpoints and Human Approval
9. `C9` Public API and Minimal Read Surface
10. `C10` Workflow Definition and SDK Seam
11. `C11` Integration Reliability Test Suite

---

## Delivery Waves

### Wave 1: Runtime Bedrock

- `C1`
- `C2`
- `C3`

### Wave 2: Real Execution Reliability

- `C4`
- `C5`

### Wave 3: Product Differentiator

- `C6`

### Wave 4: Operational Control Flow

- `C7`
- `C8`

### Wave 5: Product Surface and Proof

- `C9`
- `C10`
- `C11`

---

## Priority Classification

### Must-Have Core

- `C1`
- `C2`
- `C3`
- `C4`
- `C5`
- `C6`
- `C9`
- `C11`

### Late v1

- `C7`
- `C8`
- `C10`

Late v1 still belongs to the intended product, but it should not displace the execution and recovery foundation.

---

## Task Design Rule

When this map is turned into a backlog, tasks should be written as verifiable slices of capability.

Prefer:

- `C1.2 WAL scan and tail corruption recovery`
- `C4.3 Reject late CompleteTask after LeaseExpired`
- `C6.2 Runtime spawn validation and cycle rejection`

Avoid:

- `implement wal package`
- `build scheduler`
- `work on worker stuff`

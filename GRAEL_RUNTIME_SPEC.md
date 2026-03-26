# Grael Runtime Specification

**Version:** 1.0-draft
**Status:** Normative
**Supersedes:** `ARCHITECTURE.md`, `ARCHITECTURE_ADDENDUM.md`, `ARCHITECTURE_CORRECTIONS.md`

This document is the authoritative runtime semantics specification for the Grael workflow engine. Where this document conflicts with earlier architecture documents, this document governs. Ambiguities in earlier documents are resolved here with a single, precise model.

Conforming implementations MUST satisfy every MUST/MUST NOT clause. MAY and SHOULD clauses describe recommended behavior.

---

## Table of Contents

1. Runtime Model Overview
2. Global Invariants
3. Full Event Catalog
4. Full Command Catalog
5. Formal Node State Machine
6. Formal Run State Machine
7. Timer Semantics
8. Lease & Worker Protocol Spec
9. Condition Evaluation Spec
10. Cancellation Spec
11. Compensation Spec
12. Deterministic Ordering Rules
13. Memory Semantics
14. Projection Semantics
15. Recovery & Crash Semantics
16. Illegal States and Rejected Sequences
17. Conformance Test Matrix
18. Naming Corrections

---

## 1. Runtime Model Overview

### 1.1 Entity Definitions

#### Run

| Attribute | Value |
|-----------|-------|
| Purpose | A single execution instance of a workflow definition |
| Lifecycle | `ADMITTED_QUEUED` → ... → terminal state |
| Source of truth | WAL: derived by applying all events with matching `RunID` |
| Persisted | Yes — `WorkflowStarted` event is the creation record |
| Identity | `RunID` — globally unique, opaque string |

A Run holds the execution context for one invocation of a workflow definition. Its state is never stored independently; it is always derived from the WAL.

#### Node

| Attribute | Value |
|-----------|-------|
| Purpose | A unit of work within a Run's graph |
| Lifecycle | `PENDING` → ... → terminal state |
| Source of truth | WAL: derived by applying node-scoped events |
| Persisted | Node definitions are written as part of `WorkflowStarted` or `NodeCompleted{spawn}` events |
| Identity | `NodeID` — unique within a Run, stable across retries |

A Node represents one activity invocation or control construct (checkpoint, sub-workflow, fan-out). Retries of the same Node do not change its `NodeID`; they create new `AttemptID` values.

#### Event

| Attribute | Value |
|-----------|-------|
| Purpose | An immutable fact recording that something happened |
| Lifecycle | Append-only; never mutated or deleted during a Run's active life |
| Source of truth | WAL — the event IS the source of truth |
| Persisted | Yes — append-only WAL with CRC32 integrity |
| Ordering | Total order within a Run via monotonic `Seq` number |

No state transition in the engine may occur without a corresponding event in the WAL. This is an absolute invariant.

#### Command

| Attribute | Value |
|-----------|-------|
| Purpose | An instruction to perform a side effect |
| Lifecycle | Produced by Scheduler; consumed by CommandProcessor; not persisted |
| Source of truth | Derived — commands are always re-derivable from current ExecutionState |
| Persisted | No |

Commands are never written to the WAL. They are ephemeral instructions. CommandProcessor executes commands and writes resulting events to the WAL. If the engine crashes between producing a command and writing its resulting event, the command is re-derived after rehydration.

#### Lease

| Attribute | Value |
|-----------|-------|
| Purpose | Exclusive ownership claim on a task attempt by a specific worker |
| Lifecycle | `LeaseGranted` → (`LeaseRenewed`*) → `LeaseExpired` or voided by `NodeCompleted`/`NodeFailed` |
| Source of truth | WAL: `LeaseGranted`, `LeaseRenewed`, `LeaseExpired` events |
| Persisted | Yes — via events |
| Identity | Keyed by `(AttemptID, WorkerID)` |

A lease binds one AttemptID to one WorkerID. An expired lease is permanently dead and cannot be revived. A lease is automatically voided when `NodeCompleted` or `NodeFailed` is written for the same AttemptID.

#### Timer

| Attribute | Value |
|-----------|-------|
| Purpose | A scheduled future event keyed to a wall-clock time |
| Lifecycle | `TimerScheduled` → `TimerFired` or `TimerCancelled` |
| Source of truth | WAL: all three event types |
| Persisted | Yes — via events |
| Identity | `TimerID` — unique per scheduled timer instance |

Timers are the only mechanism by which wall-clock time may influence workflow state. All timers MUST be persisted before they are armed. The in-memory timer heap is always reconstructable from WAL events.

#### Workflow Definition

| Attribute | Value |
|-----------|-------|
| Purpose | Static description of node types, dependencies, policies, and compensation |
| Lifecycle | Immutable once registered; new versions registered separately |
| Source of truth | Definition registry (separate from WAL) |
| Persisted | Yes — definition registry |
| Identity | `(WorkflowName, WorkflowVersion)` |

A Run is pinned to a specific workflow version at creation. The definition does not change for the lifetime of a Run. The engine simultaneously serves multiple versions.

#### Activity Definition

| Attribute | Value |
|-----------|-------|
| Purpose | Specification of an activity type: required worker capability version range, retry policy, deadlines, compensation handler |
| Lifecycle | Part of a Workflow Definition |
| Source of truth | Workflow Definition |
| Persisted | Yes — with Workflow Definition |

#### Projection

| Attribute | Value |
|-----------|-------|
| Purpose | A derived, read-optimized view of a Run's state for a specific consumer |
| Lifecycle | Computed on demand from WAL + latest snapshot |
| Source of truth | Derived — WAL is source of truth |
| Persisted | No — never stored as primary state |

Three projection modes exist: Compact, Operational, Forensic. See §14.

#### Compensation Stack

| Attribute | Value |
|-----------|-------|
| Purpose | Ordered list of completed nodes that have defined compensation handlers, to be executed in reverse order on failure |
| Lifecycle | Grows as nodes reach COMPLETED; consumed in reverse order during compensation |
| Source of truth | Derived from WAL: all `NodeCompleted` events for nodes with `compensate:` defined |
| Persisted | Derived — reconstructed during rehydration |

Only nodes that reached COMPLETED state enter the compensation stack. FAILED, SKIPPED, and CANCELLED nodes are excluded.

#### Memory Profile

| Attribute | Value |
|-----------|-------|
| Purpose | Contextual knowledge injected into agent activities at run start |
| Lifecycle | Assembled once at `WorkflowStarted`; frozen for run duration (default) |
| Source of truth | `WorkflowStarted` payload (or `MemoryRefreshed` event for `ManualRefresh` mode) |
| Persisted | Yes — embedded in the triggering WAL event |

Under `StablePerRun` (default) and `ManualRefresh` modes, the memory profile is deterministic. Under `UnsafeRefreshOnBoundary` mode, the profile at boundary points comes from the live memory store and is not deterministically reproducible.

---

## 2. Global Invariants

These invariants are absolute. Any implementation that violates them is incorrect. They are organized as load-bearing rules that, if broken, cause silent data corruption, non-determinism, or replay divergence.

### I-1: WAL is the Only Source of Truth

Every piece of state the engine acts on MUST be derivable from the WAL. No component may maintain authoritative state independently of the WAL.

### I-2: Rehydration is Read-Only

```
Rehydration MUST NOT append any event to the WAL.
Rehydration MUST NOT emit any Command.
Rehydration MUST NOT contact any worker.
Rehydration MUST NOT perform any I/O.
Rehydration is a pure, deterministic function: WAL → ExecutionState.
```

### I-3: Every State Transition is Caused by Exactly One Persisted Event

```
∀ transition T: ∃ event e ∈ WAL such that T = Apply(e)
No transition may be caused by:
  - observing wall-clock time directly
  - receiving a network message
  - polling a database
  - any mechanism other than Apply(event) where event ∈ WAL
```

### I-4: The Scheduler Has No Access to Wall-Clock Time

The Scheduler MUST NOT call `time.Now()` or any equivalent. All time-dependent behavior is expressed as timer events. The Scheduler reacts to `TimerFired` and `LeaseExpired` events; it does not originate time observations.

### I-5: Wall-Clock Time May Only Enter the WAL Through Designated Writers

The only components authorized to observe wall-clock time and write time-triggered events to the WAL are:

| Component | Event written |
|-----------|--------------|
| TimerManager | `TimerFired` |
| LeaseMonitor | `LeaseExpired` |
| AdmissionMonitor | `AdmissionTimedOut` |

No other component may write time-triggered events.

### I-6: Expired Lease is Permanently Dead

```
Once LeaseExpired is written to the WAL for (AttemptID, WorkerID),
no subsequent RenewLease or CompleteTask for that (AttemptID, WorkerID) is valid.
The lease MUST NOT be revived.
Late results from the original worker MUST be discarded unconditionally.
```

### I-7: Completed Nodes Never Re-Enter Dispatch

A node in state `COMPLETED` or `SKIPPED` MUST NOT appear in the Scheduler's ready-node set. Rehydration applies all WAL events; nodes already `COMPLETED` are already reflected as such in `ExecutionState`. They will never reach `dispatch()`.

### I-8: The Scheduler is a Pure Function

```
Scheduler.Decide(state ExecutionState) → []Command
```

This function MUST have no side effects, no I/O, no goroutines, no randomness, and no access to wall-clock time. Given identical input state, it MUST return identical output commands in identical order.

### I-9: Command Ordering is Deterministic and Stable

Given the same `ExecutionState`, `Scheduler.Decide()` MUST always produce the same `[]Command` in the same order. The ordering rules are defined in §12.

### I-10: Conditions Are Closed Over Recorded State

Skip conditions MUST be pure functions of events already present in the WAL at the time of evaluation. They MUST NOT access wall-clock time, external services, or the live memory store. See §9 for full definition.

### I-11: Compensation Applies Only to COMPLETED Nodes

The compensation stack MUST contain only nodes that reached the `COMPLETED` state. `FAILED`, `SKIPPED`, `CANCELLED`, and `TIMED_OUT` nodes MUST NOT enter the compensation stack.

### I-12: In-Progress Side Effects at Cancellation Time Are Not Guaranteed Compensable

When a run is cancelled (either `GracefulCancel` or `RevokeAndAbandon`), nodes that have not yet reached `COMPLETED` at the moment of cancellation are excluded from the compensation stack. The engine makes no guarantee that side effects produced by in-progress nodes at cancellation time will be reversed.

### I-13: "Orchestration Replay" Does Not Exist in Grael

Grael does not re-execute orchestration code during recovery. Rehydration rebuilds `ExecutionState` by applying events. There is no interception mechanism, no code re-execution, and no "replay mode" in the Temporal sense. The term "replay" in Grael refers exclusively to state reconstruction from events.

### I-14: Fan-Out Graph Mutations Are Atomic

`NodeCompleted` carries both the output and the `spawn` list in a single payload. These MUST be processed atomically. There is no state where output is visible but spawned nodes are not, or vice versa.

### I-15: Spawned Nodes May Only Reference Existing Nodes

Nodes spawned via `NodeCompleted{spawn}` MUST only declare dependencies on nodes that already exist in the graph at the time of the `NodeCompleted` event. Forward references to not-yet-existing nodes are illegal and MUST result in a `GraphViolationError`.

### I-16: AbsoluteDeadline Cannot Be Paused or Extended

Once scheduled, the `AbsoluteDeadline` timer for a node MUST fire at `nodeCreatedAt + AbsoluteDeadlineConfig`, regardless of the node's current state, including `AWAITING_APPROVAL`. No approval, configuration, or API call may extend or pause the AbsoluteDeadline.

### I-17: UnsafeRefreshOnBoundary Requires Explicit Opt-In

A run MUST NOT use `UnsafeRefreshOnBoundary` memory policy unless the `StartWorkflow` request explicitly includes `AllowNondeterministicMemory: true`. Absence of this flag with `UnsafeRefreshOnBoundary` policy MUST cause request rejection.

### I-18: Projections Are Never Primary State

Projections (Compact, Operational, Forensic) MUST be computed from the WAL. No projection data may serve as input to the Scheduler or CommandProcessor. Projections are read-only, derived outputs.

### I-19: Late CompleteTask After LeaseExpired is a Protocol Violation

A worker that receives `LEASE_EXPIRED` in response to `RenewLease` or `CompleteTask` MUST stop processing that task immediately. Retrying `CompleteTask` with the same `AttemptID` after receiving `LEASE_EXPIRED` is a protocol violation. The engine MUST reject such calls unconditionally.

### I-20: RevokeAndAbandon Cannot Kill Worker Processes

`RevokeAndAbandon` revokes leases and rejects future results. It MUST NOT claim to terminate worker processes or interrupt blocking I/O. Workers may continue executing after `RevokeAndAbandon`. Their results will be rejected.

---

## 3. Full Event Catalog

Events are immutable facts written to the WAL. Each event has a unique monotonic `Seq` number within a Run.

**Notation:**
- `Emitter`: the component that writes the event
- `Precondition`: the Run/Node state that must hold before the event is valid
- `Effect`: the state transition caused by applying the event
- `Terminal`: whether the event transitions a Run or Node to a terminal state
- `Illegal duplicate`: sequences that MUST be rejected

---

### 3.1 Run Lifecycle Events

#### `WorkflowStarted`

| Field | Value |
|-------|-------|
| Emitter | API handler (on `StartWorkflow` RPC) |
| Precondition | Run does not exist OR (Run exists AND `RunID` is idempotent match) |
| Effect | Run created in `RUNNING` state; initial graph nodes created as `PENDING`; memory profile locked |
| Terminal | No |
| Payload | `RunID`, `WorkflowName`, `WorkflowVersion`, `Input`, `MemoryProfile`, `NondeterministicMemory bool`, `Policy overrides`, `CreatedAt` |
| Illegal duplicate | Second `WorkflowStarted` for same `RunID` with different payload is rejected; same payload is idempotent |

#### `AdmissionQueued`

| Field | Value |
|-------|-------|
| Emitter | API handler (when engine is at capacity) |
| Precondition | Run does not exist; engine admission queue has capacity |
| Effect | Run created in `ADMITTED_QUEUED` state |
| Terminal | No |
| Payload | `RunID`, `WorkflowName`, `WorkflowVersion`, `Input`, `QueuedAt`, `AdmissionDeadline` |

#### `AdmissionTimedOut`

| Field | Value |
|-------|-------|
| Emitter | AdmissionMonitor |
| Precondition | Run in `ADMITTED_QUEUED`; `AdmissionDeadline` reached |
| Effect | Run transitions to `ADMISSION_REJECTED` |
| Terminal | Yes (Run) |
| Payload | `RunID`, `QueuedAt`, `TimedOutAt` |
| Illegal duplicate | MUST NOT appear for Run not in `ADMITTED_QUEUED` |

#### `AdmissionAccepted`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (executing `AdmitQueuedRun`) |
| Precondition | Run in `ADMITTED_QUEUED`; capacity available |
| Effect | Run transitions to `RUNNING`; initial nodes become `PENDING` |
| Terminal | No |
| Payload | `RunID`, `AcceptedAt` |

#### `CancellationRequested`

| Field | Value |
|-------|-------|
| Emitter | API handler (on `CancelRun` RPC) |
| Precondition | Run in `RUNNING`, `HANDLING_ERROR`, `ADMITTED_QUEUED` |
| Effect | Cancellation propagation begins; PENDING/READY nodes immediately transition to CANCELLED |
| Terminal | No (transitional — precedes `CancellationCompleted`) |
| Payload | `RunID`, `RequestedBy`, `CancellationType` (`graceful` | `revoke_and_abandon`), `GraceDeadline`, `RequestedAt` |
| Illegal duplicate | If Run already in cancelling or terminal state, request is rejected |

#### `CancellationCompleted`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (after all nodes reach terminal state post-cancellation) |
| Precondition | `CancellationRequested` exists; all nodes terminal |
| Effect | Run transitions to `CANCELLED` or `CANCELLED_WITH_COMPENSATION` |
| Terminal | Yes (Run) |
| Payload | `RunID`, `CompensationRan bool`, `CompletedAt` |

#### `WorkflowCompleted`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (executing `CompleteWorkflow`) |
| Precondition | All terminal nodes are `COMPLETED` or `SKIPPED`; no `FAILED`/`CANCELLED` nodes remain |
| Effect | Run transitions to `COMPLETED` |
| Terminal | Yes (Run) |
| Payload | `RunID`, `Output`, `CompletedAt` |

#### `WorkflowFailed`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (executing `FailWorkflow`) |
| Precondition | A node has failed with retries exhausted; no error handler defined or error handler also failed; not entering compensation |
| Effect | Run transitions to `FAILED` |
| Terminal | Yes (Run) |
| Payload | `RunID`, `Trigger` (`node_failure` | `budget_exceeded` | `deadline` | `graph_violation`), `TriggerNodeID`, `Reason`, `FailedAt` |

---

### 3.2 Node Lifecycle Events

#### `NodeReady`

| Field | Value |
|-------|-------|
| Emitter | Scheduler (via CommandProcessor) |
| Precondition | Node in `PENDING`; all dependency nodes in `COMPLETED` or `SKIPPED` |
| Effect | Node transitions to `READY` |
| Terminal | No |
| Payload | `RunID`, `NodeID`, `ReadyAt` |

#### `NodeStarted`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (after dispatching to worker and writing lease) |
| Precondition | Node in `READY`; lease granted |
| Effect | Node transitions to `RUNNING`; `AbsoluteDeadline` timer scheduled; `ExecutionDeadline` countdown starts |
| Terminal | No |
| Payload | `RunID`, `NodeID`, `AttemptID`, `OperationID`, `WorkerID`, `StartedAt` |
| Illegal duplicate | MUST NOT appear for Node already in `RUNNING`, `COMPLETED`, `FAILED`, `SKIPPED`, `CANCELLED` |

#### `NodeCompleted`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (after receiving `CompleteTask` from worker with success) |
| Precondition | Node in `RUNNING` or `AWAITING_APPROVAL`; `AttemptID` matches active lease; lease not expired |
| Effect | Node transitions to `COMPLETED`; spawned nodes added as `PENDING`; node pushed to compensation stack if `compensate:` defined |
| Terminal | Yes (Node) |
| Payload | `RunID`, `NodeID`, `AttemptID`, `Output`, `SpawnedNodes []NodeDefinition`, `CompletedAt` |
| Illegal duplicate | MUST NOT appear for Node in `COMPLETED`, `FAILED`, `SKIPPED`, `CANCELLED`, `TIMED_OUT` |

#### `NodeFailed`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (after receiving `FailTask` from worker, or after lease expiry with retries exhausted) |
| Precondition | Node in `RUNNING`; retries exhausted or error is non-retryable |
| Effect | Node transitions to `FAILED`; error handler or compensation triggered |
| Terminal | Yes (Node) |
| Payload | `RunID`, `NodeID`, `AttemptID`, `ErrorCode`, `ErrorMessage`, `Attempt int`, `FailedAt` |

#### `NodeSkipped`

| Field | Value |
|-------|-------|
| Emitter | Scheduler (via CommandProcessor) |
| Precondition | Node in `PENDING`; skip condition evaluated true |
| Effect | Node transitions to `SKIPPED`; no compensation stack entry |
| Terminal | Yes (Node) |
| Payload | `RunID`, `NodeID`, `SkippedAt`, `Reason` |
| Note | `SKIPPED` is semantically distinct from `COMPLETED(nil)`. A skipped node produced no output and never ran. |

#### `NodeTimedOut`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (after `TimerFired{ExecutionDeadline}` or `TimerFired{AbsoluteDeadline}`) |
| Precondition | Node in `RUNNING` or `AWAITING_APPROVAL` |
| Effect | Node transitions to `TIMED_OUT`; applies same downstream semantics as `NodeFailed` (retry policy, compensation) |
| Terminal | Yes (Node) |
| Payload | `RunID`, `NodeID`, `AttemptID`, `DeadlineType` (`execution` | `absolute`), `TimedOutAt` |
| Note | `TIMED_OUT` is a distinct terminal state for observability. Retry and compensation logic treats it identically to `NodeFailed`. |

#### `NodeCancelled`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (during cancellation propagation) |
| Precondition | Node in `RUNNING`, `AWAITING_APPROVAL`, `PENDING`, or `READY` when `CancellationRequested` is processed |
| Effect | Node transitions to `CANCELLED` |
| Terminal | Yes (Node) |
| Payload | `RunID`, `NodeID`, `Graceful bool`, `CancelledAt` |

---

### 3.3 Retry Events

#### `NodeRetryScheduled`

| Field | Value |
|-------|-------|
| Emitter | Scheduler (via CommandProcessor) after `NodeFailed` or `NodeTimedOut` with retries remaining |
| Precondition | Node in `FAILED` or `TIMED_OUT`; `Attempt < MaxAttempts`; error is retryable |
| Effect | Node transitions back to `PENDING`; `TimerScheduled{retry_backoff}` written |
| Terminal | No |
| Payload | `RunID`, `NodeID`, `NextAttempt int`, `BackoffDuration`, `FireAt`, `ScheduledAt` |

---

### 3.4 Timer Events

#### `TimerScheduled`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (executing `ScheduleTimer`) |
| Precondition | No active unexpired timer with same `TimerID` |
| Effect | Timer registered in TimerManager heap; will fire at `FireAt` |
| Terminal | No |
| Payload | `TimerID`, `RunID`, `NodeID`, `FireAt time.Time` (absolute UTC), `Purpose TimerPurpose` |

#### `TimerFired`

| Field | Value |
|-------|-------|
| Emitter | TimerManager (only) |
| Precondition | `TimerScheduled` exists for `TimerID`; no `TimerCancelled` for same `TimerID` |
| Effect | State transition determined by `Purpose` field (see §7) |
| Terminal | No (itself); may cause terminal transition in node/run |
| Payload | `TimerID`, `RunID`, `NodeID`, `Purpose`, `FiredAt time.Time`, `Late bool` |

#### `TimerCancelled`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (executing `CancelTimer`) |
| Precondition | `TimerScheduled` exists; no `TimerFired` for same `TimerID` |
| Effect | Timer removed from TimerManager heap; will not fire |
| Terminal | No |
| Payload | `TimerID`, `Reason`, `CancelledAt` |
| Illegal sequence | MUST NOT appear after `TimerFired` for same `TimerID` |

---

### 3.5 Lease Events

#### `LeaseGranted`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (before `NodeStarted`) |
| Precondition | Node in `READY`; no active lease for same `(NodeID, AttemptID)` |
| Effect | Lease recorded; LeaseMonitor begins expiry tracking |
| Terminal | No |
| Payload | `AttemptID`, `NodeID`, `WorkerID`, `GrantedAt`, `ExpiresAt` |

#### `LeaseRenewed`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (after worker `RenewLease` RPC, if not expired) |
| Precondition | `LeaseGranted` exists for `(AttemptID, WorkerID)`; no `LeaseExpired` for same |
| Effect | `ExpiresAt` updated to `min(now + LeaseDuration, GrantedAt + MaxLeaseDuration)` |
| Terminal | No |
| Payload | `AttemptID`, `WorkerID`, `RenewedAt`, `NewExpiresAt` |
| Illegal sequence | MUST NOT appear after `LeaseExpired` for same `AttemptID` |

#### `LeaseExpired`

| Field | Value |
|-------|-------|
| Emitter | LeaseMonitor (only) |
| Precondition | `LeaseGranted` exists; `time.Now() >= ExpiresAt`; no `LeaseRenewed` written first |
| Effect | Lease is permanently dead; node transitions based on retry policy |
| Terminal | No (lease); may cause `NodeFailed` if retries exhausted |
| Payload | `AttemptID`, `WorkerID`, `ExpiredAt` |
| Note | Once written, `LeaseExpired` is permanent. No subsequent event may revive this lease. |

---

### 3.6 Checkpoint Events

#### `CheckpointReached`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (after worker returns a checkpoint instruction in `StepResult`) |
| Precondition | Node in `RUNNING`; worker lease valid |
| Effect | Node transitions to `AWAITING_APPROVAL`; worker lease released; `ExecutionDeadline` timer paused; `CheckpointTimeout` timer scheduled; `AbsoluteDeadline` timer continues |
| Terminal | No |
| Payload | `RunID`, `NodeID`, `AttemptID`, `Message`, `Targets []NotificationTarget`, `CheckpointDeadline`, `ReachedAt` |

#### `CheckpointApproved`

| Field | Value |
|-------|-------|
| Emitter | API handler (human or automated approval) |
| Precondition | Node in `AWAITING_APPROVAL` |
| Effect | Node transitions to `READY`; `CheckpointTimeout` timer cancelled; `ExecutionDeadline` timer resumed with remaining budget; new dispatch issued |
| Terminal | No |
| Payload | `RunID`, `NodeID`, `ApprovedBy`, `ApprovedAt` |
| Illegal sequence | MUST NOT appear for Node not in `AWAITING_APPROVAL` |

#### `CheckpointRejected`

| Field | Value |
|-------|-------|
| Emitter | API handler (human rejection) |
| Precondition | Node in `AWAITING_APPROVAL` |
| Effect | Node transitions to `FAILED`; applies retry policy |
| Terminal | Yes (Node) unless retried |
| Payload | `RunID`, `NodeID`, `RejectedBy`, `Reason`, `RejectedAt` |

---

### 3.7 Compensation Events

#### `CompensationStarted`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (executing `TriggerCompensation`) |
| Precondition | Run has a non-empty compensation stack; trigger condition met |
| Effect | Run transitions to `COMPENSATING`; SagaCoordinator begins reverse-order execution |
| Terminal | No |
| Payload | `RunID`, `Trigger CompensationTrigger`, `StackSize int`, `StartedAt` |

#### `CompensationActionCompleted`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (after compensation activity worker completes) |
| Precondition | Run in `COMPENSATING`; compensation action was active |
| Effect | Compensation advances to next stack entry |
| Terminal | No |
| Payload | `RunID`, `NodeID` (original node), `AttemptID`, `CompletedAt` |

#### `CompensationActionFailed`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (after compensation activity fails with retries exhausted) |
| Precondition | Run in `COMPENSATING` |
| Effect | Depends on `CompensationFailAction`: `continue` → advance to next; `halt` → stop |
| Terminal | No |
| Payload | `RunID`, `NodeID`, `Reason`, `FailedAt` |

#### `CompensationCompleted`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (after all compensation stack entries are processed) |
| Precondition | Run in `COMPENSATING` |
| Effect | Run transitions to `COMPENSATED`, `COMPENSATION_PARTIAL`, or `COMPENSATION_FAILED` |
| Terminal | Yes (Run) |
| Payload | `RunID`, `SucceededCount int`, `FailedCount int`, `HaltedEarly bool`, `CompletedAt` |

---

### 3.8 Error Handler Events

#### `HandlerStarted`

| Field | Value |
|-------|-------|
| Emitter | Scheduler (via CommandProcessor) |
| Precondition | Run's error handler node is defined; trigger node is `FAILED` or `TIMED_OUT` |
| Effect | Run transitions to `HANDLING_ERROR`; handler node transitions to `RUNNING` |
| Terminal | No |
| Payload | `RunID`, `HandlerNodeID`, `TriggerNodeID`, `StartedAt` |
| Illegal sequence | MUST NOT appear if no error handler is configured in the workflow definition |

#### `HandlerCompleted`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (handler activity worker completed) |
| Precondition | Run in `HANDLING_ERROR` |
| Effect | Run transitions to `FAILED_HANDLED` (terminal) |
| Terminal | Yes (Run) |
| Payload | `RunID`, `HandlerNodeID`, `Output`, `CompletedAt` |

#### `HandlerFailed`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (handler activity worker failed with retries exhausted) |
| Precondition | Run in `HANDLING_ERROR` |
| Effect | Compensation triggered if configured; else Run transitions to `FAILED` |
| Terminal | No (compensating) or Yes (Run, if no compensation) |
| Payload | `RunID`, `HandlerNodeID`, `Reason`, `FailedAt` |

---

### 3.9 Memory Events

#### `MemoryRefreshed`

| Field | Value |
|-------|-------|
| Emitter | CommandProcessor (executing `RefreshMemory` built-in activity) |
| Precondition | Run uses `ManualRefresh` or `UnsafeRefreshOnBoundary` policy; `RefreshMemory` node completed |
| Effect | New `MemoryProfile` recorded in WAL; subsequent nodes receive updated profile via `StepContext.Memory` |
| Terminal | No |
| Payload | `RunID`, `NodeID`, `MemoryProfile`, `RefreshedAt`, `Nondeterministic bool` |
| Note | Under `UnsafeRefreshOnBoundary`: profile is fetched from live memory store at boundary point. Replay may return different profile. |

---

### 3.10 External Event Events

#### `ExternalEventIngested`

| Field | Value |
|-------|-------|
| Emitter | API handler (webhook receiver or polling adapter) |
| Precondition | `ExternalEventID` not in deduplication store; target Run exists and is active |
| Effect | Event payload delivered to run; may unblock nodes awaiting external events |
| Terminal | No |
| Payload | `RunID`, `ExternalEventID`, `SourceID`, `EventType`, `Payload`, `IngestedAt` |
| Deduplication | `ExternalEventID` checked against dedup store before processing; duplicates are acked and dropped |

---

## 4. Full Command Catalog

Commands are ephemeral instructions produced by the Scheduler and executed by the CommandProcessor. They are never persisted directly. The resulting events of command execution are what get persisted.

**Notation:**
- `Producer`: the component that emits this command
- `Executor`: the component that executes this command
- `Side effects`: what the executor does
- `Events produced`: what events are written to WAL as a result
- `Failure outcomes`: what happens if execution fails

---

### 4.1 `DispatchActivity`

| Field | Value |
|-------|-------|
| Producer | Scheduler |
| Executor | CommandProcessor → WorkerPool |
| Side effects | Writes `LeaseGranted` event; places task in worker queue; sends task to polling worker |
| Events produced | `LeaseGranted`, `NodeStarted` |
| Failure outcomes | No compatible worker: task queued up to `NoWorkerTimeout`, then `NodeFailed{NO_COMPATIBLE_WORKER}` |

```
Fields: RunID, NodeID, AttemptID, OperationID, ActivityType, RequiredVersion,
        Input []byte, TaskIdentity, NodePolicy
```

### 4.2 `RequeueActivity`

| Field | Value |
|-------|-------|
| Producer | Scheduler (on lease expiry with retries remaining, or after retry timer fires) |
| Executor | CommandProcessor → WorkerPool |
| Side effects | Previous `AttemptID` is dead; new `AttemptID` generated; new lease granted |
| Events produced | `LeaseGranted`, `NodeStarted` |
| Failure outcomes | Same as `DispatchActivity` |

```
Fields: RunID, NodeID, NewAttemptID, OldAttemptID, Attempt int, Reason
```

### 4.3 `ScheduleTimer`

| Field | Value |
|-------|-------|
| Producer | Scheduler |
| Executor | CommandProcessor → TimerManager |
| Side effects | Timer inserted into heap; `TimerScheduled` event written |
| Events produced | `TimerScheduled` |
| Failure outcomes | Duplicate `TimerID`: rejected, no event written |

```
Fields: TimerID, RunID, NodeID, FireAt time.Time, Purpose TimerPurpose
```

### 4.4 `CancelTimer`

| Field | Value |
|-------|-------|
| Producer | Scheduler |
| Executor | CommandProcessor → TimerManager |
| Side effects | Timer removed from heap; `TimerCancelled` event written |
| Events produced | `TimerCancelled` |
| Failure outcomes | Timer already fired: no-op, no event written |

```
Fields: TimerID, Reason
```

### 4.5 `GrantLease`

| Field | Value |
|-------|-------|
| Producer | CommandProcessor (internal, as part of `DispatchActivity` execution) |
| Executor | CommandProcessor → LeaseMonitor |
| Side effects | Lease record created; LeaseMonitor begins tracking `ExpiresAt` |
| Events produced | `LeaseGranted` |
| Failure outcomes | N/A (internal) |

### 4.6 `RevokeLease`

| Field | Value |
|-------|-------|
| Producer | CommandProcessor (during `RevokeAndAbandon` cancellation) |
| Executor | CommandProcessor → LeaseMonitor |
| Side effects | Immediate `LeaseExpired` written for all active leases of the Run, bypassing `ExpiresAt` |
| Events produced | `LeaseExpired` for each active lease |
| Failure outcomes | N/A |

### 4.7 `NotifyCheckpoint`

| Field | Value |
|-------|-------|
| Producer | Scheduler |
| Executor | CommandProcessor → NotificationService |
| Side effects | Sends checkpoint notification to configured targets (Slack, webhook, email) |
| Events produced | None (notifications are fire-and-forget; delivery is not recorded in WAL) |
| Failure outcomes | Notification delivery failure is logged and metriced; does not affect Run state |

```
Fields: RunID, NodeID, Targets []NotificationTarget, Message, CheckpointDeadline
```

### 4.8 `TriggerCompensation`

| Field | Value |
|-------|-------|
| Producer | Scheduler |
| Executor | CommandProcessor → SagaCoordinator |
| Side effects | Writes `CompensationStarted`; SagaCoordinator begins reverse-order stack execution |
| Events produced | `CompensationStarted` |
| Failure outcomes | Empty stack: no-op, Run proceeds to failure terminal state directly |

```
Fields: RunID, Stack []CompensationEntry, Trigger CompensationTrigger, Reason
```

### 4.9 `FailWorkflow`

| Field | Value |
|-------|-------|
| Producer | Scheduler |
| Executor | CommandProcessor |
| Side effects | Writes `WorkflowFailed`; publishes failure event to subscribers |
| Events produced | `WorkflowFailed` |
| Failure outcomes | N/A (terminal) |

```
Fields: RunID, Trigger, TriggerNodeID, Reason
```

### 4.10 `CompleteWorkflow`

| Field | Value |
|-------|-------|
| Producer | Scheduler |
| Executor | CommandProcessor |
| Side effects | Writes `WorkflowCompleted`; publishes completion event to subscribers |
| Events produced | `WorkflowCompleted` |
| Failure outcomes | Precondition check: MUST NOT emit if any non-terminal nodes exist |

```
Fields: RunID, Output []byte
```

### 4.11 `PropagateCancel`

| Field | Value |
|-------|-------|
| Producer | Scheduler |
| Executor | CommandProcessor → child run Scheduler |
| Side effects | Issues cancellation request to child run; writes `CancellationRequested` in child run's WAL |
| Events produced | `CancellationRequested` in child run's WAL |
| Failure outcomes | Child run already terminal: no-op |

```
Fields: ParentRunID, ChildRunID, CancellationType
```

### 4.12 `RefreshMemory`

| Field | Value |
|-------|-------|
| Producer | Scheduler (when a `RefreshMemory` built-in node is dispatched) |
| Executor | CommandProcessor → MemoryStore |
| Side effects | Calls `MemoryStore.GetProfile()`; records result in WAL |
| Events produced | `MemoryRefreshed` |
| Failure outcomes | Memory store unavailable: node fails; retry policy applies |

```
Fields: RunID, NodeID, SpaceID, TaskDescription
```

### 4.13 `AdmitQueuedRun`

| Field | Value |
|-------|-------|
| Producer | Scheduler (when global capacity frees up) |
| Executor | CommandProcessor |
| Side effects | Writes `AdmissionAccepted`; Run transitions to `RUNNING` |
| Events produced | `AdmissionAccepted` |
| Failure outcomes | Run already cancelled or timed out: no-op |

### 4.14 `RejectQueuedRun`

| Field | Value |
|-------|-------|
| Producer | Scheduler (on explicit rejection or capacity exhaustion policy) |
| Executor | CommandProcessor |
| Side effects | Writes `AdmissionTimedOut` or `AdmissionRejected` |
| Events produced | `AdmissionTimedOut` or `AdmissionRejected` |
| Failure outcomes | N/A (terminal) |

---

## 5. Formal Node State Machine

### 5.1 State Definitions

| State | Type | Description |
|-------|------|-------------|
| `PENDING` | Transient | Node exists; dependencies not yet all `COMPLETED`/`SKIPPED` |
| `READY` | Transient | All dependencies satisfied; queued for dispatch |
| `RUNNING` | Transient | Dispatched to worker; lease held |
| `AWAITING_APPROVAL` | Transient | Checkpoint reached; no lease; awaiting human/automated approval |
| `COMPLETED` | Terminal | Worker completed successfully |
| `FAILED` | Terminal | Worker failed with retries exhausted, or non-retryable error |
| `SKIPPED` | Terminal | Skip condition evaluated true; node never ran |
| `TIMED_OUT` | Terminal | ExecutionDeadline or AbsoluteDeadline exceeded |
| `CANCELLED` | Terminal | Run cancelled while node was in progress or pending |
| `COMPENSATING` | Transient | Compensation handler for this node is executing |
| `COMPENSATED` | Terminal | Compensation handler completed |

**Terminal states:** `COMPLETED`, `FAILED`, `SKIPPED`, `TIMED_OUT`, `CANCELLED`, `COMPENSATED`

### 5.2 Transition Table

| Current State | Event | Next State | Guard / Notes |
|---------------|-------|------------|---------------|
| `PENDING` | `NodeReady` | `READY` | All deps in {COMPLETED, SKIPPED} |
| `PENDING` | `NodeSkipped` | `SKIPPED` | Skip condition evaluated true |
| `PENDING` | `CancellationRequested` | `CANCELLED` | Immediate; no dispatch |
| `READY` | `NodeStarted` | `RUNNING` | Lease granted; ExecutionDeadline and AbsoluteDeadline timers scheduled |
| `READY` | `CancellationRequested` | `CANCELLED` | Removed from dispatch queue |
| `RUNNING` | `NodeCompleted` | `COMPLETED` | Lease voided; spawn processed |
| `RUNNING` | `NodeFailed` | `FAILED` | Retries exhausted or non-retryable |
| `RUNNING` | `NodeTimedOut` | `TIMED_OUT` | ExecutionDeadline or AbsoluteDeadline fired |
| `RUNNING` | `NodeRetryScheduled` | `PENDING` | Retries remain; backoff timer scheduled |
| `RUNNING` | `CheckpointReached` | `AWAITING_APPROVAL` | Worker returned checkpoint; lease released |
| `RUNNING` | `NodeCancelled` (graceful) | `CANCELLED` | Worker acked within GracePeriod |
| `RUNNING` | `NodeCancelled` (revoke) | `CANCELLED` | Lease revoked; GracePeriod skipped |
| `AWAITING_APPROVAL` | `CheckpointApproved` | `READY` | ExecutionDeadline resumed with remaining budget; new dispatch |
| `AWAITING_APPROVAL` | `CheckpointRejected` | `FAILED` | Retry policy applies |
| `AWAITING_APPROVAL` | `NodeTimedOut` (checkpoint) | `TIMED_OUT` | CheckpointTimeout or AbsoluteDeadline fired |
| `AWAITING_APPROVAL` | `NodeTimedOut` (absolute) | `TIMED_OUT` | AbsoluteDeadline fired; overrides all |
| `AWAITING_APPROVAL` | `NodeCancelled` | `CANCELLED` | Immediate; no worker to signal |
| `FAILED` | `CompensationStarted` | `COMPENSATING` | Node has `compensate:` defined; not for FAILED itself — this is run-level |
| `COMPLETED` | `CompensationStarted` (stack entry) | `COMPENSATING` | Run-level compensation; this node is being unwound |
| `COMPENSATING` | `CompensationActionCompleted` | `COMPENSATED` | Compensation handler completed successfully |
| `COMPENSATING` | `CompensationActionFailed` (continue) | `COMPENSATED` | Policy: continue; recorded as partial |
| `COMPENSATING` | `CompensationActionFailed` (halt) | `COMPENSATING` | Stops; run enters COMPENSATION_FAILED |

### 5.3 Forbidden Transitions

| From | Event | Reason Forbidden |
|------|-------|-----------------|
| `COMPLETED` | Any `NodeStarted` | Terminal; node may not re-enter dispatch |
| `COMPLETED` | Any `NodeFailed` | Terminal |
| `SKIPPED` | Any | Terminal; no further transitions |
| `FAILED` | `NodeCompleted` | Terminal |
| `TIMED_OUT` | `NodeCompleted` | Terminal |
| `CANCELLED` | Any | Terminal |
| `COMPENSATED` | Any | Terminal |
| Any state | `NodeSkipped` | Skip is only valid from `PENDING` |
| `AWAITING_APPROVAL` | `NodeCompleted` | No lease held; worker cannot complete while awaiting approval |

### 5.4 Retry Semantics

Retries do not change a node's `NodeID` or logical identity. A retry creates a new `AttemptID`. From the state machine's perspective:

1. `NodeFailed` written for `AttemptID=N`
2. Retry policy: retries remain
3. `TimerScheduled{retry_backoff}` written
4. `TimerFired` → `NodeRetryScheduled` written → Node transitions back to `PENDING` with `Attempt=N+1`
5. `NodeReady` → `READY` → new `DispatchActivity` with `AttemptID=N+1`

The node state machine traverses `PENDING → READY → RUNNING` again. From an external observer's view, the node "retried." In the WAL, all attempts are present.

### 5.5 Skip Semantics

`SKIPPED` is not `COMPLETED(nil)`. Differences:

| Aspect | SKIPPED | COMPLETED(nil) |
|--------|---------|----------------|
| Worker ran | No | Yes |
| Output available | No | Yes (empty/nil) |
| Enters compensation stack | No | Yes (if `compensate:` defined) |
| Counts toward fan-out success | No (omitted) | Yes |
| Dependent nodes | May trigger if deps include SKIPPED | Normal |

Downstream nodes that declare a dependency on a SKIPPED node treat it as satisfied. The SKIPPED node produces no output; condition expressions on its output will see `nil`/absent values.

### 5.6 TIMED_OUT → FAILED Equivalence

`TIMED_OUT` is a terminal state for a node that is distinct from `FAILED` for observability. However, for downstream semantics (retry policy evaluation, compensation stack, error handler trigger, fan-out failure policy), `TIMED_OUT` is treated identically to `FAILED`.

The `NodeTimedOut` event's `DeadlineType` field distinguishes `execution` from `absolute` deadline expiry.

---

## 6. Formal Run State Machine

### 6.1 State Definitions

| State | Type | Description |
|-------|------|-------------|
| `ADMITTED_QUEUED` | Transient | Run accepted but waiting for engine capacity |
| `RUNNING` | Transient | Active execution; at least one node is non-terminal |
| `HANDLING_ERROR` | Transient | Error handler node is executing |
| `COMPENSATING` | Transient | Saga coordinator is unwinding completed nodes |
| `COMPLETED` | Terminal | All nodes terminal in {COMPLETED, SKIPPED}; none failed |
| `FAILED` | Terminal | Node failed; no handler; no compensation; or handler also failed |
| `FAILED_HANDLED` | Terminal | Node failed; error handler completed successfully |
| `COMPENSATED` | Terminal | All compensation actions succeeded |
| `COMPENSATION_PARTIAL` | Terminal | Compensation ran; some actions failed (continue policy) |
| `COMPENSATION_FAILED` | Terminal | Compensation halted due to action failure (halt policy) |
| `CANCELLED` | Terminal | Cancellation completed; no compensation was run |
| `CANCELLED_WITH_COMPENSATION` | Terminal | Cancellation completed; compensation was run |
| `ADMISSION_REJECTED` | Terminal | Admission queue timed out or explicitly rejected |

**Terminal states:** `COMPLETED`, `FAILED`, `FAILED_HANDLED`, `COMPENSATED`, `COMPENSATION_PARTIAL`, `COMPENSATION_FAILED`, `CANCELLED`, `CANCELLED_WITH_COMPENSATION`, `ADMISSION_REJECTED`

### 6.2 Transition Table

| Current State | Event | Next State | Guard / Notes |
|---------------|-------|------------|---------------|
| *(new)* | `WorkflowStarted` | `RUNNING` | Direct admission (capacity available) |
| *(new)* | `AdmissionQueued` | `ADMITTED_QUEUED` | Engine at capacity |
| `ADMITTED_QUEUED` | `AdmissionAccepted` | `RUNNING` | Capacity freed |
| `ADMITTED_QUEUED` | `AdmissionTimedOut` | `ADMISSION_REJECTED` | Deadline exceeded |
| `ADMITTED_QUEUED` | `CancellationRequested` | `CANCELLED` | Cancelled before admission |
| `RUNNING` | All nodes → {COMPLETED, SKIPPED} | `COMPLETED` | Via `WorkflowCompleted` event |
| `RUNNING` | Node `FAILED`/`TIMED_OUT`; error handler defined | `HANDLING_ERROR` | Via `HandlerStarted` event |
| `RUNNING` | Node `FAILED`/`TIMED_OUT`; no handler; compensation configured | `COMPENSATING` | Via `CompensationStarted` event |
| `RUNNING` | Node `FAILED`/`TIMED_OUT`; no handler; no compensation | `FAILED` | Via `WorkflowFailed` event |
| `RUNNING` | `CancellationRequested` | `RUNNING` | Cancellation propagation begins; Run remains RUNNING until all nodes terminal |
| `RUNNING` | All nodes cancelled; `CancellationCompleted{compensation=false}` | `CANCELLED` | |
| `RUNNING` | All nodes cancelled; compensation ran; `CancellationCompleted{compensation=true}` | `CANCELLED_WITH_COMPENSATION` | |
| `HANDLING_ERROR` | `HandlerCompleted` | `FAILED_HANDLED` | Via `HandlerCompleted` event |
| `HANDLING_ERROR` | `HandlerFailed`; compensation configured | `COMPENSATING` | Via `CompensationStarted` event |
| `HANDLING_ERROR` | `HandlerFailed`; no compensation | `FAILED` | Via `WorkflowFailed` event |
| `COMPENSATING` | `CompensationCompleted{failed=0}` | `COMPENSATED` | All actions succeeded |
| `COMPENSATING` | `CompensationCompleted{failed>0, halt=false}` | `COMPENSATION_PARTIAL` | Continue policy |
| `COMPENSATING` | `CompensationCompleted{halt=true}` | `COMPENSATION_FAILED` | Halt policy |

### 6.3 Forbidden Run Transitions

| From | To | Reason |
|------|----|----|
| Any terminal | Any | Terminal states are final |
| `COMPLETED` | `FAILED` | Once completed, cannot fail |
| `FAILED` | `COMPENSATING` | Compensation must be triggered before reaching FAILED |
| `COMPENSATED` | `FAILED` | Terminal; no re-entry |

### 6.4 Cancellation and Run State

`CancellationRequested` does not immediately change `RunState`. The Run remains in `RUNNING` (or `HANDLING_ERROR`) while nodes are being cancelled. The Run transitions to a terminal cancelled state only after `CancellationCompleted` is written, which requires all nodes to have reached terminal states.

---

## 7. Timer Semantics

### 7.1 Timer Types

| Timer Purpose | Scheduled by | Fires when | State transition caused | Cancellable |
|--------------|-------------|-----------|------------------------|-------------|
| `retry_backoff` | Scheduler (after `NodeFailed`/`NodeTimedOut` with retry) | Backoff interval elapses | Node: `PENDING` (re-queued) | No (removed on node cancellation) |
| `node_exec_deadline` | CommandProcessor (at `NodeStarted`) | Cumulative active execution time exceeded | Node: `RUNNING`/`AWAITING_APPROVAL` → `TIMED_OUT` | Yes (on node completion) |
| `node_abs_deadline` | CommandProcessor (at `NodeStarted`) | `nodeCreatedAt + AbsoluteDeadlineConfig` | Node: any non-terminal → `TIMED_OUT` | No |
| `checkpoint_timeout` | CommandProcessor (at `CheckpointReached`) | Checkpoint approval deadline | Node: `AWAITING_APPROVAL` → `TIMED_OUT` (or OnTimeout action) | Yes (on `CheckpointApproved`/`CheckpointRejected`) |
| `lease_expiry` | LeaseMonitor | `ExpiresAt` from `LeaseGranted` | Lease: expired → `LeaseExpired` written → node retry or fail | Yes (voided by `NodeCompleted`/`NodeFailed`) |
| `admission_timeout` | AdmissionMonitor | `AdmissionDeadline` from `AdmissionQueued` | Run: `ADMITTED_QUEUED` → `ADMISSION_REJECTED` | No |

### 7.2 MUST Rules for Timers

- Every timer MUST be recorded via `TimerScheduled` event before the in-memory timer is armed.
- `FireAt` MUST be an absolute UTC timestamp. Duration-based configurations (`"retry in 5s"`) MUST be converted to absolute time at scheduling time.
- `TimerFired` MUST be written to WAL before any state transition caused by the timer.
- The Scheduler MUST NOT contain expressions of the form `if time.Now().After(deadline)`. All such logic lives in TimerManager/LeaseMonitor.

### 7.3 Overdue Timers on Restart

When the engine restarts:

1. TimerManager reads all `TimerScheduled` events from WAL.
2. Removes those with a subsequent `TimerFired` or `TimerCancelled`.
3. For each remaining timer:
   - If `FireAt` is in the past: fire immediately (catch-up), set `Late: true` in `TimerFiredPayload`.
   - If `FireAt` is in the future: schedule for `FireAt - now`.

**Late timers are correct, not erroneous.** Downstream logic MUST handle `Late: true` gracefully. A late retry backoff fires and the retry proceeds normally. A late checkpoint timeout fires and the checkpoint is expired normally.

### 7.4 `node_exec_deadline` — Accounting During AWAITING_APPROVAL

The `node_exec_deadline` timer tracks cumulative active execution time (time spent in `RUNNING` state only).

- At `CheckpointReached`: `ExecutionDeadline` timer is cancelled; remaining budget recorded in `ExecutionState`.
- At `CheckpointApproved`: new `node_exec_deadline` timer scheduled for `now + remainingBudget`.
- If the remaining budget at approval time is zero or negative: `NodeTimedOut` is emitted immediately without dispatching.

### 7.5 `node_abs_deadline` — No Exceptions

The `node_abs_deadline` timer is scheduled once at `NodeStarted` time. It MUST NOT be cancelled by any approval, configuration, or RPC call. If it fires while the node is in `AWAITING_APPROVAL`, the checkpoint is immediately expired and `NodeTimedOut` is written. This is the safety valve against infinite-via-checkpoints patterns.

### 7.6 Timer and Cancellation Interaction

When a node is cancelled, the following timers MUST be cancelled (via `CancelTimer` commands):
- `node_exec_deadline` for that node
- `node_abs_deadline` — EXCEPTION: AbsoluteDeadline fires unconditionally. However, if the node is already in a terminal state when it fires, the event is a no-op.
- `checkpoint_timeout` for that node
- `retry_backoff` for that node (if pending retry timer exists)

`LeaseExpired` timers are superseded by `RevokeLease` on `RevokeAndAbandon`.

---

## 8. Lease & Worker Protocol Spec

### 8.1 Task Identity

Every dispatched task carries two identifiers:

```
OperationID = hex(sha256(RunID + ":" + NodeID)[:16])
AttemptID   = hex(sha256(RunID + ":" + NodeID + ":" + strconv.Itoa(Attempt))[:16])
```

| Identifier | Stable across retries | Use cases |
|-----------|----------------------|-----------|
| `OperationID` | Yes | External API idempotency keys (Stripe, GitHub, etc.) |
| `AttemptID` | No (changes per attempt) | Tracing span ID, task queue entry, lease tracking |

Workers MUST use `OperationID` when calling external systems that support idempotency keys. Workers MUST use `AttemptID` for trace correlation.

### 8.2 Lease Lifecycle

Default values:
- `LeaseDuration`: 5 minutes
- `MaxLeaseDuration`: 1 hour
- `MinRenewalInterval`: 30 seconds
- `LeaseRenewalDeadline`: `ExpiresAt - LeaseDuration/2` (worker should renew before this)

```
Engine writes LeaseGranted{AttemptID, WorkerID, GrantedAt, ExpiresAt}
Engine writes NodeStarted{AttemptID}
Engine sends Task to worker via PollTask stream

Worker executes activity:
  Worker MUST call RenewLease before ExpiresAt
  → Engine writes LeaseRenewed{AttemptID, NewExpiresAt}
  → NewExpiresAt = min(now + LeaseDuration, GrantedAt + MaxLeaseDuration)

Worker completes:
  Worker calls CompleteTask{AttemptID, Output}
  → Engine writes NodeCompleted; lease voided

Worker fails:
  Worker calls FailTask{AttemptID, ErrorCode, ErrorMessage}
  → Engine writes NodeFailed; lease voided

Lease expires (worker did not renew):
  LeaseMonitor writes LeaseExpired{AttemptID}
  → Engine applies retry policy
```

### 8.3 Protocol RPCs

#### `PollTask`

Worker polls for available tasks matching its declared capabilities.

- Request: `{WorkerID, ActivityTypes []string, VersionRanges []CapabilityVersion}`
- Response (streaming): `{Task}` messages as tasks become available
- Stream is terminated by the engine when: the Run is cancelled (`RUN_CANCELLED`), or the worker registration expires.
- Worker MUST handle stream termination and re-establish the poll.

#### `RenewLease`

- Request: `{AttemptID, WorkerID}`
- Response: `{NewExpiresAt}` on success
- Error codes:
  - `LEASE_EXPIRED`: lease already expired; worker MUST stop processing immediately
  - `RUN_CANCELLED`: run was cancelled; worker MUST stop processing immediately
  - `INVALID_ATTEMPT`: unknown `AttemptID`; worker MUST stop processing

#### `CompleteTask`

- Request: `{AttemptID, WorkerID, Output []byte, SpawnedNodes []NodeDefinition}`
- Response: `{}` on success
- Error codes:
  - `LEASE_EXPIRED`: worker's lease expired before result was received; result is discarded
  - `RUN_CANCELLED`: run was cancelled; result is discarded
  - `ATTEMPT_SUPERSEDED`: a later attempt has already completed this node (race condition)
  - `INVALID_STATE`: node is in a state that cannot accept completion (e.g., already COMPLETED)

#### `FailTask`

- Request: `{AttemptID, WorkerID, ErrorCode string, ErrorMessage string, Permanent bool}`
- Response: `{}` on success
- Error codes: same as `CompleteTask`

#### `Heartbeat`

- Request: `{WorkerID}`
- Response: `{}` on success
- Interval: MUST be sent every 10 seconds
- On heartbeat stop: engine treats worker as dead; all leases held by `WorkerID` are immediately expired (bulk `LeaseExpired` events)

### 8.4 Lease Renewal Race Resolution

The WAL has a single writer. All writes go through a serialization point.

**Race scenario:** LeaseMonitor submits `LeaseExpired` to WAL writer queue; simultaneously, worker sends `RenewLease` which also submits `LeaseRenewed` to WAL writer queue.

**Resolution:** whichever event is written first is authoritative.

- `LeaseRenewed` written first → lease is valid; subsequent `LeaseExpired` submission is dropped (LeaseMonitor checks WAL before writing).
- `LeaseExpired` written first → lease is dead; subsequent `RenewLease` RPC returns `LEASE_EXPIRED`.

There is no ambiguous outcome. The WAL serialization point eliminates the race.

### 8.5 Worker Death vs. Stuck Task

| Scenario | Detection | Action |
|----------|-----------|--------|
| Worker dead | Heartbeat stops for > 30s | All leases held by `WorkerID` immediately expired |
| Task stuck (worker alive) | Lease not renewed within `ExpiresAt` | Only that task's lease expires; other tasks unaffected |

### 8.6 Worker Obligations on LEASE_EXPIRED

When a worker receives `LEASE_EXPIRED` (from `RenewLease` or `CompleteTask`):
1. MUST stop processing the task immediately.
2. MUST NOT call `CompleteTask` or `FailTask` with the same `AttemptID` again.
3. SHOULD attempt to roll back any in-progress side effects if possible.
4. SHOULD continue heartbeating (other tasks may still be active).

### 8.7 Worker Obligations on RUN_CANCELLED

When a worker receives `RUN_CANCELLED` (via stream termination or error code):
1. MUST stop processing all tasks for the affected Run.
2. The `context.Context` passed to the activity handler is cancelled; workers MUST propagate context cancellation.
3. Workers that ignore context cancellation are out of spec. The engine cannot enforce this.

### 8.8 Activity Version Routing

Workers declare capability version ranges at registration:

```
WorkerCapability{ActivityType: "scout", MinVersion: "1.0.0", MaxVersion: "2.0.0"}
```

Workflow definitions pin required versions:

```
ActivityDefinition{Type: "scout", RequiredVersion: ">=1.2.0 <2.0.0"}
```

The engine MUST dispatch tasks only to workers whose `[MinVersion, MaxVersion)` range satisfies the `RequiredVersion` constraint.

If no compatible worker is available:
- Task is queued (not failed) up to `NoWorkerTimeout` (default: 10 minutes).
- After timeout: `NodeFailed{ErrorCode: NO_COMPATIBLE_WORKER}`.

### 8.9 Error Codes Reference

| Code | Meaning | Worker action |
|------|---------|--------------|
| `LEASE_EXPIRED` | Lease expired before result received | Stop; do not retry with same AttemptID |
| `RUN_CANCELLED` | Run was cancelled | Stop all tasks for this Run |
| `ATTEMPT_SUPERSEDED` | A later attempt already completed this node | Stop; no retry needed |
| `NO_COMPATIBLE_WORKER` | No worker satisfies version requirements | Worker-side: update capability registration |
| `INVALID_VERSION` | Worker capability version not parseable | Worker-side: fix registration |
| `INVALID_STATE` | Node is in a state that rejects this transition | Investigate; do not retry |
| `INVALID_ATTEMPT` | Unknown AttemptID | Investigate; do not retry |

---

## 9. Condition Evaluation Spec

### 9.1 When Conditions Are Evaluated

A node's skip condition is evaluated exactly once: when all of the node's dependencies have reached terminal states ({`COMPLETED`, `SKIPPED`}) and the node would otherwise transition to `READY`.

Conditions are NOT re-evaluated after `CheckpointApproved` or after a retry.

### 9.2 Evaluation Input

```go
type ConditionContext struct {
    // Output values from NodeCompleted payloads of dependency nodes.
    // Keys are NodeIDs. Value is nil for SKIPPED dependencies.
    DepsOutputs map[string]any

    // Memory profile locked at WorkflowStarted (StablePerRun / ManualRefresh).
    // OR the most recent MemoryRefreshed payload.
    // NEVER from the live memory store directly.
    Memory MemoryProfile

    // Static values from the workflow definition (immutable).
    Constants map[string]any
}
```

**Explicitly absent from `ConditionContext`:**
- `time.Now()` or any wall-clock access
- External service calls
- Live memory store queries
- Any state not materialized in WAL events at evaluation time

### 9.3 Evaluator Properties

The condition evaluator:
- MUST be a sandboxed expression interpreter with no capability to perform I/O.
- MUST have no side effects.
- MUST be deterministic: same `ConditionContext` input → same `bool` output.
- MUST return `(bool, error)`. An evaluator error MUST cause the node to fail (not skip).
- MUST NOT be able to call functions that access time, filesystem, network, or any other external resource.

### 9.4 Result Materialization

The condition result is materialized as either:
- `NodeSkipped` event (condition = true → skip)
- `NodeReady` event (condition = false → proceed)

Both events are written to WAL before the node's state changes.

### 9.5 Interaction with UnsafeRefreshOnBoundary

Even in `UnsafeRefreshOnBoundary` mode, conditions are evaluated against the **recorded** memory profile. Memory refresh at boundary points produces a `MemoryRefreshed` event in the WAL. The condition evaluator reads from that recorded event, not from the live memory store. Conditions are always closed over recorded state.

### 9.6 Modelling External-State Conditions

If a skip condition requires external state (e.g., a feature flag), the workflow author MUST model it as an activity:

1. Add an activity node that reads the external state and returns a boolean.
2. Declare the skip condition on the downstream node as `{{ deps.featureCheck.output.enabled == true }}`.

The engine does not provide a mechanism for conditions to access external state directly.

---

## 10. Cancellation Spec

### 10.1 Cancellation Types

| Type | Guaranteed Behavior | Not Guaranteed |
|------|---------------------|----------------|
| `GracefulCancel` | Signal workers; wait `GracePeriod`; then `RevokeAndAbandon` for remaining | Worker stops within GracePeriod |
| `RevokeAndAbandon` | Immediately expire all active leases; reject all future results | Worker process terminates; in-flight side effects rolled back |

### 10.2 GracefulCancel Sequence

```
1. CancellationRequested{type: graceful, graceDeadline: now + GracePeriod} written to WAL
2. PENDING nodes → CANCELLED immediately
3. READY nodes → CANCELLED immediately (removed from dispatch queue)
4. RUNNING nodes → worker notified via context cancellation on PollTask stream
5. AWAITING_APPROVAL nodes → CANCELLED immediately (no worker to signal)
6. After GracePeriod:
   - Nodes that completed gracefully → NodeCancelled{graceful: true}
   - Nodes still RUNNING → RevokeAndAbandon applied → NodeCancelled{graceful: false}
7. CancellationCompleted written when all nodes are terminal
```

### 10.3 RevokeAndAbandon Sequence

```
1. CancellationRequested{type: revoke_and_abandon} written to WAL
2. PENDING nodes → CANCELLED immediately
3. READY nodes → CANCELLED immediately
4. RUNNING nodes → LeaseExpired written immediately (bypassing ExpiresAt) → NodeCancelled
5. AWAITING_APPROVAL nodes → CANCELLED immediately
6. All subsequent CompleteTask / FailTask / RenewLease calls → rejected with RUN_CANCELLED
7. CancellationCompleted written when all nodes are terminal
```

### 10.4 What is Guaranteed

- All `PENDING` and `READY` nodes are cancelled before any worker is dispatched.
- All `AWAITING_APPROVAL` nodes are cancelled immediately.
- After `CancellationCompleted`, no new dispatches will occur.
- After `CancellationCompleted`, no worker results will be accepted.

### 10.5 What is NOT Guaranteed

- Workers stop executing within any time bound.
- In-progress side effects at cancellation time are rolled back.
- In-progress nodes at cancellation time are compensated (only COMPLETED nodes are in the compensation stack).

### 10.6 Fan-Out Under Cancellation

1. All active fan-out items receive the cancel signal.
2. Items completing within `GracePeriod` → `FanOutItemCompleted` recorded.
3. Items not completing → `FanOutItemCancelled` recorded.
4. Fan-out coordinator writes `FanOutCancelled{completedCount, cancelledCount}`.
5. Partial fan-out results from completed items before cancellation are discarded. Fan-out result is `CANCELLED`, not partial.

### 10.7 Child Workflow Cancellation

1. `PropagateCancel` command sent to child run.
2. Child run transitions through its own cancellation sequence.
3. Parent node enters `CANCELLING` transient sub-state, waiting for child terminal state.
4. Timeout: `ChildCancelTimeout` (default: 2× GracePeriod). If exceeded, child is force-cancelled (`RevokeAndAbandon`) and parent proceeds.
5. Parent sub-workflow node transitions to `CANCELLED` after child reaches terminal state.

### 10.8 Compensation Interaction

- Compensation on cancellation: opt-in via `CancellationConfig.RunCompensation = true`.
- If `RunCompensation = true`: after all RUNNING nodes cancel, the compensation stack is executed for previously COMPLETED nodes.
- Terminal states: `CANCELLED` (no compensation) or `CANCELLED_WITH_COMPENSATION` (compensation ran).
- In-progress nodes at cancellation time are EXCLUDED from the compensation stack regardless of `RunCompensation`.

### 10.9 Terminal Run States After Cancellation

| Scenario | Terminal State |
|----------|---------------|
| Cancelled; no compensation | `CANCELLED` |
| Cancelled; compensation ran | `CANCELLED_WITH_COMPENSATION` |
| Cancelled before admission | `CANCELLED` (from `ADMITTED_QUEUED`) |

---

## 11. Compensation Spec

### 11.1 What Enters the Compensation Stack

A node enters the compensation stack if and only if:
1. The node defines a `compensate:` handler in the workflow definition.
2. The node reached the `COMPLETED` state.

Conditions that PREVENT entry:
- Node reached `FAILED`, `SKIPPED`, `CANCELLED`, or `TIMED_OUT`
- Node has no `compensate:` defined
- Node is a built-in control node (fan-out coordinator, sub-workflow trigger node itself)

### 11.2 Stack Ordering

The compensation stack is ordered by completion time: the most recently COMPLETED node is at the top. Compensation executes in reverse order (top of stack first).

Fan-out entries: when a fan-out completes, each successfully completed item is added to the stack individually, in their completion order. The fan-out coordinator node itself is added last (most recently completed) and is thus compensated first.

### 11.3 Trigger Conditions

Compensation is triggered by any of the following:

| Trigger | Condition |
|---------|-----------|
| `node_failure` | Node retries exhausted; no error handler defined |
| `error_handler` | Error handler node failed with retries exhausted |
| `cancel` | `CancellationConfig.RunCompensation = true` and cancellation completed |
| `budget_exceeded` | Cost/token budget exceeded; `BudgetAction = Fail` |
| `deadline` | `MaxRunDuration` exceeded |

Compensation does NOT trigger on:
- Successful workflow completion
- Node cancelled before running
- Node SKIPPED

### 11.4 Compensation Retry Policy

Each compensation action has its own retry policy (separate from the original activity's retry policy). Default compensation retry policy: 3 attempts, 1s initial interval, 2× multiplier.

Compensation actions MUST be idempotent. The engine provides `"compensate:" + OperationID` as a derived idempotency key for compensation calls.

### 11.5 Compensation Failure Policy

| Policy | Behavior | Terminal State |
|--------|----------|---------------|
| `continue` (default) | Log failure; emit `CompensationActionFailed`; proceed to next entry | `COMPENSATION_PARTIAL` |
| `halt` | Stop immediately; emit `CompensationHalted`; escalate | `COMPENSATION_FAILED` |

### 11.6 Execution Mode

Default: sequential (top of stack first, one at a time).

Optional: parallel (`CompensationPolicy.Parallel = true`). When parallel, all stack entries execute concurrently. Failure handling still applies per-entry.

### 11.7 Terminal States After Compensation

Compensation is always terminal. After `CompensationCompleted`:

| Outcome | Terminal State |
|---------|---------------|
| All actions succeeded | `COMPENSATED` |
| Some actions failed (continue policy) | `COMPENSATION_PARTIAL` |
| Compensation halted (halt policy) | `COMPENSATION_FAILED` |

No state transition from any compensation terminal state is possible. There is no "compensate and retry from step N." A fresh run must be started if retry-after-compensation is required.

### 11.8 Interaction with Error Handler

If an error handler is defined:
1. Node fails → Run → `HANDLING_ERROR` → error handler runs.
2. Handler succeeds → `FAILED_HANDLED` (terminal). No compensation.
3. Handler fails → compensation triggered if configured; else `FAILED`.

The error handler is not part of the compensation stack. It is a recovery path. The Saga compensation stack is for catastrophic unwind.

### 11.9 Interaction with Cancellation

When cancellation with compensation is requested:
1. All non-terminal nodes are cancelled (no compensation for them).
2. Compensation stack is executed for nodes that reached `COMPLETED` before cancellation.
3. Nodes in `RUNNING` at cancellation time are ABANDONED, not compensated.

---

## 12. Deterministic Ordering Rules

### 12.1 Command Type Precedence

`Scheduler.Decide(state)` MUST produce commands in the following type order. Lower number = emitted first:

| Priority | Command Type | Rationale |
|----------|-------------|-----------|
| 0 | `CancelTimer` | Clean up before scheduling new timers |
| 1 | `TriggerCompensation` | Compensation before any new dispatch |
| 2 | `FailWorkflow` | Terminal signals before lifecycle changes |
| 3 | `CompleteWorkflow` | Terminal signals before lifecycle changes |
| 4 | `ScheduleTimer` | Schedule before dispatching dependents |
| 5 | `NotifyCheckpoint` | Notifications before dispatch |
| 6 | `PropagateCancel` | Propagation before dispatch |
| 7 | `DispatchActivity` | Actual work — lowest priority |
| 8 | `RequeueActivity` | Lowest priority |

### 12.2 `DispatchActivity` Ordering Within Priority Level

When multiple `DispatchActivity` commands are produced in the same `Decide()` call, they MUST be ordered by:

1. **Topological depth** ascending (shallower nodes first — they may unblock deeper nodes sooner).
2. **Creation sequence number** ascending (the WAL `Seq` of the event that added this node — FIFO within same depth).
3. **NodeID** lexicographic ascending (stable tie-breaker).

### 12.3 Spawned Node Insertion Ordering

Spawned nodes from a single `NodeCompleted{spawn}` event are inserted into the graph in the order they appear in the `SpawnedNodes` slice. Their creation sequence numbers are assigned in that order. Subsequent ordering follows §12.2.

### 12.4 Fan-Out Result Ordering

Fan-out result items are ordered by their original input index, not by completion order. `FanOutResult[i]` corresponds to `FanOutDirective.Items[i]`, regardless of which item completed first. This applies to both success and failure entries.

### 12.5 Ready Node Ordering

`Graph.ReadyNodes()` MUST return nodes ordered by:
1. Topological depth ascending.
2. Creation sequence number ascending.
3. NodeID lexicographic ascending.

This is the same ordering as §12.2 and ensures the Scheduler processes nodes consistently.

### 12.6 Implication for Testing

Because command ordering is fully specified, test assertions on command sequences MUST be deterministic. A conforming test that injects identical events MUST see identical command sequences. Flaky test assertions on command ordering indicate an ordering violation in the implementation.

---

## 13. Memory Semantics

### 13.1 Memory Profile Source

The memory profile available to a workflow run comes from one of two sources:

| Mode | Source | Recorded in WAL | Deterministic |
|------|--------|----------------|---------------|
| `StablePerRun` (default) | `MemoryStore.GetProfile()` called at `WorkflowStarted` time | Yes — embedded in `WorkflowStarted` payload | Yes |
| `ManualRefresh` | `MemoryStore.GetProfile()` called when `RefreshMemory` activity runs | Yes — embedded in `MemoryRefreshed` event | Yes |
| `UnsafeRefreshOnBoundary` | `MemoryStore.GetProfile()` called at declared boundary points | Yes — embedded in `MemoryRefreshed` event, with `Nondeterministic: true` flag | No |

### 13.2 Safe Boundary Points for UnsafeRefreshOnBoundary

Boundary points where memory refresh is permitted:
- Immediately after `CheckpointApproved` (memory store is in a well-defined state; no concurrent mutations from this run)
- At the start of a sub-workflow (child run gets its own fresh profile)

Boundary points where refresh MUST NOT occur:
- Mid fan-out (different branches would receive different profiles)
- During node execution (between `NodeStarted` and `NodeCompleted`)
- Between `NodeCompleted{spawn}` and the spawned nodes' first dispatch

### 13.3 Profile Stability Within a Run

Under `StablePerRun` and `ManualRefresh`, the profile injected into `StepContext.Memory` is:
- Frozen at `WorkflowStarted` (for `StablePerRun`).
- Updated only when a `RefreshMemory` node completes (for `ManualRefresh`).

Memories written by the current run are NOT visible to that same run. They take effect in the next run. This prevents mid-run context instability.

### 13.4 Embedding Degraded Mode

Memory entries have an `EmbeddingState` field:

| State | Meaning |
|-------|---------|
| `EmbeddingPending` | Entry written; embedding computation in progress |
| `EmbeddingReady` | Embedding available; cosine similarity search enabled |
| `EmbeddingFailed` | Embedding computation failed permanently |

During profile assembly (`GetProfile`), entries in `EmbeddingPending` state are eligible for BM25 text search (fallback) but excluded from cosine/HNSW semantic search. The profile MUST indicate which entries used BM25 fallback (via a flag on `ScoredMemory`).

### 13.5 What is Recorded in WAL

| Memory-related event | WAL record |
|---------------------|------------|
| Profile at run start | Full `MemoryProfile` embedded in `WorkflowStarted` payload |
| Manual refresh | Full `MemoryProfile` embedded in `MemoryRefreshed` event |
| Unsafe boundary refresh | Full `MemoryProfile` embedded in `MemoryRefreshed{Nondeterministic: true}` event |

Memory store mutations (new memories written by workers) are NOT recorded in the workflow's WAL. They are written to the memory store independently.

### 13.6 Determinism Guarantee Loss in UnsafeRefreshOnBoundary

Under `UnsafeRefreshOnBoundary`, replaying the same workflow run for debugging purposes may produce different agent behavior because:
- The memory store may contain new entries written between the original run and the replay.
- The agent may receive a different profile at boundary points.
- Spawn/fan-out decisions influenced by the profile may diverge.

This is declared and accepted. The run MUST carry observable markers indicating nondeterministic memory was used (see I-17 enforcement).

### 13.7 Required Markers for UnsafeRefreshOnBoundary Runs

The following MUST be applied to all runs using `UnsafeRefreshOnBoundary`:
- `WorkflowStarted` payload includes `NondeterministicMemory: true`.
- All OpenTelemetry spans carry tag `grael.nondeterministic_memory=true`.
- Metric `grael_nondeterministic_runs_total` is incremented.
- All projection modes display a visible indicator for this run.

---

## 14. Projection Semantics

### 14.1 Common Properties for All Projections

- All projections are computed from WAL + latest snapshot.
- Projections are never stored as primary state.
- No projection data may be used as input to the Scheduler or CommandProcessor.
- Projections are always consistent with the WAL tail at read time.

### 14.2 Compact Projection (`?view=compact`)

**Purpose:** Quick status for product UI, notifications, webhooks, and at-a-glance monitoring.

| Aspect | Behavior |
|--------|----------|
| Node representation | One entry per node with final state only |
| Retries | Collapsed: `{state, attempts: N}` |
| Fan-out | Collapsed: `FanOutSummary{total, succeeded, failed, skipped}` |
| SKIPPED nodes | Hidden by default; shown with `?include_skipped=true` |
| CANCELLED branches | Hidden |
| Individual attempts | Not shown |
| Timer events | Not shown |
| Lease events | Not shown |
| Intermediate states | Not shown |
| Memory degraded flag | Shown at run level only |

### 14.3 Operational Projection (`?view=operational`)

**Purpose:** Ops dashboard, incident response, SLA monitoring, and active debugging of in-progress runs.

| Aspect | Behavior |
|--------|----------|
| Node representation | Each active retry attempt shown separately; past attempts collapsed with summary |
| AWAITING_APPROVAL | Shown with remaining timeout countdown |
| Fan-out | Item summary with breakdown by state; individual completed items aggregated |
| Running cost | Total cost per node shown |
| Current lease holder | Shown for RUNNING nodes (WorkerID) |
| Completed fan-out items | Aggregate only (count breakdown) |
| Terminal historical attempts | Collapsed into summary |
| Timer events | Active timers shown (remaining time); fired/cancelled not shown |
| Memory degraded flag | Shown per-node and at run level |

### 14.4 Forensic Projection (`?view=forensic`)

**Purpose:** Post-mortem analysis, audit trails, compliance, and deep debugging.

| Aspect | Behavior |
|--------|----------|
| Node representation | All attempts shown individually with per-attempt durations and error details |
| Fan-out items | All items shown individually with per-item results |
| Timer events | All shown: scheduled, fired, cancelled with timestamps and `Late` flag |
| Lease events | All shown: grants, renewals, expirations with WorkerIDs |
| Memory events | All shown: lookups with scores, degraded flags, BM25 fallback indicators |
| Cancellation events | All shown: `CancellationRequested`, per-node `NodeCancelled`, `LeaseExpired` |
| Raw event timeline | Raw WAL events interleaved with node entries |
| Hidden data | Nothing |
| Collapsed data | Nothing |

### 14.5 What Is Never Hidden in Any Projection Mode

The following MUST be visible in all projection modes:
- Run terminal state
- Whether `NondeterministicMemory` was used
- Whether compensation ran
- Whether an error handler ran

---

## 15. Recovery & Crash Semantics

### 15.1 Recovery Algorithm

On startup, the engine performs the following for each active Run:

```
1. Find latest valid snapshot (CRC32 check); if none, use empty state.
2. Deserialize ExecutionState from snapshot.
3. Open WAL, seek to snapshot.Seq.
4. Apply all events from snapshot.Seq to current WAL tail (delta replay).
5. Verify ExecutionState integrity (node count, state consistency).
6. Hand off ExecutionState to Scheduler.
7. Scheduler enters live execution mode (WaitNext loop).
```

Delta replay is O(events since last snapshot), bounded by snapshot interval.

### 15.2 Crash Scenarios

#### Crash after `NodeStarted` before worker receives task

| Persisted facts | `LeaseGranted`, `NodeStarted` |
|-----------------|-------------------------------|
| Recovery action | After rehydration: node is in `RUNNING` state with active lease. Engine waits for lease expiry or worker result. |
| What is retried | Task is retried when lease expires (new `AttemptID`). |
| What cannot be guaranteed | Worker may have received the task via the network before the crash and begun executing. |

#### Crash after worker side effect, before `NodeCompleted` appended

| Persisted facts | `NodeStarted`, `LeaseGranted`; NO `NodeCompleted` |
|-----------------|--------------------------------------------------|
| Recovery action | Node appears `RUNNING` in rehydrated state. Engine waits for worker result or lease expiry. |
| What is retried | If worker reconnects: worker can `CompleteTask` again (same `AttemptID` still valid). If lease expires: new attempt. |
| What cannot be guaranteed | If worker executed a side effect (API call) but crashed before `CompleteTask`: the side effect may have occurred. Worker's `OperationID` idempotency key protects against double-execution on retry. |

#### Crash during snapshot write

| Persisted facts | WAL is intact (snapshot is written separately). |
|-----------------|-------------------------------|
| Recovery action | Invalid snapshot (failed CRC32) is ignored. Recover from previous valid snapshot + delta replay. |
| What is retried | Nothing — WAL is the source of truth. |
| Performance impact | Larger delta replay until next successful snapshot. |

#### Crash during `TimerScheduled` event write

| Persisted facts | `TimerScheduled` may or may not be written. |
|-----------------|----------------------------------------------|
| Recovery action | TimerManager scans WAL for `TimerScheduled` without subsequent `TimerFired`/`TimerCancelled`. Reschedules all pending timers. |
| What is retried | Timer is re-armed. If `FireAt` is in the past: fires immediately on startup. |

#### Crash during lease renewal race (LeaseMonitor writes `LeaseExpired`)

| Persisted facts | `LeaseExpired` may or may not be written. |
|-----------------|-------------------------------------------|
| Recovery action | LeaseMonitor rescans WAL on startup. If `LeaseExpired` not found, re-arms expiry timer. If found, applies retry policy. |
| What is retried | Lease is permanently dead once `LeaseExpired` is written. New attempt starts. |

#### Crash during `CancellationRequested` processing

| Persisted facts | `CancellationRequested` written; individual node cancellations may be partial. |
|-----------------|------------------------------------------------|
| Recovery action | Rehydration sees `CancellationRequested`. Scheduler re-issues `NodeCancelled` commands for nodes not yet cancelled. |
| What is retried | Cancellation propagation continues from where it stopped. |

#### Crash during compensation

| Persisted facts | `CompensationStarted` written; some `CompensationActionCompleted` may be written. |
|-----------------|------------------------------------------------|
| Recovery action | SagaCoordinator reconstructs stack from WAL. Resumes from first entry without a `CompensationActionCompleted` event. |
| What is retried | Compensation action is retried with same `OperationID`. Compensation actions MUST be idempotent. |

#### Crash during error handler execution

| Persisted facts | `HandlerStarted` written; handler node may be in mid-execution. |
|-----------------|------------------------------------------------|
| Recovery action | Rehydration sees `HandlerStarted` and handler node in `RUNNING`. Same as general crash recovery for a RUNNING node. |
| What is retried | Handler activity is retried when lease expires or worker reconnects. |

### 15.3 Snapshot Policy

| Trigger | Condition |
|---------|-----------|
| Event count | Every 100 events |
| Checkpoint | Every time a `CheckpointReached` event is written |
| Time-based | If no snapshot in last 10 minutes and run is active |

Snapshots are written **asynchronously**. The engine MUST NOT block execution waiting for snapshot completion. Snapshot write failure is logged and metriced; recovery from the previous snapshot + delta is always possible.

---

## 16. Illegal States and Rejected Sequences

The following sequences MUST be rejected. Rejection means the engine returns an error and does not write any event to the WAL.

### 16.1 Node-Level Illegal Sequences

| Sequence | Why Rejected |
|----------|-------------|
| `NodeCompleted` for a node with an expired lease (`AttemptID` matches `LeaseExpired`) | I-6: expired lease is permanently dead |
| `LeaseRenewed` after `LeaseExpired` for same `AttemptID` | I-6 |
| `NodeStarted` for a node already in `COMPLETED`, `FAILED`, `SKIPPED`, `CANCELLED`, `TIMED_OUT` | I-7: terminal states are final |
| `NodeSkipped` for a node not in `PENDING` | §5.3: skip only valid from PENDING |
| `CheckpointApproved` for a node not in `AWAITING_APPROVAL` | §5.2 |
| `CheckpointRejected` for a node not in `AWAITING_APPROVAL` | §5.2 |
| `NodeCompleted` for a node in `AWAITING_APPROVAL` | §5.3: no lease held during AWAITING_APPROVAL |
| Compensation entry for a `SKIPPED` node | I-11 |
| Compensation entry for a `FAILED` node | I-11 |
| Compensation entry for a `CANCELLED` node | I-11 |
| `NodeReady` for a node whose dependencies are not all in `{COMPLETED, SKIPPED}` | §5.2 |
| `TimerFired` after `TimerCancelled` for same `TimerID` | §3.4 |

### 16.2 Run-Level Illegal Sequences

| Sequence | Why Rejected |
|----------|-------------|
| `CompleteWorkflow` while any node is in a non-terminal state | A run cannot complete with in-flight nodes |
| `CompleteWorkflow` when any node is in `FAILED` state | Completion requires all nodes in {COMPLETED, SKIPPED} |
| Multiple terminal events for same `RunID` (e.g., both `WorkflowCompleted` and `WorkflowFailed`) | Run can have exactly one terminal state |
| `CancellationRequested` when Run is already in a terminal state | Terminal states are final |
| `HandlerStarted` when no error handler is configured in the workflow definition | No handler = no `HandlerStarted` event |
| `CompensationStarted` for a Run with an empty compensation stack | No-op, but should not produce a `CompensationStarted` event |
| `WorkflowStarted` with `UnsafeRefreshOnBoundary` policy and `AllowNondeterministicMemory: false` | I-17 |

### 16.3 Condition-Related Rejections

| Sequence | Why Rejected |
|----------|-------------|
| Condition expression accessing `time.Now()` | I-10: conditions are closed over WAL-recorded state |
| Condition expression calling an external service | I-10 |
| Condition expression reading live memory store directly | I-10 |
| Condition evaluated against a state not yet materialized in WAL | I-10 |

### 16.4 Graph Mutation Rejections

| Sequence | Why Rejected |
|----------|-------------|
| Spawned node declares dependency on a not-yet-existing node | I-15 |
| Spawn that would introduce a cycle | §1.1 graph invariants |
| Spawn that exceeds `MaxNodes` | Graph policy |
| Spawn that exceeds `MaxDepth` | Graph policy |
| Spawn that exceeds remaining `SpawnBudget` | Graph policy |
| Fan-out with `len(items) > MaxFanOutWidth` | Graph policy |

### 16.5 Archival-Related Rejections

| Sequence | Why Rejected |
|----------|-------------|
| Writing any event to a WAL that has been archived (read-only) | Archived WALs are immutable |
| Projecting an archived run without loading WAL from archive | Projection MUST be computed from WAL |

---

## 17. Conformance Test Matrix

A conforming Grael implementation MUST pass all tests in this matrix. Tests are specified as state + event sequence + expected outcomes; implementation language is not prescribed.

### 17.1 Happy Path

#### TC-HP-1: Linear workflow completion

| Field | Value |
|-------|-------|
| Initial state | Run `RUNNING`; nodes A→B→C (linear dependency) |
| Event sequence | A: `NodeReady`, `NodeStarted`, `NodeCompleted`; B: `NodeReady`, `NodeStarted`, `NodeCompleted`; C: same |
| Expected commands | Exactly one `DispatchActivity` per node; `CompleteWorkflow` after C completes |
| Expected terminal state | Run `COMPLETED` |

#### TC-HP-2: Parallel fan-out completion

| Field | Value |
|-------|-------|
| Initial state | Run `RUNNING`; node A spawns fan-out of 3 items |
| Event sequence | A `NodeCompleted{spawn: [B1, B2, B3]}`; all B nodes `NodeStarted`, `NodeCompleted` |
| Expected commands | 3 concurrent `DispatchActivity` commands in depth-then-seq order |
| Expected terminal state | Run `COMPLETED` |

#### TC-HP-3: Skip condition honored

| Field | Value |
|-------|-------|
| Initial state | Run `RUNNING`; node A→B where B has skip condition |
| Event sequence | A `NodeCompleted{output: {skip_b: true}}`; condition evaluates true |
| Expected commands | `NodeSkipped` for B; `CompleteWorkflow` |
| Expected terminal state | Run `COMPLETED`; B in `SKIPPED` |

---

### 17.2 Retry Tests

#### TC-RT-1: Transient failure retried

| Field | Value |
|-------|-------|
| Initial state | Node A in `RUNNING`; `MaxAttempts: 3`, `Attempt: 1` |
| Event sequence | `NodeFailed{AttemptID: A1, ErrorCode: TRANSIENT}` |
| Expected commands | `ScheduleTimer{purpose: retry_backoff}`; after `TimerFired`: `RequeueActivity` (new `AttemptID: A2`) |
| Expected terminal state | Node re-enters `PENDING` |

#### TC-RT-2: Non-retryable error fails immediately

| Field | Value |
|-------|-------|
| Initial state | Node A in `RUNNING`; `MaxAttempts: 3`, `Attempt: 1`; error code in `NonRetryable` list |
| Event sequence | `NodeFailed{ErrorCode: NON_RETRYABLE}` |
| Expected commands | No `ScheduleTimer`; directly `FailWorkflow` or `TriggerCompensation` |
| Expected terminal state | Node `FAILED`; Run `FAILED` or `COMPENSATING` |

#### TC-RT-3: Retries exhausted

| Field | Value |
|-------|-------|
| Initial state | Node A; `MaxAttempts: 3` |
| Event sequence | 3× `NodeFailed{TRANSIENT}` with backoff timers |
| Expected commands | After 3rd failure: no more `ScheduleTimer`; `FailWorkflow` or handler/compensation |
| Expected terminal state | Node `FAILED` |

---

### 17.3 Lease Expiry Tests

#### TC-LE-1: Lease expires; retry succeeds

| Field | Value |
|-------|-------|
| Initial state | Node A in `RUNNING`; `Attempt: 1`; retries remain |
| Event sequence | `LeaseExpired{AttemptID: A1}` |
| Expected commands | `RequeueActivity{NewAttemptID: A2}` |
| Expected terminal state | Node re-enters `RUNNING` with new lease |

#### TC-LE-2: Late CompleteTask after LeaseExpired rejected

| Field | Value |
|-------|-------|
| Initial state | Node A with expired lease `A1`; new attempt `A2` active |
| Event sequence | Worker sends `CompleteTask{AttemptID: A1}` |
| Expected response | Error: `LEASE_EXPIRED` |
| Expected WAL | No new events; node state unchanged |

#### TC-LE-3: Lease renewal race — expiry wins

| Field | Value |
|-------|-------|
| Initial state | Node A with lease `A1`; `LeaseExpired` written |
| Event sequence | Worker sends `RenewLease{AttemptID: A1}` after `LeaseExpired` is written |
| Expected response | Error: `LEASE_EXPIRED` |
| Expected WAL | No `LeaseRenewed` event |

#### TC-LE-4: Worker heartbeat stops; all leases expire

| Field | Value |
|-------|-------|
| Initial state | Worker W holds leases for nodes A, B, C; heartbeat stops |
| Event sequence | HeartbeatMonitor detects timeout |
| Expected commands | Bulk `RevokeLease` for A, B, C; `RequeueActivity` for each (if retries remain) |

---

### 17.4 Cancellation Tests

#### TC-CA-1: GracefulCancel, worker acks within GracePeriod

| Field | Value |
|-------|-------|
| Initial state | Node A in `RUNNING`; GracePeriod: 30s |
| Event sequence | `CancellationRequested{graceful}`; worker calls `FailTask{CANCELLED}` within 30s |
| Expected terminal state | Node `CANCELLED{graceful: true}`; Run `CANCELLED` |

#### TC-CA-2: RevokeAndAbandon, immediate

| Field | Value |
|-------|-------|
| Initial state | Nodes A (RUNNING), B (PENDING), C (AWAITING_APPROVAL) |
| Event sequence | `CancellationRequested{revoke_and_abandon}` |
| Expected commands | `RevokeLease` for A; immediate CANCELLED for B, C |
| Expected terminal state | Run `CANCELLED`; all nodes `CANCELLED` |

#### TC-CA-3: GracefulCancel with compensation

| Field | Value |
|-------|-------|
| Initial state | Node D COMPLETED (has `compensate:`); Node E RUNNING; `RunCompensation: true` |
| Event sequence | `CancellationRequested{graceful, run_compensation=true}`; E cancelled |
| Expected commands | After E cancelled: `TriggerCompensation` for stack [D] |
| Expected terminal state | Run `CANCELLED_WITH_COMPENSATION`; D `COMPENSATED` |

---

### 17.5 Checkpoint Tests

#### TC-CK-1: Checkpoint approved, execution resumes

| Field | Value |
|-------|-------|
| Initial state | Node A in `RUNNING` |
| Event sequence | `CheckpointReached`; `CheckpointApproved` |
| Expected commands | `CancelTimer{checkpoint_timeout}`; `DispatchActivity` (new `AttemptID`) |
| Expected terminal state | Node A re-enters `RUNNING` |

#### TC-CK-2: Checkpoint timeout

| Field | Value |
|-------|-------|
| Initial state | Node A in `AWAITING_APPROVAL` |
| Event sequence | `TimerFired{checkpoint_timeout}`; `OnTimeout: Fail` |
| Expected terminal state | Node A → `TIMED_OUT`; retry policy applies |

#### TC-CK-3: AbsoluteDeadline fires during AWAITING_APPROVAL

| Field | Value |
|-------|-------|
| Initial state | Node A in `AWAITING_APPROVAL`; AbsoluteDeadline timer active |
| Event sequence | `TimerFired{node_abs_deadline}` (fires while awaiting) |
| Expected terminal state | Node A → `TIMED_OUT` immediately; approval is no longer possible |

---

### 17.6 Compensation Tests

#### TC-CM-1: Node fails, compensation runs, succeeds

| Field | Value |
|-------|-------|
| Initial state | Nodes A (COMPLETED, has compensate), B (FAILED, retries exhausted) |
| Event sequence | `NodeFailed` for B with retries exhausted |
| Expected commands | `TriggerCompensation{stack: [A]}`; `DispatchActivity` for A's compensator |
| Expected terminal state | Run `COMPENSATED` |

#### TC-CM-2: Compensation action fails (continue policy)

| Field | Value |
|-------|-------|
| Initial state | Compensation running; stack [A, B] |
| Event sequence | A's compensator → `CompensationActionFailed`; policy: continue |
| Expected commands | Proceed to B's compensator dispatch |
| Expected terminal state | Run `COMPENSATION_PARTIAL` after both processed |

#### TC-CM-3: Compensation action fails (halt policy)

| Field | Value |
|-------|-------|
| Initial state | Compensation running; stack [A, B]; policy: halt |
| Event sequence | A's compensator → `CompensationActionFailed` |
| Expected commands | No further dispatch |
| Expected terminal state | Run `COMPENSATION_FAILED` |

---

### 17.7 Error Handler Tests

#### TC-EH-1: Node fails, handler succeeds

| Field | Value |
|-------|-------|
| Initial state | Run with error handler node configured; Node A `FAILED` |
| Event sequence | `HandlerStarted`; handler `NodeCompleted` |
| Expected terminal state | Run `FAILED_HANDLED` |

#### TC-EH-2: Handler itself fails

| Field | Value |
|-------|-------|
| Initial state | Run with error handler; handler node `FAILED` with retries exhausted |
| Event sequence | `HandlerFailed` |
| Expected terminal state | Run `FAILED` (no compensation) or `COMPENSATING` (if compensation configured) |

#### TC-EH-3: No handler configured — HandlerStarted not emitted

| Field | Value |
|-------|-------|
| Initial state | Run without error handler; Node A `FAILED` |
| Event sequence | Node A retries exhausted |
| Expected commands | No `HandlerStarted`; directly `FailWorkflow` or `TriggerCompensation` |

---

### 17.8 Unsafe Memory Mode Tests

#### TC-UM-1: StartWorkflow with UnsafeRefreshOnBoundary, no AllowNondeterministicMemory flag

| Field | Value |
|-------|-------|
| Initial state | — |
| Event sequence | `StartWorkflow` RPC with `MemoryPolicy: UnsafeRefreshOnBoundary`, `AllowNondeterministicMemory: false` |
| Expected response | Error: request rejected |
| Expected WAL | No events written |

#### TC-UM-2: UnsafeRefreshOnBoundary run carries markers

| Field | Value |
|-------|-------|
| Initial state | — |
| Event sequence | `StartWorkflow` with `UnsafeRefreshOnBoundary`, `AllowNondeterministicMemory: true` |
| Expected | `WorkflowStarted{NondeterministicMemory: true}`; metric `grael_nondeterministic_runs_total` incremented |

---

### 17.9 Admission Queue Tests

#### TC-AQ-1: Admission queue, then admitted

| Field | Value |
|-------|-------|
| Initial state | Engine at capacity |
| Event sequence | `StartWorkflow` → `AdmissionQueued`; capacity frees → `AdmissionAccepted` |
| Expected terminal state | Run transitions `ADMITTED_QUEUED` → `RUNNING` |

#### TC-AQ-2: Admission timeout

| Field | Value |
|-------|-------|
| Initial state | Run in `ADMITTED_QUEUED` |
| Event sequence | `AdmissionTimedOut` |
| Expected terminal state | Run `ADMISSION_REJECTED` |

---

### 17.10 Timer Recovery Tests

#### TC-TR-1: Engine restarts with overdue retry timer

| Field | Value |
|-------|-------|
| Initial state | WAL contains `TimerScheduled{retry_backoff, FireAt: T-10m}` without `TimerFired` |
| Event sequence | Engine restart |
| Expected | `TimerFired{Late: true}` written immediately on startup; node re-queued |

#### TC-TR-2: Engine restarts with future timer

| Field | Value |
|-------|-------|
| Initial state | WAL contains `TimerScheduled{FireAt: T+5m}` |
| Event sequence | Engine restart at T |
| Expected | Timer heap armed for `T+5m`; no immediate fire |

---

### 17.11 Duplicate External Events

#### TC-EE-1: Duplicate external event (same ExternalEventID)

| Field | Value |
|-------|-------|
| Initial state | `ExternalEventIngested{ExternalEventID: X}` already in dedup store |
| Event sequence | Second delivery of same event with `ExternalEventID: X` |
| Expected | Ack and drop; no new WAL event; no state change |

---

### 17.12 Deterministic Ordering Tests

#### TC-DO-1: Scheduler produces identical commands for identical state

| Field | Value |
|-------|-------|
| Initial state | ExecutionState S with 3 ready nodes at depths 1, 1, 2 |
| Event sequence | Call `Scheduler.Decide(S)` twice |
| Expected | Identical command sequences both times, in correct depth order |

#### TC-DO-2: CancelTimer emitted before DispatchActivity

| Field | Value |
|-------|-------|
| Initial state | Node A RUNNING (has timer); A completes; B becomes ready |
| Event sequence | `NodeCompleted` for A |
| Expected | `CancelTimer{A's exec deadline}` appears before `DispatchActivity{B}` in command list |

---

## 18. Naming Corrections

The following names from earlier architecture documents are replaced in this specification. All future code, API schemas, and documentation MUST use the names in the "Correct Name" column.

| Incorrect / Ambiguous Name | Correct Name | Reason |
|---------------------------|-------------|--------|
| `ImmediateCancel` | `RevokeAndAbandon` | "Immediate" implies process kill, which the engine cannot guarantee |
| `RefreshOnBoundary` | `UnsafeRefreshOnBoundary` | Silent nondeterminism is worse than an obvious name |
| `WAITING_APPROVAL` | `AWAITING_APPROVAL` | Grammatical consistency; also now a top-level state, not a sub-state |
| `Replay bool` in `StepContext` | (removed) | Replay awareness does not belong in the worker API |
| `ResultCache` | (removed) | Concept was architecturally wrong; rehydration handles this implicitly |
| "replay of orchestration logic" | (banned) | Orchestration replay in the Temporal sense does not exist in Grael; use "rehydration" |
| "NodeDeadline" (single) | `ExecutionDeadline` + `AbsoluteDeadline` | Two separate deadlines with different pause semantics |
| `HANDLER_COMPLETED` (run state) | `FAILED_HANDLED` | Clearly communicates that the run failed but was handled; more informative |
| `LeaseExpiry timer` (in-memory only) | `LeaseExpired` event (WAL) | Lease expiry is a persisted event, not an in-memory timer |
| "synthesize event from cache" | (banned) | Events are never synthesized; this phrase described the removed ResultCache pattern |
| `NodeFailed{reason: timeout}` variant | `NodeTimedOut` (separate event type) | Distinct state needed for observability; semantics are equivalent but type differs |
| "backoff interval" stored as duration | `FireAt` absolute timestamp | Absolute time eliminates drift on replay |

---

*End of Grael Runtime Specification v1.0-draft*

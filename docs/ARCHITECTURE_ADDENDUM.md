# Grael — Architecture Addendum

> Note
> Any memory-layer sections in this addendum are historical and superseded by the OmnethDB product split.
> They should not be treated as an active implementation plan for Grael.

This document is a second-pass refinement of `ARCHITECTURE.md`. It does not repeat what is already specified. It closes architectural gaps that remain after the first pass and would prevent Grael from being a correct, predictable, production-grade runtime.

---

## 1. Replay Transparency — Remove `Replay bool` from StepContext

### The Problem

Passing `Replay: bool` into `StepContext` is a leaky abstraction. Workers can branch on it, producing different behaviour during replay than during live execution. This breaks the determinism guarantee entirely.

### Solution: Memoization Layer Inside the Engine

The replay/live distinction is handled entirely inside `Executor.dispatch()`. Workers have no visibility into it.

```go
// ResultCache is built once during rehydration from NodeCompleted events.
// It is read-only during execution.
type ResultCache interface {
    Get(workflowID, nodeID string, attempt int) (*NodeCompletedPayload, bool)
}

func (e *Executor) dispatch(ctx context.Context, runID string, node ReadyNode) {
    cached, ok := e.resultCache.Get(runID, node.ID, node.Attempt)
    if ok {
        // Replay path: synthesize the event from the recorded result.
        // No worker contact. No I/O.
        e.wal.Append(runID, Event{
            Type:    NodeCompleted,
            Payload: mustEncode(cached),
        })
        return
    }

    // Live path: dispatch to worker as normal.
    e.workerPool.Submit(ctx, buildTask(runID, node))
}
```

**The deterministic boundary is `Executor.dispatch()`**. Everything above it (Scheduler, state machine, command production) is deterministic over the event log. Everything below it (worker execution) is not — but its outputs are recorded and replayed transparently.

`StepContext` loses the `Replay` field entirely. Workers never know.

---

## 2. Operation ID vs Attempt ID — Two Separate Identifiers

### The Problem

A single idempotency key that encodes the attempt number is wrong: external systems that support idempotency keys expect a **stable** key for the same logical operation across retries. Encoding the attempt makes every retry look like a new operation.

### Model

```go
type TaskIdentity struct {
    // Stable across all retries of this node.
    // Used as the idempotency key for external side effects.
    // Workers MUST pass this to external calls.
    OperationID string  // = hex(sha256(workflowID + ":" + nodeID)[:16])

    // Unique per attempt. Changes on every retry.
    // Used for: tracing spans, task queue IDs, lease tracking, log correlation.
    AttemptID string    // = hex(sha256(workflowID + ":" + nodeID + ":" + attempt)[:16])

    Attempt int
}
```

### Usage Rules

| Use case | Key to use | Why |
|----------|-----------|-----|
| External API idempotency (Stripe, GitHub) | `OperationID` | Same logical op across retries |
| Tracing span ID | `AttemptID` | Each attempt is a distinct span |
| Task queue entry | `AttemptID` | Queue must not confuse retry with original |
| Worker lease | `AttemptID` | Lease is per-attempt, not per-operation |
| Deduplication of worker result | `AttemptID` | Engine deduplicates per attempt |

### Responsibility

The engine injects both IDs into `Task`. Workers receive them and are responsible for using `OperationID` when calling external systems. This is a documented contract, not an engine enforcement. The engine cannot verify how workers use the key — this is a worker author responsibility.

**Trade-off considered:** encoding the `attempt` into `OperationID` would allow external systems to distinguish retries, but breaks idempotency entirely. Not viable.

---

## 3. Timer Model — First-Class Persisted Timers

### The Problem

Retry backoff intervals, node deadlines, checkpoint timeouts, and sleep steps all involve wall-clock time. If these are held only in-memory (Go tickers, channels), they are lost on restart. This silently breaks all time-dependent behaviour after a crash.

Wall-clock time must never be implicit.

### Timer as Persisted Event

Every timer is a first-class event in the WAL:

```go
type TimerPurpose string
const (
    TimerRetryBackoff      TimerPurpose = "retry_backoff"
    TimerNodeDeadline      TimerPurpose = "node_deadline"
    TimerCheckpointTimeout TimerPurpose = "checkpoint_timeout"
    TimerSleep             TimerPurpose = "sleep"   // explicit sleep step
)

// Events
type TimerScheduledPayload struct {
    TimerID    string
    NodeID     string
    FireAt     time.Time   // absolute UTC timestamp, not duration
    Purpose    TimerPurpose
}

type TimerFiredPayload struct {
    TimerID string
    FiredAt time.Time  // actual fire time (may be later than FireAt after downtime)
    Late    bool       // true if FiredAt > FireAt + LateTolerance
}

type TimerCancelledPayload struct {
    TimerID string
    Reason  string
}
```

`FireAt` is always an absolute timestamp. The engine converts `"retry in 5s"` to `now + 5s` at scheduling time and persists the absolute value. This eliminates drift on replay.

### Timer Manager

On startup, the Timer Manager rebuilds its state from the event log:
1. Collect all `TimerScheduled` events.
2. Remove those with a subsequent `TimerFired` or `TimerCancelled`.
3. For each remaining timer: if `FireAt` is in the past → fire immediately (catch-up). If in the future → schedule a Go timer for `FireAt - now`.

```go
type TimerManager struct {
    heap *timerHeap          // min-heap ordered by FireAt
    wal  WALWriter
    out  chan<- TimerFired    // feeds back into Scheduler event loop
}

func (tm *TimerManager) OnStartup(pending []PendingTimer) {
    now := time.Now().UTC()
    for _, t := range pending {
        if t.FireAt.Before(now) {
            tm.fire(t, now) // immediate catch-up
        } else {
            tm.heap.Push(t)
        }
    }
}
```

**Recovery semantic:** late timers fire immediately on restart, not at the original `FireAt`. The `Late: true` flag in `TimerFiredPayload` signals this to the Scheduler. Downstream logic (retry, checkpoint timeout) must handle late fires gracefully — they are correct, not errors.

**Determinism:** timer fire times are recorded in `TimerFiredPayload.FiredAt`. On replay, the engine does not re-fire timers. The `TimerFired` event is the authoritative record — its `FiredAt` is replayed from the log, not re-computed from `time.Now()`.

---

## 4. Command / Event Separation

### The Problem

Mixing decision logic with I/O makes the Scheduler untestable in isolation and creates implicit coupling between what the engine *decided* and what it *did*.

### Formal Model

```
WAL Events (immutable facts — what happened)
    │
    ▼
ExecutionState.Apply(event) → updated State (pure, no I/O)
    │
    ▼
Scheduler.Decide(state) → []Command (pure function, no I/O)
    │
    ▼
CommandProcessor.Execute(cmd) → side effects → emits new Events → WAL
```

The Scheduler is a **pure function**: `(State, Event) → []Command`. No goroutines. No clocks. No I/O. Fully unit-testable by feeding events and asserting commands.

### Command Catalogue

```go
type Command interface{ isCommand() }

// Execution
type DispatchActivity struct {
    WorkflowID  string
    NodeID      string
    Attempt     int
    TaskIdentity TaskIdentity
    Input       []byte // msgpack StepContext (sans Replay field)
}

type RequeueActivity struct {
    WorkflowID string
    NodeID     string
    OldAttemptID string
    Reason     string
}

// Timers
type ScheduleTimer struct {
    TimerID    string
    WorkflowID string
    NodeID     string
    FireAt     time.Time
    Purpose    TimerPurpose
}

type CancelTimer struct {
    TimerID string
    Reason  string
}

// Lifecycle
type CompleteWorkflow struct {
    WorkflowID string
    Output     []byte
}

type FailWorkflow struct {
    WorkflowID string
    Reason     string
    Trigger    FailTrigger // NodeFailed | BudgetExceeded | DeadlineExceeded | GraphViolation
}

type TriggerCompensation struct {
    WorkflowID string
    Stack      []CompensationEntry
    Reason     string
}

type NotifyCheckpoint struct {
    WorkflowID string
    NodeID     string
    Targets    []NotificationTarget
    Message    string
}

type PropagateCancel struct {
    WorkflowID string
    ChildRunID string
}
```

### Who Produces / Who Executes

| Command | Produced by | Executed by |
|---------|-------------|-------------|
| `DispatchActivity` | Scheduler | CommandProcessor → WorkerPool |
| `RequeueActivity` | Scheduler (on lease expiry / retry) | CommandProcessor → WorkerPool |
| `ScheduleTimer` | Scheduler | CommandProcessor → TimerManager |
| `CancelTimer` | Scheduler | CommandProcessor → TimerManager |
| `CompleteWorkflow` | Scheduler | CommandProcessor → WAL + notify |
| `FailWorkflow` | Scheduler | CommandProcessor → TriggerCompensation if needed |
| `TriggerCompensation` | Scheduler | CommandProcessor → SagaCoordinator |
| `NotifyCheckpoint` | Scheduler | CommandProcessor → NotificationService |
| `PropagateCancel` | Scheduler | CommandProcessor → child run Scheduler |

CommandProcessor is the only component allowed to perform I/O. It writes back to the WAL, which re-enters the Scheduler on the next iteration.

---

## 5. Scheduler / Executor Boundary — Strict Decomposition

### The Problem

The original spec had the Scheduler dispatching activities directly and the Executor as a vague concept. This conflates decision-making with I/O.

### Strict Component Ownership

```
Scheduler
  Owns: state machine, node lifecycle transitions, command production
  Does NOT: touch workers, timers, WAL (reads only), external systems
  Inputs: stream of Events (from WAL)
  Outputs: stream of Commands
  Test strategy: pure unit tests, no mocks needed

CommandProcessor
  Owns: executing commands produced by Scheduler
  Does NOT: make decisions, modify state
  Inputs: Commands from Scheduler
  Outputs: new Events appended to WAL
  Test strategy: integration tests with real WAL

WorkerPool
  Owns: task queue, worker assignment, lease management
  Does NOT: understand workflow semantics
  Inputs: DispatchActivity / RequeueActivity commands
  Outputs: TaskCompleted / TaskFailed events

TimerManager
  Owns: timer heap, fire scheduling, catch-up on startup
  Does NOT: understand what the timer means
  Inputs: ScheduleTimer / CancelTimer commands
  Outputs: TimerFired events

SagaCoordinator
  Owns: compensation stack, reverse-order execution
  Does NOT: decide when to compensate (Scheduler decides)
  Inputs: TriggerCompensation command
  Outputs: CompensationStarted / CompensationDone / CompensationFailed events
```

### State Machine Ownership

The node state machine lives entirely inside the Scheduler. State transitions happen only via `ExecutionState.Apply(event)`. No component other than the Scheduler calls `Apply`. No component other than the Scheduler reads `ExecutionState` during execution (CommandProcessor reads the commands it receives, not the state).

### Retry Ownership

Retry logic lives in the Scheduler. When `NodeFailed` arrives:
1. Scheduler checks retry policy: remaining attempts, backoff interval.
2. If retries remain: emit `ScheduleTimer{purpose: RetryBackoff, fireAt: now + backoff}`.
3. Timer fires → `TimerFired` event → Scheduler emits `RequeueActivity`.
4. If exhausted: transition node to `FAILED`, emit `TriggerCompensation` if needed.

The Scheduler never sleeps. It emits a timer command and waits for the timer event.

---

## 6. Cancellation — Formal Model

### Cancellation Types

```go
type CancellationType string
const (
    GracefulCancel  CancellationType = "graceful"   // signal + grace period
    ImmediateCancel CancellationType = "immediate"  // force-stop now
)
```

### State Transitions Under Cancellation

```
CancellationRequested event written to WAL

PENDING  → CANCELLED immediately (never dispatched)
READY    → CANCELLED immediately (remove from queue)
RUNNING  → signal worker via LeaseExpiry shortcut
             ├── worker acks within GracePeriod → write NodeCancelled{graceful: true}
             └── GracePeriod expires → write NodeCancelled{graceful: false}, lease revoked
AWAITING_APPROVAL → CANCELLED immediately (no worker to signal)
SKIPPED  → unchanged (already terminal)
COMPLETED → unchanged (already terminal)
FAILED   → unchanged (already terminal)
```

### Fan-out Under Cancellation

When a fan-out is RUNNING at cancellation time:
1. All active fan-out items receive the cancel signal (same mechanism as RUNNING node).
2. Items that complete within GracePeriod are recorded as `FanOutItemCompleted`.
3. Items that do not complete are recorded as `FanOutItemCancelled`.
4. Fan-out coordinator writes `FanOutCancelled{completedCount, cancelledCount}`.

Results from completed items before cancellation **are discarded** — the fan-out result is not partial; it is CANCELLED. Workflow authors must not rely on partial fan-out results.

### Child Workflow Cancellation

`PropagateCancel` command is sent to the child run. The child must reach a terminal state (CANCELLED, COMPLETED, COMPENSATED) before the parent marks the sub-workflow node as CANCELLED. The parent holds the node in `CANCELLING` (a transient sub-state) while waiting.

Timeout on child cancellation: `ChildCancelTimeout` (default: 2× GracePeriod). If exceeded, the child is force-cancelled and the parent proceeds.

### Compensation on Cancel

```go
type CancellationConfig struct {
    Type              CancellationType
    GracePeriod       time.Duration         // default: 30s
    RunCompensation   bool                  // default: false
    ChildCancelTimeout time.Duration        // default: 60s
}
```

If `RunCompensation: true`, after all RUNNING nodes reach CANCELLED, the Saga Coordinator runs the compensation stack for all previously COMPLETED nodes.

### Event Log

```
CancellationRequested { by, type, graceDeadline }
NodeCancelled         { nodeID, graceful bool }
FanOutCancelled       { nodeID, completedCount, cancelledCount }
CompensationStarted   { trigger: "cancellation" }
CancellationCompleted { compensated bool }
```

`CancellationCompleted` is the terminal event. After it, the run is in `CANCELLED` or `CANCELLED_WITH_COMPENSATION` state.

---

## 7. Task Lease Model

### The Problem

A heartbeat answers "is the worker alive?" A lease answers "does this worker still own this task?" These are different questions. A worker can be alive but have abandoned a task (deadlock, infinite loop). Without leases, stuck tasks are never reclaimed.

### Lease Protocol

```go
type TaskLease struct {
    TaskID      string         // = AttemptID
    WorkerID    string
    GrantedAt   time.Time
    ExpiresAt   time.Time      // = GrantedAt + LeaseDuration
    RenewCount  int
}

// Default lease duration: 5 minutes
// Max lease duration: 1 hour (prevents indefinite extension)
// Min renewal interval: 30 seconds
```

**Protocol:**

```
1. Engine writes LeaseGranted event → sends Task to worker
2. Worker executes activity
3. Worker calls RenewLease(taskID, workerID) before ExpiresAt
   → Engine writes LeaseRenewed{taskID, newExpiresAt}
   → newExpiresAt = min(now + LeaseDuration, GrantedAt + MaxLeaseDuration)
4a. Worker completes → CompleteTask → Engine writes NodeCompleted, lease becomes void
4b. Worker fails → FailTask → Engine writes NodeFailed, lease becomes void

On lease expiry (detected by LeaseExpiry timer):
  Engine writes LeaseExpired{taskID, workerID}
  → Increments attempt, checks retry policy
  → If retries remain: emit RequeueActivity (new lease, new AttemptID)
  → If exhausted: NodeFailed
```

**Distinguishing worker dead from task stuck:**

A worker sends heartbeat every 10s. If heartbeat stops → worker is dead → all its leases are expired immediately (bulk `LeaseExpired` events for all tasks held by that worker).

A worker sends heartbeat but doesn't renew a specific lease → that task is stuck. Other tasks on the same worker are unaffected.

**Worker's obligation:** renew the lease every `LeaseDuration / 2` at minimum. If the worker cannot renew (e.g., the task is blocked on a slow external call), it must still renew the lease with `RenewLease`. If the task cannot make progress and cannot renew, the worker should call `FailTask` proactively.

**Engine's guarantee:** the engine will not dispatch a task to a second worker while the lease is valid. After `LeaseExpired`, the original worker's `CompleteTask` call is rejected with `LEASE_EXPIRED` — its result is discarded.

---

## 8. Activity Versioning

### The Problem

Workflow versioning pins the workflow structure. But if the activity implementation behind `"scout"` changes incompatibly, pinned workflows can break silently.

### Recommended Model: Capability Version Ranges

```go
// Worker declares what versions it can handle
type WorkerCapability struct {
    ActivityType string  // e.g., "scout"
    MinVersion   string  // semver, inclusive, e.g., "1.0.0"
    MaxVersion   string  // semver, exclusive, e.g., "2.0.0"
}

// Workflow definition pins required activity versions
type ActivityDefinition struct {
    Type            string  // "scout"
    RequiredVersion string  // semver constraint, e.g., ">=1.2.0 <2.0.0"
}
```

The engine dispatches only to workers whose `[MinVersion, MaxVersion)` range satisfies the workflow's `RequiredVersion` constraint.

If no compatible worker is registered: task is queued (not failed) up to `NoWorkerTimeout` (default: 10 minutes). After timeout: `NodeFailed` with `ErrorCode: NO_COMPATIBLE_WORKER`.

This allows rolling worker upgrades without workflow restarts: deploy new workers (v1.3.0), old workers (v1.2.0) continue serving existing tasks, new tasks preferentially route to v1.3.0 workers.

**Alternatives considered:**

| Option | Pros | Cons |
|--------|------|------|
| Versioned names (`scout:v2`) | Simple, explicit | Name explosion, registry management |
| Capability ranges (recommended) | Flexible rolling upgrade, no name change | Slightly complex routing |
| Explicit contract versions | Clean governance | Requires central contract registry |
| Worker compatibility matrix | Full visibility | Engine must maintain the matrix, complex |

**Compatibility contract:** activity major version bump = breaking change = requires workflow version bump. Minor/patch bumps are backward-compatible by convention. This is a documented social contract between workflow authors and worker authors, not enforced by the engine.

---

## 9. Compensation — Formal Policy Model

### Trigger Conditions

```go
type CompensationTrigger string
const (
    CompensationOnNodeFailure    CompensationTrigger = "node_failure"     // retries exhausted, no error handler
    CompensationOnErrorHandler   CompensationTrigger = "error_handler"    // error handler itself failed
    CompensationOnCancel         CompensationTrigger = "cancel"           // CancellationConfig.RunCompensation=true
    CompensationOnBudgetExceeded CompensationTrigger = "budget_exceeded"  // BudgetAction=Fail
    CompensationOnDeadline       CompensationTrigger = "deadline"         // MaxRunDuration exceeded
)
```

Compensation does NOT trigger on:
- Node SKIPPED (never ran)
- Node CANCELLED before running
- Graceful workflow COMPLETED

### Compensation Stack

The stack is ordered: most recently COMPLETED node is at the top. Only nodes that reached COMPLETED (not FAILED, SKIPPED, CANCELLED) are pushed.

Fan-out adds entries per completed item, in completion order. When the fan-out itself is partial (some items CANCELLED), only the COMPLETED items are in the stack.

### Compensation Policy

```go
type CompensationPolicy struct {
    RetryPolicy    RetryPolicy     // applies to each compensation action
    OnFailure      CompensationFailAction
    Parallel       bool            // default: false (sequential, top of stack first)
    Timeout        time.Duration   // per-action timeout, default: 5 minutes
}

type CompensationFailAction string
const (
    CompensationContinue CompensationFailAction = "continue"
    CompensationHalt     CompensationFailAction = "halt"
)
```

`Continue`: log the failure, emit `CompensationActionFailed` event, proceed to the next entry. Best-effort. The workflow ends in `COMPENSATION_PARTIAL`.

`Halt`: stop compensation immediately, emit `CompensationHalted` event. The workflow ends in `COMPENSATION_FAILED`. An alert is sent to the configured escalation targets.

### Compensation Actions Must Be Idempotent

This is a hard requirement for workflow authors. The engine may retry a compensation action (up to `RetryPolicy.MaxAttempts`). The action receives the same `OperationID` as the original activity — it can derive `"compensate:" + OperationID` as its own idempotency key.

### Terminal State After Compensation

Compensation is always terminal. Possible terminal states after compensation:

| State | Meaning |
|-------|---------|
| `COMPENSATED` | All compensation actions completed successfully |
| `COMPENSATION_PARTIAL` | Some actions failed (policy: continue) |
| `COMPENSATION_FAILED` | Compensation halted (policy: halt) |

None of these states allow resuming the original workflow. There is no "compensate and retry from step N" in Grael. That pattern should be modelled as a new workflow run.

### Resume After Compensation Branch

Not supported. If a workflow author needs "compensate and retry", the correct model is:

```yaml
nodes:
  attempt:
    type: activity
    run: do-something
    on_failure: recovery

  recovery:
    type: activity
    run: undo-something     # inline compensation
    depends_on: [attempt]
    condition: "{{ nodes.attempt.status == 'failed' }}"

  retry:
    type: activity
    run: do-something-differently
    depends_on: [recovery]
```

The Saga pattern is for catastrophic unwind. Recovery paths are regular workflow nodes.

---

## 10. `AWAITING_APPROVAL` — Top-Level State

### The Problem

`WAITING_APPROVAL` as a sub-state of RUNNING is wrong:
- RUNNING implies a worker lease is held. A checkpoint holds no lease.
- Timeout semantics differ: node deadline vs checkpoint timeout.
- Metrics conflation: "running 4h" vs "awaiting approval 4h" are operationally different.

### Revised Node State Machine

```
PENDING
  │ all deps COMPLETED or SKIPPED
  ▼
READY
  │ dispatched to worker
  ▼
RUNNING ──────────── lease held ─────────────────────────────────────┐
  │                                                                   │
  │ checkpoint instruction returned in StepResult                    │
  ▼                                                                   │
AWAITING_APPROVAL ── no lease held                                   │
  │ CheckpointApproved                                               │
  ├──────────────────────────────────────────────────────────────────┘
  │                    (re-dispatches to worker, new lease)
  │ CheckpointRejected
  ├──► FAILED (apply retry policy)
  │ timeout
  └──► (apply OnTimeout action: Fail | AutoApprove | Escalate)

RUNNING / AWAITING_APPROVAL
  │ result returned
  ▼
COMPLETED ──► (spawn if any)
  │
  OR
  │ error, retries exhausted
  ▼
FAILED ──► (compensation stack, error handler)
  │
  OR SKIPPED (condition false) ──► (nothing)
  │
  OR TIMED_OUT (node deadline exceeded) ──► treated as FAILED
  │
  OR CANCELLED ──► (if compensation policy)
```

### Timeout Accounting

When a node transitions from RUNNING to AWAITING_APPROVAL:
- The node's execution timer (`NodeDeadline`) is **paused** (not cancelled).
- A new timer is scheduled: `CheckpointTimeout`.
- When the checkpoint is approved: `NodeDeadline` timer resumes with remaining time.
- If `NodeDeadline` was already exceeded before approval: the node fails immediately on approval.

This prevents a pattern where a checkpoint is used to "pause the clock" indefinitely.

### Metrics

```
grael_node_state{state="awaiting_approval"}   distinct gauge, not counted in "running"
grael_checkpoint_wait_duration_seconds        time from checkpoint reached to decision
grael_checkpoint_timeout_total               counter of timed-out checkpoints
```

---

## 11. Memory Profile Refresh Policy

### Policy Model

```go
type MemoryRefreshPolicy string
const (
    // Profile locked at WorkflowStarted. Same memory for entire run.
    // Strongest determinism. Default.
    StablePerRun MemoryRefreshPolicy = "stable_per_run"

    // Profile refreshed at explicit boundary points.
    // Weaker determinism: replay may see different memory if store changed.
    RefreshOnBoundary MemoryRefreshPolicy = "refresh_on_boundary"

    // Profile refreshed only when an explicit RefreshMemory activity runs.
    // Deterministic: refresh is recorded as an event (the new profile is in the log).
    ManualRefresh MemoryRefreshPolicy = "manual_refresh"
)
```

### Safe Boundary Points for `RefreshOnBoundary`

Safe (memory store is in a well-defined state, no concurrent mutation from this run):
- Immediately after `CheckpointApproved`
- At the start of a sub-workflow (child gets its own fresh profile)

Unsafe (must NOT refresh):
- Mid fan-out (different branches would get different profiles)
- During a node's execution
- Between a spawn and the spawned node's start

### `ManualRefresh` Semantics

```go
// RefreshMemory is a built-in activity type.
// When dispatched, it calls GetProfile, records the result as a NodeCompleted event.
// Subsequent nodes receive the new profile via StepContext.Memory.
// On replay: the recorded profile is used — GetProfile is NOT re-called.
type RefreshMemoryActivity struct {
    SpaceID string
    Query   string
}
```

`ManualRefresh` is the only policy that preserves full replay determinism while allowing mid-run profile updates. The profile at each point in the run is in the event log.

### Documentation Requirement

Workflows using `RefreshOnBoundary` must document this explicitly. Replay of such workflows may produce different profile contents if the memory store has changed since the original run. This is accepted and declared — not a silent inconsistency.

---

## 12. Async Embeddings — Degraded Memory Semantics

### Memory Entry States

```go
type EmbeddingState string
const (
    EmbeddingPending  EmbeddingState = "pending"   // queued for generation
    EmbeddingReady    EmbeddingState = "ready"      // generated, searchable
    EmbeddingFailed   EmbeddingState = "failed"     // permanent failure
)
```

### Recall Behaviour by State

| Entry state | Semantic search | Text fallback | Included in GetProfile |
|-------------|----------------|---------------|----------------------|
| `pending` | no | yes (BM25 over content) | yes, with degraded flag |
| `ready` | yes | yes | yes |
| `failed` | no | yes (BM25) | yes, with warning flag |

BM25 text search is the fallback — it operates over the raw `content` string stored in bbolt. It is correct but less precise than semantic search. `ScoredMemory.Score` is set to `0.0` for BM25 results to distinguish them from semantic results.

### Timeout and Permanent Failure

```go
type EmbeddingConfig struct {
    MaxGenerationTime time.Duration // default: 30s; after this → EmbeddingFailed
    RetryOnFailure    bool          // default: true, retried once after restart
}
```

`EmbeddingFailed` entries remain in the store and are returned via BM25. They are never promoted to semantic search unless explicitly re-triggered by an admin operation (`grael memory reembed <spaceID>`).

### Degraded State Signalling

```
Health endpoint:
  /healthz  → 200 always if engine is running (liveness)
  /readyz   → 200 when HNSW ready (or brute-force threshold not exceeded)
              503 during HNSW initial build

Metrics:
  grael_memory_embedding_pending_total{tenant, space}
  grael_memory_embedding_failed_total{tenant, space}
  grael_memory_index_state{state: "building|ready|degraded"}

GetProfile response includes:
  DegradedEntries int  // count of entries returned via BM25 fallback
```

If `DegradedEntries > 0`, the agent receives a note in the profile:
```
[Note: N memories are in degraded state and were retrieved via text search only.
 Semantic relevance may be reduced.]
```

---

## 13. External Event Guarantees — Honest Contract

### What Grael Guarantees

1. **Deduplication within TTL window**: an external event with a previously seen `externalEventID` arriving within `DeduplicationTTL` (default: 24h) will be silently dropped. The resulting workflow will not be double-triggered.

2. **Idempotent workflow creation**: `StartWorkflow` with the same deterministic `workflowRunID` (derived from trigger + input hash) returns the existing run. This protects against duplicate triggers even after dedup TTL expiry, for identical inputs.

3. **FIFO within a single source**: events from the same source (same webhook endpoint or same polling connection) are processed in arrival order.

4. **At-least-once processing**: every delivered external event is eventually processed (retried on failure until ack).

### What Grael Does NOT Guarantee

1. **Global ordering across sources**: no ordering guarantee between events from different sources (e.g., a GitHub webhook and a Linear webhook may arrive and be processed in any order).

2. **Ordering after dedup TTL expiry**: a re-delivered event after TTL expiry will be processed as a new event.

3. **Ordering relative to workflow internal state**: an external event arriving while a run is in progress may be processed before or after any specific node completes.

4. **Exactly-once processing with arbitrary external systems**: Grael's dedup is best-effort within the TTL window. External systems that are down during the dedup window may re-deliver.

### Design Guidance for Workflow Authors

Workflows sensitive to event ordering must:
1. Use a single inbound source (one webhook) and rely on FIFO within that source.
2. Implement ordering validation as an explicit activity (check sequence numbers against a persistent counter).
3. Not assume that two events from different sources that were causally related externally will arrive in causal order inside Grael.

---

## 14. Admission Control for StartWorkflow

### Synchronous Rejections (return error immediately, no run created)

| Condition | gRPC status | Notes |
|-----------|------------|-------|
| Workflow name/version not found | `NOT_FOUND` | |
| Input exceeds `MaxInputBytes` (default: 1MB) | `INVALID_ARGUMENT` | |
| Tenant not found or suspended | `NOT_FOUND` / `PERMISSION_DENIED` | |
| Minimum required budget not met | `RESOURCE_EXHAUSTED` | |
| Invalid workflow YAML structure | `INVALID_ARGUMENT` | |
| Authentication / RBAC failure | `UNAUTHENTICATED` / `PERMISSION_DENIED` | |

### Queued Admission (run created in PENDING state, not yet executing)

| Condition | Behaviour | Timeout |
|-----------|-----------|---------|
| No compatible workers registered | Queue, start when worker appears | `NoWorkerTimeout` (default: 10m) |
| Tenant concurrent run quota reached (policy: Queue) | Queue with position | `AdmissionQueueTimeout` |
| Global semaphore at capacity | Queue at engine level | `AdmissionQueueTimeout` |

Queued runs are visible via `GetRun` with state `ADMITTED_QUEUED`. They are not yet consuming resources.

### Rejection at Queue Depth

```go
type AdmissionConfig struct {
    QueueDepth        int           // per-tenant, default: 100
    OnQueueFull       QueueFullAction // Reject | ReplaceOldest
    AdmissionTimeout  time.Duration  // how long a queued run waits, default: 5m
    NoWorkerTimeout   time.Duration  // default: 10m
}
```

`ReplaceOldest`: evict the oldest queued run (transition it to `ADMISSION_REJECTED`) to make room. Notifies the evicted run's caller if a notification target was registered.

### Response Guarantees

`StartWorkflow` returns synchronously in all cases. It either:
- Returns a run in `RUNNING` state (workers available, quotas OK)
- Returns a run in `ADMITTED_QUEUED` state (will start when conditions are met)
- Returns a gRPC error (rejected, see table above)

There is no fire-and-forget. Every call gets a definitive response.

---

## 15. Retention, Archival, and Compaction

### Data Lifecycle Categories

```
Category                  Retention          Compactable?  Archive?
────────────────────────────────────────────────────────────────────
Active run WAL            until terminal     no            no
Completed run WAL         AuditRetention     no            yes (after retention)
Orphaned WAL (no run)     24h                yes           no
Old WAL segments          until no run refs  yes           no
Snapshots (latest)        indefinite         no            with WAL
Snapshots (older)         2 previous kept    yes           no
Memory entries (latest)   indefinite         no            no
Memory entries (forgotten) MemoryTombstone TTL yes         no
Memory relations          with entry         with entry    no
Dedup records             DeduplicationTTL   yes           no
Tenant data               until deletion     n/a           n/a
```

### Archival

Archival moves data to cold storage. The engine cannot replay from archived data without first restoring it. The engine provides `grael archive restore <runID>` to bring archived runs back for audit.

```go
type RetentionConfig struct {
    AuditRetentionDays     int    // default: 90
    MemoryTombstoneTTLDays int    // default: 30
    DeduplicationTTLHours  int    // default: 24
    ArchiveTarget          string // "local:/path" | "s3://bucket/prefix"
}
```

### What Cannot Be Deleted Without Losing Correctness

1. WAL segments for any run that has not reached a terminal state.
2. The snapshot for a run that is currently being replayed.
3. Memory entries with `IsLatest: true` — they may be marked `IsForgotten` (soft delete) but must not be hard-deleted while the version chain is active.
4. Dedup records within their TTL window.

### Purge Semantics for Tenant Deletion

Tenant deletion is a two-phase operation:

1. **Soft delete**: tenant marked as `deleted`, all new admission rejected, existing runs proceed to completion.
2. **Hard delete** (after `TenantDeletionGracePeriod`, default 30 days): cascade delete all WAL, memory, snapshots for the tenant.

Hard delete cannot be undone. The engine requires an explicit confirmation token (returned in the soft-delete response) to proceed with hard delete.

---

## 16. Error Handler — Formal Model

### A Single, Terminal Remediation Branch

```go
type ErrorHandlerConfig struct {
    Node        NodeDefinition
    CanSpawn    bool          // default: false; if true, handler may spawn remediation nodes
    RetryPolicy RetryPolicy   // for the handler node itself
}
```

### Rules — No Ambiguity

1. **Exactly one handler per workflow.** No nested handlers, no per-node handlers. Handlers are workflow-level.
2. **Terminal.** The handler is a remediation branch, not a recovery path. After the handler COMPLETES, the workflow enters `HANDLER_COMPLETED` — a distinct terminal state from `COMPLETED` and `FAILED`.
3. **Cannot resume the original path.** The handler receives the failed node's ID, the error, and the last output. It cannot mark nodes as re-runnable or alter the graph of the original execution.
4. **May spawn (if `CanSpawn: true`).** Spawned nodes exist only within the handler branch. They cannot depend on original workflow nodes that are in `FAILED` state. This allows multi-step remediation (e.g., rollback + notify + log).
5. **If handler fails:** workflow transitions to `HANDLER_FAILED`. No second-level handler. SagaCoordinator runs compensation for the original completed nodes (if compensation policy triggers on `HANDLER_FAILED`).
6. **Handler is not in the compensation stack.** If the handler completes but compensation is triggered (e.g., by subsequent cancel), the handler's actions are not compensated. The handler is responsible for ensuring its own actions are idempotent or self-compensating.

### Terminal State Summary

```
COMPLETED             all nodes completed successfully
HANDLER_COMPLETED     workflow failed, handler completed successfully
HANDLER_FAILED        workflow failed, handler also failed
FAILED                workflow failed, no handler defined
COMPENSATED           compensation completed fully
COMPENSATION_PARTIAL  compensation completed with failures (policy: continue)
COMPENSATION_FAILED   compensation halted (policy: halt)
CANCELLED             run cancelled
CANCELLED_WITH_COMPENSATION  run cancelled, compensation completed
```

---

## 17. Policy Precedence Model

### Precedence Levels (Highest to Lowest)

```
Level 5: Node policy      (defined on individual node in workflow YAML)
Level 4: Run override     (passed in StartWorkflowRequest.PolicyOverrides)
Level 3: Workflow policy  (defined in workflow definition)
Level 2: Tenant policy    (configured per-tenant in engine admin)
Level 1: Global default   (hardcoded engine defaults, always present)
```

### Resolution Rules — Two Categories

**Resource limits (strictest bound wins):**
```go
func resolveLimit(levels ...OptionalInt) int {
    return min(nonNilValues(levels)...)
}
// Applies to: MaxNodes, MaxDepth, MaxFanOutWidth, MaxConcurrency,
//             CostBudget.MaxTotalUSD, MaxRunDuration, MaxRetries
```

The tenant cannot override the global max. A node cannot exceed the workflow max. A run override cannot loosen the tenant limit.

**Behaviour policies (most specific wins):**
```go
func resolvePolicy[T any](levels ...Optional[T]) T {
    for _, level := range []Optional[T]{node, runOverride, workflow, tenant, global} {
        if level.IsSet() { return level.Value() }
    }
    panic("missing global default") // must not happen
}
// Applies to: RetryPolicy, ErrorHandlerConfig, CompensationPolicy,
//             MemoryRefreshPolicy, CheckpointConfig.OnTimeout
```

The most specific (innermost) level that defines the policy wins. If a node defines its own `RetryPolicy`, the workflow-level policy does not apply to that node.

### Explicit Conflict Documentation

When effective policies are computed at run start, the engine records them in `WorkflowStarted` payload. This makes the effective policy visible in the audit log — there is no ambiguity about which level was applied.

```go
type WorkflowStartedPayload struct {
    // ...
    EffectivePolicies EffectivePolicies  // computed and recorded at start
}
```

---

## 18. SKIPPED Semantics — Precise Definitions

**1. Does SKIPPED unblock dependents?**
Yes. A node with dependencies that are all in `{COMPLETED, SKIPPED}` transitions to READY. SKIPPED is treated as a satisfied dependency.

**2. Is SKIPPED different from COMPLETED with empty output?**
Yes. They are semantically distinct:
- `SKIPPED`: condition evaluated to false. Node never ran. Output is `nil` with no `Output` field.
- `COMPLETED(output=nil)`: node ran and returned nil. Output is `nil` with an `Output` field present (zero value).

Downstream nodes that read outputs from deps must handle both. If a node reads `{{ deps.nodeA.output }}` and nodeA was SKIPPED, the value is absent (not nil). Workflow authors must handle this explicitly.

**3. Can a SKIPPED node spawn?**
No. Spawn is a return value from node execution. SKIPPED nodes never execute.

**4. Compensation.**
SKIPPED nodes are never added to the compensation stack. Nothing ran, nothing to undo.

**5. Fan-out item SKIPPED.**
If a fan-out step's `condition` evaluates to false for a specific item, that item is SKIPPED. `FanOutResult.Results[i]` is nil with `FanOutResult.Skipped[i] = true`. The reduce step receives the full result including skip indicators.

**6. Metrics.**
```
grael_node_completed_total{status="skipped"}   tracked separately, not in "completed"
```

---

## 19. Execution History vs Materialized Graph View

### Two Separate Representations

**Execution History** (`GET /runs/{id}/events`):
- Raw append-only event stream from WAL
- Every event, every retry attempt, every timer fire, every lease expiry
- Immutable, authoritative, never summarised
- Used for: audit, debugging, replay, compliance

**Materialized Graph View** (`GET /runs/{id}/graph`):
- Derived on-demand from execution history
- Collapses retries: one node entry with `Attempts: 3`, not three entries
- Hides cancelled/abandoned fan-out items: shows aggregate (`FanOutSummary{total: 10, succeeded: 9, failed: 1}`)
- Hides CANCELLED branches from SKIPPED paths
- Shows current effective state of each node

```go
type MaterializedNode struct {
    ID             string
    Type           string
    State          NodeState
    Attempts       int
    FirstStartedAt time.Time
    LastStartedAt  time.Time
    CompletedAt    *time.Time
    Duration       time.Duration   // sum of active execution time, excluding AWAITING_APPROVAL
    Output         *any
    Error          *NodeError      // most recent error
    FanOut         *FanOutSummary  // nil for non-fan-out nodes
    Spawned        []string        // IDs of spawned nodes
}

type FanOutSummary struct {
    Total     int
    Succeeded int
    Failed    int
    Skipped   int
    Cancelled int
}
```

### Non-Negotiable Rule

The materialized view is **always derived, never stored as primary state**. The event log is the single source of truth. The materialized view is a projection over it.

If the materialized view and the event log disagree, the event log wins. The materialized view is regenerated.

---

## 20. Memory HNSW Startup — Degraded Mode Strategy

### Startup Modes

```go
type MemoryIndexMode string
const (
    EagerBuild    MemoryIndexMode = "eager"      // block startup until HNSW ready
    LazyBuild     MemoryIndexMode = "lazy"       // start accepting requests immediately, build in background
    BruteForce    MemoryIndexMode = "brute"      // never build HNSW, always brute-force
)
```

**Recommended: `LazyBuild`.**

`EagerBuild` is only appropriate if HNSW rebuild takes < 5s (small datasets). Beyond that, it creates unacceptable cold-start latency. Not recommended for production.

`BruteForce` is appropriate for development or deployments with < 10k memories per space. Operationally simple.

### Startup Sequence for `LazyBuild`

```
1. Engine starts. MemoryStore opens bbolt. Index state: BUILDING.
2. /healthz → 200 (engine is running)
3. /readyz → 503 with Retry-After header (index not ready)
4. Background goroutine: iterate bbolt embeddings bucket, insert into HNSW
   → progress: grael_memory_index_build_progress gauge (0.0 → 1.0)
5. During build:
   → Recall uses brute-force over the same bbolt embeddings (correct, O(n))
   → GetProfile works normally (via brute-force)
   → Remember writes to bbolt; entry is also inserted into in-progress HNSW build
     (incremental: new entries are visible to both brute-force and HNSW once built)
6. HNSW build complete. Index state: READY.
7. /readyz → 200
8. Subsequent Recall uses HNSW (O(log n))
```

### Rebuild on Restart

HNSW is in-memory only. It is rebuilt from bbolt on every restart. This is intentional:
- Avoids HNSW persistence format versioning
- Avoids corruption of a persisted index
- bbolt is the source of truth; HNSW is an acceleration structure

Rebuild time: ~2s for 100k vectors on modern hardware. Acceptable for `LazyBuild`.

### Correctness During Build

Entries written to bbolt during HNSW build are inserted into the in-progress index concurrently. This is safe because HNSW supports concurrent incremental insertion (`coder/hnsw` supports this). No entries are lost or missed.

### Metrics and Health

```
grael_memory_index_state          gauge {state: "building|ready|degraded"}
grael_memory_index_build_seconds  histogram (build duration)
grael_memory_index_size           gauge (entries in HNSW)
grael_memory_recall_mode          counter {mode: "hnsw|brute_force"} (per query)
```

`degraded` state = HNSW could not be built (OOM, corrupt bbolt). Engine falls back to permanent brute-force and emits alert.

---

## Delta from Current ARCHITECTURE.md

| # | What changed | Why it matters |
|---|-------------|----------------|
| 1 | Removed `Replay bool` from StepContext; replay handled by ResultCache inside Executor | Workers could branch on replay flag, breaking determinism |
| 2 | Split idempotency key into `OperationID` (stable) + `AttemptID` (per-attempt) | External systems need stable key across retries; tracing needs unique per-attempt |
| 3 | Timers as first-class persisted events (TimerScheduled/Fired/Cancelled); absolute timestamps only | In-memory timers lost on crash; relative durations accumulate drift on replay |
| 4 | Introduced Command/Event separation; Scheduler is a pure function | Makes Scheduler fully unit-testable; eliminates I/O from decision logic |
| 5 | Strict Scheduler/Executor/WorkerPool/TimerManager/SagaCoordinator decomposition | Prevents logic from bleeding across components; each component is independently testable |
| 6 | Formal cancellation state model; CANCELLING transient state; explicit fan-out cancel semantics | Original model had no formal semantics for cancellation propagation |
| 7 | Task lease model (LeaseGranted/Renewed/Expired); distinct from worker heartbeat | Heartbeat proves worker alive; lease proves task is making progress — different concerns |
| 8 | Activity versioning via capability version ranges; task queued (not failed) on no compatible worker | Silent dispatch to wrong version breaks pinned workflows |
| 9 | Compensation: formal trigger list, CompensationFailAction, fan-out partial stack, terminal-only | "Best-effort compensation" is ambiguous without formal policy |
| 10 | AWAITING_APPROVAL is top-level state, not RUNNING sub-state; node deadline paused during approval | RUNNING implies active work and held lease; neither is true for a checkpoint |
| 11 | Memory profile refresh as explicit policy (stable_per_run / refresh_on_boundary / manual_refresh) | Hard stable-per-run was too rigid; manual_refresh preserves determinism with mid-run updates |
| 12 | Memory embedding states (pending/ready/failed); BM25 fallback; DegradedEntries in profile | Silent "memory not searchable" is worse than a degraded-but-honest search |
| 13 | External event guarantees written as explicit contract (guarantees and non-guarantees) | Optimistic ordering claims would mislead workflow authors |
| 14 | Admission control: synchronous reject vs queued (ADMITTED_QUEUED state) | Original had no model for "no workers available at start" |
| 15 | Retention/archival/compaction policy; what cannot be deleted; tenant deletion two-phase | Missing lifecycle policy leads to unbounded storage growth |
| 16 | Error handler: single terminal branch, no nested handlers, HANDLER_COMPLETED terminal state | Allowing handler recovery paths creates a second, inconsistent execution model |
| 17 | Formal policy precedence: strictest-bound-wins for limits, most-specific-wins for behaviour | Implicit precedence causes surprises; recorded in WorkflowStarted for audit |
| 18 | SKIPPED: precise dep-unblocking, distinction from COMPLETED(nil), no spawn, no compensation | Underdefined SKIPPED semantics cause subtle bugs in workflows that branch on skip |
| 19 | Materialized graph view separated from execution history; view is always derived, never stored | Storing derived state creates consistency split; event log is sole source of truth |
| 20 | HNSW lazy build with brute-force fallback; rebuild from bbolt on restart; correctness during build | Blocking startup on HNSW rebuild is unacceptable; persisted HNSW creates versioning problems |

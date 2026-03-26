# Grael — Final Semantic Corrections

Precision pass on correctness edge cases. Does not repeat prior specifications.
Corrects specific semantic errors and ambiguities in `ARCHITECTURE.md` and `ARCHITECTURE_ADDENDUM.md`.

---

## Part I — Core Invariants

These invariants are absolute. Any implementation that violates them is incorrect.

### I-1: Rehydration Never Writes Events

```
Rehydration MUST NOT append any event to the WAL.
Rehydration MUST NOT emit any Command.
Rehydration MUST NOT contact any worker.
Rehydration is a read-only operation that produces ExecutionState.
```

### I-2: Time Never Directly Causes State Transitions

```
No workflow state transition may be caused by observing wall-clock time.
Every state transition is caused by Apply(event) where event ∈ WAL.
The only entities authorized to observe wall-clock time and write to WAL are:
  - TimerManager (writes TimerFired)
  - LeaseMonitor  (writes LeaseExpired)
  - AdmissionMonitor (writes AdmissionTimedOut)
The Scheduler has no access to time.Now().
```

Formally: `∀ transition T: ∃ event e ∈ WAL such that T = Apply(e)`.

### I-3: Expired Lease Is Permanently Dead

```
Once LeaseExpired is written to the WAL for a given (taskID, attemptID),
no subsequent RenewLease or CompleteTask for that (taskID, attemptID) is valid.
The expired lease cannot be revived.
Late results from the original worker are discarded unconditionally.
```

### I-4: Condition Evaluation Is Closed Over the Event Log

```
A node's skip condition must be a pure, deterministic function of:
  - outputs recorded in dependency NodeCompleted events
  - the memory profile recorded in WorkflowStarted (or last ManualRefresh NodeCompleted)
  - static values from the workflow definition

A condition MUST NOT access:
  - wall-clock time
  - external services
  - the live memory store
  - any state not materialized in the WAL

Violation: if an author needs external-state-dependent conditions,
they MUST model them as an activity whose boolean output the condition reads.
```

### I-5: Command Ordering Is Deterministic and Stable

```
Given the same ExecutionState, Scheduler.Decide() MUST always produce
the same []Command in the same order.
```

### I-6: ImmediateCancel Is Lease Revocation, Not Process Kill

```
ImmediateCancel revokes all active leases and stops acknowledging worker results.
It does NOT and CANNOT terminate worker processes or interrupt blocking I/O.
Workers may continue executing for an unbounded duration after ImmediateCancel.
Their results will be rejected.
```

---

## Part II — Semantic Corrections

### Correction 1: Remove ResultCache from Executor — It Is Wrong

**What was written:** `Executor.dispatch()` checks a `ResultCache`; on a cache hit (replay path), it synthesizes and appends a `NodeCompleted` event to the WAL.

**Why this is wrong:**

If `NodeCompleted` is already in the WAL, then during rehydration `ExecutionState.Apply(NodeCompleted)` already marked that node as `COMPLETED`. The Scheduler's `ReadyNodes()` only returns nodes in `PENDING` state whose dependencies are all `COMPLETED|SKIPPED`. A node that is already `COMPLETED` will never appear in `ReadyNodes()`. It never reaches `dispatch()`.

There is no replay path in `dispatch()`. The concept is unnecessary.

**Corrected model:**

```
Rehydration applies all WAL events → ExecutionState reflects exact current state.
Scheduler sees only nodes that have NOT yet been completed (they are PENDING).
Dispatcher contacts workers only for live, never-dispatched activities.

The only case requiring special handling on restart:
  - A NodeStarted event exists but no NodeCompleted/NodeFailed follows it.
  - The node was dispatched but the result was not recorded (crash mid-flight).
  - Action: check lease status. If lease expired → LeaseExpired event → retry.
            If lease not expired → wait for the original worker to complete or expire.
```

There is no ResultCache. There is no replay path in the Executor. Delete both.

---

### Correction 2: Precise Definitions of Rehydration, Replay, Live Execution

The addendum conflated these. Precise definitions:

**Rehydration (state reconstruction):**
- Purpose: rebuild `ExecutionState` from WAL after startup or crash.
- Reads WAL: yes, sequentially from latest snapshot seq.
- Writes WAL: **never**.
- Contacts workers: **never**.
- Emits commands: **never**.
- Output: `ExecutionState` whose `LastSeq` = tail of WAL.
- Terminates: when WAL tail is reached.

**"Replay of orchestration logic":**
This concept does not exist in Grael as a separate phase.

In Temporal, workflow code (Go/Java/Python) is re-executed from the start on each recovery. Activity results are intercepted and returned from history. This requires a replay mode.

In Grael, the orchestration is a **data-driven state machine**, not code. Rehydration rebuilds state by applying events. There is no code to re-execute. There is no interception. Replay mode in the Temporal sense is **absent by design**. This is a deliberate advantage.

The term "replay" in Grael refers exclusively to rehydration (state reconstruction from events). It must not be overloaded to mean "re-execution."

**Live execution:**
- Begins immediately after rehydration completes.
- Reads WAL: yes, via `WaitNext()` (blocks until new event).
- Writes WAL: yes (CommandProcessor writes events resulting from command execution).
- Contacts workers: yes.
- Emits commands: yes (Scheduler.Decide produces commands on each event).

**Summary table:**

| Phase | Reads WAL | Writes WAL | Contacts Workers | Emits Commands |
|-------|-----------|------------|-----------------|----------------|
| Rehydration | yes | **no** | **no** | **no** |
| Live execution | yes (streaming) | yes | yes | yes |
| "Orchestration replay" | **does not exist in Grael** | — | — | — |

---

### Correction 3: Timer Authority — Formal Invariant

Every timer type maps to exactly one event that causes the state transition. No timer type may cause a transition without a persisted event.

| Timer type | Written by | Event written | State transition caused |
|------------|-----------|---------------|------------------------|
| Retry backoff | TimerManager | `TimerFired{purpose: retry_backoff}` | `FAILED → PENDING` (requeue) |
| Node ExecutionDeadline | TimerManager | `TimerFired{purpose: node_exec_deadline}` | `RUNNING → TIMED_OUT` |
| Node AbsoluteDeadline | TimerManager | `TimerFired{purpose: node_abs_deadline}` | Any non-terminal → `TIMED_OUT` |
| Checkpoint timeout | TimerManager | `TimerFired{purpose: checkpoint_timeout}` | `AWAITING_APPROVAL → (OnTimeout action)` |
| Lease expiry | LeaseMonitor | `LeaseExpired` | `RUNNING → lease revoked; requeue or fail` |
| Admission timeout | AdmissionMonitor | `AdmissionTimedOut` | `ADMITTED_QUEUED → ADMISSION_REJECTED` |

The Scheduler must not contain any expression of the form `if time.Now().After(deadline)`. All such logic lives in TimerManager/LeaseMonitor and is expressed as events. The Scheduler reacts to events, never to wall-clock observations.

**Consequence for testing:** The Scheduler's behavior under all time-based scenarios is fully testable by injecting synthetic `TimerFired` and `LeaseExpired` events. No time mocking required.

---

### Correction 4: Deterministic Command Ordering

`Scheduler.Decide(state)` MUST produce `[]Command` in a stable, deterministic order. The following total ordering is defined:

**Phase 1 — Command type precedence (lower number = emitted first):**

```
0: CancelTimer           (clean up before scheduling new timers)
1: TriggerCompensation   (compensation before any new dispatch)
2: FailWorkflow          (terminal signals before lifecycle changes)
3: CompleteWorkflow      (terminal signals before lifecycle changes)
4: ScheduleTimer         (schedule before dispatching dependents)
5: NotifyCheckpoint      (notifications before dispatch)
6: PropagateCancel       (propagation before dispatch)
7: DispatchActivity      (lowest priority — actual work)
8: RequeueActivity
```

**Phase 2 — Within `DispatchActivity`, sort by:**

1. **Topological depth** ascending (shallower nodes dispatch first — they may unblock deeper nodes sooner).
2. **Creation sequence number** ascending (the WAL seq of the event that added this node — FIFO within same depth).
3. **Node ID** lexicographic ascending (stable tie-breaker).

**Why this matters:**

- **Replay reproducibility**: if a test injects the same events in the same order, it always gets the same command sequence and the same trace IDs.
- **Stable spans**: since AttemptID is derived from `(workflowID, nodeID, attempt)`, the span ID is always the same for the same node. This makes traces reproducible and comparable across runs.
- **Predictable tests**: deterministic ordering means test assertions on command sequences are not flaky.

The ordering is part of the Grael specification, not an implementation detail. Any conforming implementation must produce identical command sequences for identical input states.

---

### Correction 5: Lease Race — Strict Last-Writer-Wins via WAL

The WAL has a single writer goroutine. All writes — including `LeaseRenewed` and `LeaseExpired` — go through this serialization point. There is no actual concurrent write race at the WAL level.

**The rule:** whichever event is written to the WAL first is authoritative.

**The race scenario resolved:**

LeaseMonitor observes `ExpiresAt` and submits `LeaseExpired` to the WAL writer queue.
Worker sends `RenewLease` RPC, which also submits `LeaseRenewed` to the WAL writer queue.

The WAL writer processes them in arrival order. One of two outcomes:
- `LeaseRenewed` written first → lease is valid, `LeaseExpired` (which arrives in the queue after) is dropped by the LeaseMonitor's write logic upon seeing the renewal.
- `LeaseExpired` written first → lease is dead. Any subsequent `LeaseRenewed` from the same RPC is rejected with `LEASE_EXPIRED` before it reaches the WAL writer.

**Late `CompleteTask` after `LeaseExpired`:** rejected unconditionally. The worker receives `LEASE_EXPIRED` error code. The worker MUST stop processing that task immediately. Retrying `CompleteTask` with the same `AttemptID` after receiving `LEASE_EXPIRED` is a worker protocol violation.

**Can a `LeaseExpired` lease be revived?** No. `LeaseExpired` is a permanent, immutable event. A new attempt creates a new `AttemptID` and a new lease grant.

---

### Correction 6: `ImmediateCancel` — Honest Contract

**Rename:** `ImmediateCancel` is renamed to `RevokeAndAbandon` in all APIs and documentation to prevent misunderstanding.

**What `RevokeAndAbandon` does (guaranteed):**
1. Writes `CancellationRequested{type: revoke_and_abandon}` to WAL.
2. LeaseMonitor immediately expires all active leases for this run (bulk `LeaseExpired` events, bypassing `ExpiresAt`).
3. All subsequent `CompleteTask`, `RenewLease`, `FailTask` calls for this run are rejected with `RUN_CANCELLED`.
4. Workers polling `PollTask` on a streaming RPC receive a stream termination signal.
5. No grace period. No waiting.

**What `RevokeAndAbandon` does NOT do (explicitly not guaranteed):**
- Does not send SIGKILL or any OS signal to worker processes.
- Does not interrupt blocking network calls inside workers.
- Does not guarantee workers stop within any time bound.
- Does not roll back side effects already produced by workers.

**Worker responsibility:** workers MUST propagate gRPC context cancellation to their internal work. When the `PollTask` stream is terminated by the engine, the `context.Context` passed to the activity handler is cancelled. Workers that ignore context cancellation are out of spec. The engine cannot enforce this — it is a worker implementation contract.

**`GracefulCancel`** (existing) remains unchanged: signals workers, waits `GracePeriod`, then transitions to `RevokeAndAbandon` for any still-running nodes.

**Compensation interaction:** neither cancellation type guarantees that side effects from in-progress (not yet completed) activities are included in the compensation stack. Only COMPLETED nodes are in the stack. In-progress work at cancellation time is abandoned, not compensated.

---

### Correction 7: Two-Deadline Model for AWAITING_APPROVAL

**The problem with single paused deadline:** a node that alternates between `RUNNING` and `AWAITING_APPROVAL` can execute indefinitely in wall-clock time by accumulating many checkpoint pauses. This is an abuse vector.

**Two independent deadlines per node:**

```go
type NodeDeadlines struct {
    // Cumulative active execution time. Paused during AWAITING_APPROVAL.
    // Default: workflow-level MaxNodeExecutionDuration.
    ExecutionDeadline time.Duration

    // Wall-clock time from node creation. NEVER paused. NEVER extended.
    // Default: workflow-level MaxNodeWallClockDuration.
    AbsoluteDeadline time.Duration
}
```

**Semantics:**

`ExecutionDeadline`: measures only time spent in `RUNNING` state. Paused during `AWAITING_APPROVAL`. The remaining budget is tracked in `ExecutionState` and updated at each `RUNNING → AWAITING_APPROVAL` and `AWAITING_APPROVAL → RUNNING` transition. When the budget reaches zero, `TimerFired{purpose: node_exec_deadline}` causes `RUNNING → TIMED_OUT`.

`AbsoluteDeadline`: a single timer scheduled at node creation time (`NodeStarted`). It fires at `createdAt + AbsoluteDeadline` regardless of the node's current state. If it fires while `AWAITING_APPROVAL`, the checkpoint is immediately timed out and the node fails. This deadline cannot be paused, extended, or bypassed by any approval.

**Default values (can be overridden per node, subject to policy precedence):**

```
ExecutionDeadline: 30 minutes (active CPU/IO time)
AbsoluteDeadline:  7 days     (hard wall-clock limit from node creation)
```

The `AbsoluteDeadline` is the safety valve against infinite-via-checkpoints workflows.

---

### Correction 8: `RefreshOnBoundary` Is an Unsafe, Opt-In Mode

**Previous wording** treated this as a normal policy option alongside `StablePerRun`. This underrepresents the severity of the guarantee loss.

**Corrected model:**

```go
type MemoryRefreshPolicy string
const (
    // SAFE (default): memory profile locked at WorkflowStarted.
    // Full replay determinism guaranteed.
    StablePerRun MemoryRefreshPolicy = "stable_per_run"

    // SAFE: profile refreshed only via explicit RefreshMemory activity.
    // Result recorded in WAL → replay determinism preserved.
    ManualRefresh MemoryRefreshPolicy = "manual_refresh"

    // UNSAFE: profile refreshed at boundary points from live memory store.
    // Replay of this workflow may produce different agent behavior if
    // the memory store has changed since the original run.
    // Requires explicit opt-in acknowledgment.
    UnsafeRefreshOnBoundary MemoryRefreshPolicy = "unsafe_refresh_on_boundary"
)
```

**API enforcement:** `StartWorkflow` request with `UnsafeRefreshOnBoundary` MUST include:
```go
AllowNondeterministicMemory bool // must be true; if false, request is rejected
```

**WorkflowStarted payload:** includes `NondeterministicMemory: true` flag. This is indexed and queryable.

**Observability:**
- All spans for this run carry tag `grael.nondeterministic_memory=true`.
- Metric `grael_nondeterministic_runs_total` is incremented.
- The materialized graph view (all projection modes) shows a visible warning indicator.
- Audit log entries note the nondeterminism mode.

**What the guarantee loss means concretely:** if this workflow is replayed for debugging, the agent may receive a different memory profile at boundary points (because memories written between the original run and the replay are now visible). The agent may produce different spawn/fanout decisions. The replay may diverge from the original execution. This is declared and accepted.

**Recommendation for workflow authors:** use `UnsafeRefreshOnBoundary` only when memory freshness is more important than replay fidelity — e.g., long-running workflows where stale memory causes incorrect agent decisions. Use `ManualRefresh` as the first alternative, as it preserves replay fidelity.

---

### Correction 9: Condition Evaluation — Closed Over Recorded State

**Formal rule:**

A skip condition is a pure function:
```
condition: ConditionContext → bool
```

where `ConditionContext` is populated exclusively from events already in the WAL at the time of evaluation.

```go
type ConditionContext struct {
    // Populated from NodeCompleted payloads of dependency nodes
    DepsOutputs map[string]any

    // Populated from WorkflowStarted.MemoryProfile
    // OR from the most recent ManualRefresh NodeCompleted payload
    // NOT from the live memory store
    Memory MemoryProfile

    // From workflow definition (static, immutable)
    Constants map[string]any

    // Explicitly absent: time.Now(), external calls, live state
}
```

The condition evaluator is a sandboxed expression interpreter with **no capability to perform I/O**. It receives `ConditionContext` by value. It returns `(bool, error)`. It has no side effects.

**What this means for the `UnsafeRefreshOnBoundary` interaction:**
Even in `UnsafeRefreshOnBoundary` mode, conditions are evaluated against the **recorded** profile (the one in `WorkflowStarted` or `ManualRefresh` NodeCompleted), not the live memory store. Memory refresh happens at boundary points before node dispatch, and the refreshed profile is recorded in the WAL (as a synthetic `MemoryRefreshed` event) before conditions are evaluated. Conditions are always closed over recorded state, even in unsafe mode.

**Modelling external-state-dependent conditions:**

If a skip condition needs to depend on external state (e.g., "skip if feature flag X is enabled"), the author must:
1. Add an activity that reads the feature flag and returns a boolean.
2. Express the condition as `{{ deps.featureFlagCheck.output.enabled == true }}`.

The engine will not provide a mechanism to make conditions non-deterministic, even at author request.

---

### Correction 10: Three Projection Modes for Event Log

**Source of truth:** the raw WAL. All projections are derived on demand. None are stored as primary state.

**Compact Projection** (`?view=compact`):
- Purpose: quick status, product UI, notifications, webhooks.
- Shows: one entry per node with final state only.
- Collapses: all retry attempts into one entry with `attempts: N`.
- Collapses: fan-out into `FanOutSummary{total, succeeded, failed, skipped}` only.
- Hides: cancelled/abandoned branches, SKIPPED nodes (unless explicitly requested).
- Does not show: individual attempts, timers, lease events, intermediate states.

**Operational Projection** (`?view=operational`):
- Purpose: ops dashboard, incident response, SLA monitoring, active debugging.
- Shows: each active retry attempt separately (past attempts collapsed, current shown in detail).
- Shows: `AWAITING_APPROVAL` state with remaining timeout countdown.
- Shows: fan-out item summary with breakdown by state.
- Shows: running cost totals per node.
- Shows: current lease holder per RUNNING node.
- Hides: completed fan-out items (aggregate only), terminal historical attempts.

**Forensic Projection** (`?view=forensic`):
- Purpose: post-mortem, audit, compliance, deep debugging.
- Shows: everything. No collapsing. No hiding.
- Shows: all attempts with individual durations and error details.
- Shows: all individual fan-out items with per-item results.
- Shows: all timer events (scheduled, fired, cancelled) with timestamps.
- Shows: all lease grants, renewals, expirations with worker IDs.
- Shows: memory lookups with scores and degraded flags.
- Shows: all `CancellationRequested`, `NodeCancelled`, `LeaseExpired` events.
- Shows: raw events interleaved with node entries (timeline view).

**Projection is always computed from WAL + latest snapshot.** There is no projection-specific storage. The only performance optimization is the snapshot-delta approach described in `ARCHITECTURE.md §7`.

---

## Part III — Recommended Wording Replacements

The following phrasings in prior documents should be replaced:

| Original | Replace with | Why |
|----------|-------------|-----|
| "Replay path: synthesize the event from the cached result" | Remove entirely | Replay never writes events; the concept is wrong |
| "ResultCache" | Remove entirely | Unnecessary; rehydration already reflects completed nodes |
| "Replay bool" in StepContext | Remove entirely | Replay awareness does not belong in worker API |
| "ImmediateCancel" | "RevokeAndAbandon" | Original name implies process kill, which the engine cannot do |
| "RefreshOnBoundary" | "UnsafeRefreshOnBoundary" | Silent unsafeness is worse than an obvious name |
| "NodeDeadline is paused during AWAITING_APPROVAL" | "ExecutionDeadline is paused; AbsoluteDeadline is not" | Single deadline model enables checkpoint-based deadline bypass |
| "Replay of orchestration logic" | Remove / replace with "rehydration" | Orchestration replay in the Temporal sense does not exist in Grael |

---

## Part IV — Delta vs Previous Addendum

| # | Correction | Why Critical |
|---|-----------|-------------|
| 1 | ResultCache and replay path in Executor removed entirely | A node already in COMPLETED state never reaches dispatch; the concept was architecturally wrong |
| 2 | "Orchestration replay" declared non-existent in Grael | Prevents implementors from building a Temporal-style replay interceptor that isn't needed |
| 3 | Timer authority formalized as invariant with full table | Without this, implementors may add deadline checks inside the Scheduler (violates determinism) |
| 4 | Command ordering defined as total order with priority + topo depth | Without stable ordering, replay produces different traces; tests are flaky |
| 5 | Lease race resolved via WAL single-writer serialization; expired lease is permanently dead | "Almost simultaneous" race was unresolved; result was ambiguous worker protocol |
| 6 | ImmediateCancel renamed and bounded honestly | Claiming force-stop semantics in a distributed system is a false guarantee; workers must know the real contract |
| 7 | Two-deadline model (ExecutionDeadline + AbsoluteDeadline) | Single paused deadline allows checkpoint-based deadline bypass; AbsoluteDeadline closes this |
| 8 | UnsafeRefreshOnBoundary requires explicit opt-in, carries observable markers | Treating nondeterministic mode as a normal option understates the guarantee loss |
| 9 | Condition evaluation formally closed over WAL-recorded state only; evaluator has no I/O capability | Without this, a condition that calls time.Now() or an external service breaks determinism silently |
| 10 | Three projection modes (compact / operational / forensic) with precise definitions | Single "materialized view" is insufficient; ops and forensic have different requirements and different noise levels |

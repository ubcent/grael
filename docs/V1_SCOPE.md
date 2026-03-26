# Grael v1 — Brutally Honest Scope Definition

---

## 1. What Grael v1 ACTUALLY IS

Grael v1 is a **durable execution engine for long-running, branching AI agent pipelines**, where the shape of the pipeline is not known upfront. A single binary. No Postgres. No Redis. Workers connect over gRPC, poll for tasks, and complete them. The engine survives crashes. When an agent completes a step and discovers new work, it spawns new nodes at runtime — the graph grows. Human approvals block specific steps without blocking the engine. Retries, backoff, and basic saga rollback are built-in. Target user in v1: a backend engineer building an autonomous AI system (coding assistant, research pipeline, document processing) who has already hit the wall with ad-hoc queues and stateless function chains, and needs durability + dynamic structure without standing up a Temporal cluster.

Not a generic workflow engine. Not a replacement for Airflow. Not a platform.

---

## 2. Non-negotiable Core

These six things ARE Grael. Remove any one of them and the product becomes "just another task queue."

---

### 2.1 Append-only WAL + Rehydration

**Why it's core:** Without it, Grael is a queue. With it, it's a durable execution engine that survives crashes and gives you a full audit trail. This is the entire foundation.

**What breaks without it:** Every other guarantee — retries, compensation, replay, determinism — disappears. You're back to at-most-once.

**Minimal acceptable implementation:**
- Append-only flat file, one WAL file per run.
- msgpack-encoded events, no custom binary format in v1.
- CRC32 per event (corruption detection, not correction).
- In-memory index `seq → offset` rebuilt on startup by scanning the file.
- Snapshots: take one. On startup: load snapshot + replay delta. That's it.
- No multi-WAL sharding, no compaction, no archival in v1.

---

### 2.2 Node State Machine + Scheduler as Pure Function

**Why it's core:** This is what makes Grael correct. The Scheduler is `f(state) → []commands`. No I/O, no time, no randomness. You can unit test the entire orchestration logic by feeding events and asserting commands. Without this separation, you have spaghetti.

**What breaks without it:** Non-determinism leaks in. Tests require mocks everywhere. Recovery becomes guesswork.

**Minimal acceptable implementation:**
- 7 node states: `PENDING, READY, RUNNING, AWAITING_APPROVAL, COMPLETED, FAILED, SKIPPED`. That's it for v1 — drop `TIMED_OUT` as a separate state, map it to `FAILED{reason: timeout}`.
- `Scheduler.Decide(state ExecutionState) []Command` — pure function, no goroutines.
- `CommandProcessor.Execute(cmd)` — does the I/O, writes events back.
- State machine lives in `ExecutionState.Apply(event)`.

---

### 2.3 Living DAG — Runtime Node Spawn

**Why it's core:** This is THE differentiator. Temporal can sorta do this but it's ugly. LangGraph can't persist it durably. This is the one thing that makes an engineer stop and say "oh that's different."

**What breaks without it:** Grael becomes a fixed-graph workflow engine. Just use Inngest or Trigger.dev instead.

**Minimal acceptable implementation:**
- `NodeCompleted` payload carries `SpawnedNodes []NodeDefinition`.
- On `Apply(NodeCompleted)`: add spawned nodes to graph as `PENDING`.
- Graph is just an in-memory adjacency structure, rebuilt from WAL on rehydration.
- Cycle detection: yes, required (DFS at spawn time). Not optional. A cycle in the WAL is unrecoverable.
- No `MaxNodes` enforcement in v1 — document the risk, add a soft warning at 500 nodes, hard limit at 5000. Don't build a full policy engine.

---

### 2.4 Worker Protocol (gRPC PollTask / CompleteTask / FailTask)

**Why it's core:** Without a real worker protocol, Grael has no execution model. Workers are where AI agents run. If workers can't connect over gRPC and poll reliably, nothing works.

**What breaks without it:** You have a state machine with no way to run actual work.

**Minimal acceptable implementation:**
- Three RPCs: `PollTask` (long-poll, not streaming), `CompleteTask`, `FailTask`.
- Lease model: `LeaseGranted` event before dispatch; `LeaseExpired` by LeaseMonitor when timer fires. This is required for correctness (stuck worker detection) — do not skip.
- Worker identified by `WorkerID` string. No mTLS, no RBAC in v1 — use a single shared secret in config if you care, but don't build an auth system.
- Heartbeat: yes. Workers send `Heartbeat` every 10s. On timeout: expire all leases.

---

### 2.5 Retries + Backoff Timers

**Why it's core:** AI agents fail. Network calls fail. Without automatic retry, every transient failure is a manual intervention. Without persisted timers, retries disappear on crash.

**What breaks without it:** Every pipeline requires manual recovery on any failure. Unusable for production AI workloads.

**Minimal acceptable implementation:**
- RetryPolicy per node: `MaxAttempts`, `InitialInterval`, `Multiplier`, `NonRetryable []string`.
- `TimerScheduled` / `TimerFired` events in WAL (absolute timestamps only, never durations).
- TimerManager: min-heap, catch-up on restart (overdue timers fire immediately with `Late: true`).
- Node deadline: yes, required — without it, a stuck worker can stall a pipeline forever.
- Two deadline types (ExecutionDeadline + AbsoluteDeadline): **simplify to one in v1**. Call it `NodeDeadline`. The two-deadline model is correct but it adds implementation complexity. Document it as a known limitation: checkpoint abuse can extend execution. Ship the two-deadline model in v1.1.

Actually no — keep two deadlines. Here's why: a checkpoint that can stall indefinitely will be hit in the first serious demo. One extra timer is not complex. The impl cost is 1 additional timer type. Keep it.

---

### 2.6 Cancellation (GracefulCancel only)

**Why it's core:** Any workflow engine that can't be stopped is unusable. Cancellation is table stakes.

**What breaks without it:** Long-running pipelines can't be interrupted. Any mis-fire or budget overrun is permanent.

**Minimal acceptable implementation:**
- `GracefulCancel` only. `RevokeAndAbandon` is v1.1.
- `CancellationRequested` → PENDING/READY nodes cancelled immediately → RUNNING nodes get grace period → `CancellationCompleted`.
- Fan-out cancellation: yes (same mechanism).
- Child workflow cancellation: not in v1 (sub-workflows are cut — see §3).
- Compensation on cancel: **optional config, implement it** — this is 2 additional lines in the Scheduler after GracefulCancel logic is done.

---

## 3. Aggressive Cuts

Everything below is either killed entirely or replaced with a primitive.

---

### 3.1 Memory Layer — KILL ENTIRELY FOR v1

This is the most important cut. The memory layer is not a feature of Grael. **It is a separate product.** A versioned knowledge graph with embeddings, HNSW, BM25 fallback, relationship confidence scoring, and async embedding pipeline is a significant system on its own. Supermemory built a whole startup around it.

**Kill:**
- Everything in `MemoryStore` interface
- `MemoryProfile` injection into `StepContext`
- `MemoryRefreshed` events
- `UnsafeRefreshOnBoundary` mode
- Embeddings, HNSW, cosine similarity, BM25
- Versioned knowledge graph
- `GetProfile`, `Recall`, `Remember`, `Forget`

**Replace with:**
- `StepContext.Input` carries whatever the caller put in `StartWorkflow.Input`. If a worker needs memory, it queries its own memory service. Grael doesn't care.
- Document the integration point: "inject memory profile into workflow input at start time."

**Why this is not cowardice:** The living DAG + durable execution is already a complete, shippable product without memory. Adding memory in v1 means you ship neither properly.

---

### 3.2 Sub-workflows — KILL FOR v1

Sub-workflows require: parent-child run linking, `PropagateCancel` crossing run boundaries, child cancellation timeout logic, child terminal state propagation back to parent node.

This is not one feature. It's four separate correctness problems.

**Replace with:** Workers can call `StartWorkflow` on a new run themselves. The parent workflow can use an external event trigger to wait for the child to complete. Hacky? Yes. Good enough for v1? Yes.

---

### 3.3 Fan-out / Map-reduce — SIMPLIFY

Full fan-out with `FanOutFailurePolicy` (FailFast/FailSlow/BestEffort), partial result types, and per-item compensation tracking is complex.

**Replace with:**
- Agents implement fan-out by spawning N nodes from `NodeCompleted{spawn}`. That IS the fan-out mechanism — it's the living DAG.
- A built-in "gather" node type that waits for all deps in `{COMPLETED, SKIPPED}` is just the regular PENDING→READY transition. No new code needed.
- Skip `FanOutFailurePolicy` entirely. Default: if any item fails → retries → if exhausted → workflow fails. One policy only.
- No `FanOutResult` wrapper type. Worker that receives the gather node gets dep outputs via `StepContext.DepsOutputs`.

---

### 3.4 Three Projection Modes — REPLACE WITH ONE

Compact / Operational / Forensic projection design is excellent. Build it in v2.

**Replace with:**
- One endpoint: `GetRun(runID)` returns current state of all nodes (state, attempts, last error).
- One endpoint: `ListEvents(runID)` returns raw WAL events. This IS the forensic view.
- That's it. No fancy projection engine. No collapsing. No aggregation.

---

### 3.5 Activity Versioning (Capability Ranges) — SIMPLIFY

Version ranges (`MinVersion/MaxVersion`, semver constraint matching) are genuinely useful but add routing complexity.

**Replace with:**
- Activity type is a string. Workers register: `{WorkerID, ActivityTypes: ["scout", "review"]}`.
- If no worker registered for activity type X: task queued up to `NoWorkerTimeout`, then fail.
- No version ranges in v1. Document: "breaking changes to an activity require a new activity type name."

---

### 3.6 Error Handler (separate from retry) — KILL FOR v1

A separate error handler node that runs in `HANDLING_ERROR` run state adds: a new run state, a new event type, `HandlerStarted/Completed/Failed`, and interaction with compensation.

**Kill:** Remove `HANDLING_ERROR`, `HandlerStarted`, `HandlerCompleted`, `HandlerFailed`. Remove from run state machine.

**Replace with:** Retry policy + compensation covers 90% of v1 use cases. If a node fails permanently, the workflow fails. Workers can model error handling as a regular dependency node with a skip condition.

---

### 3.7 Admission Queue — KILL FOR v1

The admission queue with `ADMITTED_QUEUED`, `AdmissionTimedOut`, `AdmissionAccepted`, `AdmissionRejected` solves the problem of engine overload.

**Kill:** In v1, `StartWorkflow` either succeeds (run starts immediately) or returns an error if the engine is at hard capacity. No queue, no wait. Just synchronous reject.

---

### 3.8 External Event Deduplication — SIMPLIFY

The full dedup store with TTL windows and sequence number buffering is a real feature but overkill for v1.

**Replace with:**
- `IngestExternalEvent(runID, eventID, payload)` — write to WAL if `eventID` not seen in last 24h window.
- Implement dedup store as a simple bbolt bucket with key=`eventID`, value=`processedAt`.
- No out-of-order buffering. No sequence numbers. If ordering matters, that's the caller's problem in v1.

---

### 3.9 mTLS / RBAC — KILL FOR v1

**Kill entirely.** Worker authentication in v1: shared token in config. If you need RBAC, you are not the v1 user.

---

### 3.10 Conformance Test Matrix (as a separate artifact) — KILL

The conformance test matrix in the spec is for when there are multiple implementations conforming to the spec. Right now there is one implementation. Write real integration tests, not a spec for tests.

---

### 3.11 `UnsafeRefreshOnBoundary` Memory Mode — MOOT

Already killed with the memory layer. Nothing to cut.

---

### 3.12 Workflow Versioning / Migration — SIMPLIFY

Full `(name, version)` pinning with registered migration functions is correct for v3.

**Replace with:**
- Every workflow run records the definition hash at start time. That's it.
- No migration API. If the definition changes incompatibly, old runs finish on the old definition (already in WAL state), new runs use the new definition. No engine enforcement.

---

## 4. Fake Complexity

These are elements in the current spec that look rigorous but provide no value in v1.

---

### 4.1 20 Global Invariants as a Separate Section

The invariants are correct. Writing them as a numbered list in a spec doc does not make the implementation satisfy them. Tests do. In v1, these invariants should be expressed as property-based tests and integration tests, not as a documentation artifact.

**Simplify:** Delete the invariants section from the spec. Encode them as tests. An invariant that isn't tested is just a wish.

---

### 4.2 OperationID vs AttemptID Distinction

This is correct and you should implement both. The fake complexity is the elaborate derivation formula and documentation. Just generate both, inject both into the task, and document that `OperationID` is for external idempotency. That's the entire thing. Half a page in the spec, 3 lines of code.

---

### 4.3 `TimerPurpose` as a Rich Enum with 6 Values

`retry_backoff`, `node_exec_deadline`, `node_abs_deadline`, `checkpoint_timeout`, `lease_expiry`, `admission_timeout`.

In v1: `admission_timeout` is gone (admission queue killed). `lease_expiry` is handled by LeaseMonitor separately from the timer heap (it has its own eviction logic). That leaves 4 purposes. Not simplifiable, actually correct. This is not fake complexity. Keep it.

---

### 4.4 `CompensationPolicy.Parallel` Mode

Sequential compensation is the only mode you need in v1. Parallel compensation requires a fan-out coordinator for compensation actions, which is scope creep. Sequential is also safer and easier to reason about.

**Kill:** `Parallel bool` field removed. Sequential only in v1.

---

### 4.5 `FanOutFailurePolicy` Enum

Already killed in §3.3. Noting it here because the spec treats it as a fundamental abstraction. It's not. It's a configuration knob that matters only when you have high-volume fan-outs with mixed acceptable failure rates. v1 AI pipelines don't have this problem yet.

---

### 4.6 Three-Mode Projection Architecture

Already killed. But worth noting: the real fake complexity here is designing a projection system before having actual users who need projections. In v1, the users ARE you (or the first 5 people using Grael). You know exactly what they need from `GetRun`. Build that. Don't build a projection engine.

---

### 4.7 Relationship Confidence Scoring in Memory Layer

Already killed with the memory layer. But the reason it's fake complexity in v1: you haven't shipped a single workflow yet. You don't know which memories matter and by how much. Confidence scoring is an optimization of a system you don't have.

---

### 4.8 Graph Policy Enforcer as a Separate Component

`MaxNodes`, `MaxDepth`, `MaxFanOutWidth`, `MaxSpawnBudget`, `MaxRunDuration` as a `GraphPolicy` struct with full enforcement.

**Simplify:** In v1, hard-code sane limits: 5000 nodes, 100 depth, 2000 spawn budget. Enforce at spawn time with 3 if-statements. Do not build a configurable policy system. The policy system is for when you have multi-tenant deployments with different limits per customer. You don't have that yet.

---

## 5. The Hard Truths

---

### 5.1 Temporal Already Exists and It's Good

Temporal runs at Stripe, Netflix, Descript, Airbyte, and hundreds of other companies. It handles dynamic workflows (you can spawn new activities from workflow code), has a mature Go SDK, a cloud offering, a UI, and a massive ecosystem.

The honest differentiation Grael has against Temporal:
1. **No external database.** Temporal requires a database (Postgres or Cassandra) and a separate service. Grael is one binary. This is genuinely valuable for self-hosted, air-gapped, or resource-constrained deployments.
2. **Living DAG as a first-class concept.** Temporal's dynamic workflows work by re-executing workflow code with a replay flag — this is conceptually fragile and limits what agents can express. Grael's data-driven DAG has no equivalent replay problem.
3. **Built specifically for AI agent orchestration.** Temporal was designed for microservice orchestration. Grael's primitives (spawn, checkpoint, memory) match how AI pipelines actually work.

If Grael v1 doesn't demonstrate #1 and #2 clearly, there is no product. #3 requires the memory layer which is cut. So v1 differentiates on: single binary, no external deps, living DAG.

**Risk:** This is a narrow differentiation. "Temporal but simpler and single-binary" is a valid product but not a category-defining one. The category-defining version requires the memory layer. Which means the memory layer, cut for v1, is actually the long-term bet. v1 must be positioned carefully.

---

### 5.2 LangGraph Is Growing Fast

LangGraph (LangChain) just shipped a cloud product. It's Python-native, which is where AI engineers live. It has persistence, resumability, and checkpoints. It's not as rigorous as Grael but it's good enough for most AI workflows.

The honest answer: for Python AI engineers, LangGraph will be their first choice. Grael's v1 Go SDK addresses Go-first engineers who care about correctness over convenience. That's a real but small market.

---

### 5.3 The Memory Layer Is the Actual Product Differentiator

Here is the uncomfortable truth: the "living DAG" is cool. The event sourcing is correct. But neither of these makes Grael irreplaceable.

What makes Grael irreplaceable for AI pipelines is a memory layer that:
- Remembers what worked and what didn't across runs
- Lets agents get smarter over time
- Is fully integrated with the execution engine (memory is first-class, not bolted on)

This is the part no one else has. Temporal doesn't have it. LangGraph has a basic form of it but it's not engine-integrated. Without this, Grael is "correct workflow engine" — a nice property, not a category.

**Implication for v1:** Ship the engine fast and well. Get users. Use the feedback to design the memory layer correctly. The memory layer in the current spec was designed in a vacuum. Real workflows will tell you what memory primitives actually need to be.

---

### 5.4 The Spec Is Beautiful But You Are Months From Running a Single Workflow

The current architecture documents are excellent. GRAEL_RUNTIME_SPEC.md is a production-quality specification. But right now you cannot run a single "hello world" workflow. No code exists.

The risk: you keep refining the spec. The spec becomes a substitute for shipping. After 6 months you have a perfect spec and no code. This is how ambitious solo projects die.

The spec is done. Stop touching it. Write Go code.

---

### 5.5 Checkpoint (Human-in-the-Loop) Is Overestimated for v1

Human checkpoints are a good feature. But in v1, most users will have zero human checkpoints in their workflows. They're building fully autonomous pipelines where human review comes outside the engine (in a Slack message or a PR).

The checkpoint feature adds: `AWAITING_APPROVAL` node state, `CheckpointReached` / `CheckpointApproved` / `CheckpointRejected` events, timeout handling (two deadline types), notification service, a new run state transition. This is 3-4 weeks of work.

**Recommendation:** Keep checkpoints in v1. They are the thing that enterprise users need from day one (compliance, approval workflows). But implement them last (weeks 10-11). If you're running out of time, this is the first thing to push to v1.1.

---

### 5.6 Solo Engineer Timeline Is the Biggest Risk

An event-sourced workflow engine with a full worker protocol, timers, leases, compensation, and a living DAG is a large system. Even with the cuts above, this is 4-6 months of focused engineering for a senior engineer who knows the domain.

"8-12 weeks" is aggressive. It's achievable only if:
- You write code every day, not architecture docs
- You do not gold-plate anything
- You cut scope when you hit resistance
- You have a working demo by week 8, not a perfect engine

If week 8 arrives and checkpoints aren't done: ship without checkpoints. If week 8 arrives and saga compensation isn't done: ship without saga. The non-negotiable by week 8 is: WAL, living DAG, worker protocol, retries, leases, cancellation. Everything else is v1.1.

---

## 6. v1 Architecture

This is what actually gets built. No aspirational layers.

### Package Structure

```
grael/
  cmd/
    grael/              # main.go — single binary, flag parsing, server start
  internal/
    wal/                # WAL read/write, CRC32, scan, offset index
    state/              # ExecutionState, Apply(event), Graph, LiveGraph
    scheduler/          # Scheduler.Decide(state) → []Command — pure function
    processor/          # CommandProcessor: executes commands, writes events
    timer/              # TimerManager: min-heap, catch-up, TimerFired emission
    lease/              # LeaseMonitor: expiry tracking, heartbeat, bulk expire
    worker/             # gRPC server: PollTask, CompleteTask, FailTask, Heartbeat
    api/                # gRPC server: StartRun, CancelRun, GetRun, ListEvents
    engine/             # Engine: wires everything, one goroutine per active run
    store/              # bbolt wrapper: snapshots, external event dedup, run index
  proto/
    grael.proto         # all gRPC service definitions
  sdk/
    go/                 # thin Go worker SDK (optional, implement last)
```

**That's 10 packages.** Not 20. Not 30.

### Component Interaction

```
API RPC (StartRun)
  │
  ▼
store.CreateRun() → wal.Append(WorkflowStarted)
  │
  ▼
engine.RunLoop(runID) [one goroutine per run]
  │
  ├── rehydrate: wal.Scan() → state.Apply() per event → ExecutionState
  │
  └── loop:
        state → scheduler.Decide() → []Command
        for cmd in commands:
          processor.Execute(cmd) → wal.Append(events)
        wal.WaitNext() → state.Apply(event) → repeat
```

### Goroutine Model

| Goroutine | Count | Role |
|-----------|-------|------|
| `engine.RunLoop` | 1 per active run | Drives state machine; blocks on `WaitNext` |
| `timer.TimerManager` | 1 global | Min-heap; fires `TimerFired` events |
| `lease.LeaseMonitor` | 1 global | Scans active leases; fires `LeaseExpired` events |
| `worker.gRPCServer` | 1 | Accepts worker connections; routes tasks |
| `api.gRPCServer` | 1 | Accepts client connections |

No complex goroutine pools in v1. One goroutine per run is fine up to ~1000 concurrent runs. That's well beyond v1 scale.

### What Works Synchronously vs Asynchronously

| Operation | Sync/Async |
|-----------|-----------|
| `StartRun` → first `WorkflowStarted` | Synchronous (return after WAL write) |
| Node dispatch to worker | Async (RunLoop goroutine) |
| Timer fire | Async (TimerManager goroutine → WAL → RunLoop picks up) |
| Lease expiry | Async (LeaseMonitor goroutine → WAL → RunLoop picks up) |
| Snapshot write | Async (on background goroutine; never blocks RunLoop) |
| `GetRun` | Synchronous (reads from in-memory state, not WAL) |

### System Boundaries

- **Inbound:** gRPC API (client submits workflows, humans approve checkpoints)
- **Outbound:** gRPC worker connections (engine pushes tasks via long-poll)
- **Storage:** Single directory on disk (WAL files + bbolt database). No network storage. No S3 in v1.
- **No outbound webhooks in v1.** Notifications for checkpoints: a client can poll `GetRun` or subscribe to events. Push notifications are v1.1.

---

## 7. 8–12 Week Solo Plan

### Weeks 1–2: Storage Foundation

**What:** WAL implementation + ExecutionState skeleton.

- `wal` package: `Append(event)`, `Scan(from seq)`, `WaitNext(ctx, runID, lastSeq)` (channel-based fan-out).
- CRC32 per entry, scan-based recovery on corrupt tail.
- In-memory offset index: `seq → file_offset`. Rebuilt on startup by scanning.
- `state` package: `ExecutionState`, `Apply(event)` for all v1 event types, `LiveGraph` (adjacency list, topo sort, cycle detection).
- `store` package: bbolt for snapshots and run index.

**Observable outcome:** `go test ./internal/wal ./internal/state` passes. You can write 10,000 events to the WAL, restart, replay them, and get the same `ExecutionState`. No network, no gRPC, no workers yet. Just storage correctness.

---

### Weeks 3–4: Scheduler + CommandProcessor

**What:** Pure-function Scheduler + CommandProcessor skeleton. No workers yet.

- `scheduler` package: `Decide(state) []Command`. Covers `DispatchActivity`, `ScheduleTimer`, `CancelTimer`, `CompleteWorkflow`, `FailWorkflow`, `TriggerCompensation`.
- `processor` package: `Execute(cmd)` — writes events to WAL. For `DispatchActivity`, writes `LeaseGranted + NodeStarted` but does not yet contact a worker (stub it).
- `engine` package: `RunLoop` goroutine, rehydration, the `WaitNext` event loop.

**Observable outcome:** In a unit test, create an `ExecutionState` with 3 nodes A→B→C. Feed `WorkflowStarted` event. Run `Decide()` in a loop with synthetic `NodeCompleted` events. Verify the correct command sequence is produced in correct order. No I/O anywhere. Entire test runs in microseconds.

---

### Weeks 5–6: Worker Protocol

**What:** gRPC server, `PollTask`, `CompleteTask`, `FailTask`, `Heartbeat`. Real lease model.

- `proto/grael.proto`: define `WorkerService` with the 4 RPCs.
- `worker` package: gRPC server. `PollTask` as long-poll (HTTP/2 streaming or just a blocking RPC with a timeout). Task queue per activity type. Worker registry.
- `lease` package: `LeaseMonitor`. Tracks `ExpiresAt` for all active leases. On expiry: writes `LeaseExpired` event to WAL. Heartbeat timeout: 30s → bulk expire.
- Wire `DispatchActivity` command execution to the real worker pool.

**Observable outcome:** Write a standalone Go program that acts as a worker. It registers for activity type `"hello"`, polls, receives a task, sleeps 1 second, completes it. Start a Grael server. Submit a workflow with one `"hello"` node. Watch it go: `PENDING → READY → RUNNING → COMPLETED → WorkflowCompleted`. Crash the server mid-execution. Restart. Watch the workflow resume.

This is the first real end-to-end demo.

---

### Weeks 7–8: Timers + Living DAG

**What:** Timer system + dynamic node spawn.

- `timer` package: min-heap `TimerManager`. `ScheduleTimer` command → insert into heap. Background goroutine: sleep until next `FireAt`. On fire: `wal.Append(TimerFired)`. On startup: catch-up for overdue timers.
- Retry backoff: `NodeFailed` → Scheduler emits `ScheduleTimer{retry_backoff}` → `TimerFired` → `RequeueActivity`.
- Node deadline: `NodeStarted` → Scheduler emits `ScheduleTimer{node_deadline}` → `TimerFired` → `NodeFailed{timeout}`.
- Living DAG: Worker SDK allows returning `SpawnedNodes` in `CompleteTask`. Engine applies them. Write the spawn validation (cycle detection, depth check).

**Observable outcome:** Demo #2 — an "agent" worker that:
1. Discovers 3 files in a directory.
2. Returns `NodeCompleted{spawn: [analyze_file_1, analyze_file_2, analyze_file_3]}`.
3. Three new nodes appear in the graph at runtime.
4. Each spawns a summary node when done.
5. Show the graph growing in real-time via `GetRun` polling.

If this demo doesn't make you excited, you've implemented it wrong.

---

### Weeks 9–10: Cancellation + Saga Compensation

**What:** GracefulCancel + basic saga.

- Cancellation: `CancellationRequested` event → Scheduler cancels PENDING/READY immediately, signals RUNNING via lease mechanism. `CancellationCompleted` after all terminal.
- `CompensationStack` tracking: every `NodeCompleted` for a node with `compensate:` → push to stack.
- `TriggerCompensation` command → `SagaCoordinator` runs stack in reverse. Sequential only. One retry policy for compensation actions.
- Terminal run states: `COMPENSATED`, `COMPENSATION_PARTIAL`, `COMPENSATION_FAILED`.

**Observable outcome:** Demo #3 — a pipeline that:
1. Runs 3 nodes (A, B, C) with compensation handlers defined.
2. Node C fails permanently.
3. Compensation runs: C is skipped (never completed), B unwinds, A unwinds.
4. Watch the events in `ListEvents`: `CompensationStarted`, `CompensationActionCompleted` × 2, `CompensationCompleted`.
5. Also demo: cancel mid-flight, with `RunCompensation: true`.

---

### Weeks 11–12: Checkpoints + Go SDK + Integration Tests

**What:** Human checkpoints, Go worker SDK, conformance integration tests.

- Checkpoint: worker returns `StepResult{Checkpoint: &CheckpointRequest{...}}`. Scheduler emits `CheckpointReached` → node → `AWAITING_APPROVAL`. `ApproveCheckpoint(runID, nodeID)` API. `CheckpointApproved` → node re-dispatched. Timeout via `ScheduleTimer{checkpoint_timeout}`.
- Go SDK: thin wrapper over gRPC. `worker.Register("activity_type", handler)`. `worker.Start()`. Handler receives `Task`, returns `(output, error)`. Spawn helper: `task.Spawn([]NodeDef{...})`.
- Integration tests covering: happy path, retry+recovery, crash recovery (stop process, restart, verify completion), cancellation, compensation, checkpoint flow.

**Observable outcome:** The "wow" demo from §8. Running. Repeatable. Crash-tested.

---

## 8. What Makes v1 "Wow"

Single demo that shows everything in 5 minutes:

```
Submit workflow: "analyze-codebase"
Input: {repo_url: "..."}
```

**What happens:**

1. `clone-repo` node runs. Worker clones the repo, discovers 47 Go files. Returns:
   ```
   NodeCompleted{spawn: [analyze_file_1, analyze_file_2, ... analyze_file_47]}
   ```
   **The graph grows from 3 nodes to 50 nodes, live, in front of you.**

2. 47 `analyze_file_N` nodes run in parallel (up to concurrency limit). Three of them transiently fail (simulated). You watch them automatically retry with backoff. No human intervention.

3. One node returns a checkpoint:
   ```
   "Found potential security issue in auth.go — approve to continue?"
   ```
   **Execution pauses. The rest of the 47 nodes continue. Only this one waits.**
   You call `ApproveCheckpoint`. That specific analysis continues.

4. All 47 analyses complete. `gather-results` node runs (it depended on all 47). It generates a summary.

5. Kill the Grael process mid-execution. Restart it. Watch it pick up exactly where it stopped. No lost work. No re-execution of completed nodes.

6. Replay `ListEvents` from the beginning. Every event, in order, with timestamps. The complete history.

**What an engineer says when they see this:** "Oh. This is what I've been trying to build for the last 3 months."

The key moments:
- Watching the graph grow in real-time (living DAG)
- Seeing a crash recovery that actually works with zero data loss
- A human approval that doesn't block the rest of the graph
- The complete event history

None of these require the memory layer. All of these are possible with v1.

---

## 9. What NOT to Touch for 3 Months

Strictly forbidden until the core demo above is running and at least 3 real users have used it:

| Forbidden | Why |
|-----------|-----|
| Memory layer / embeddings / HNSW | Separate product; build after you understand what memory AI agents actually need |
| mTLS / RBAC / multi-tenancy | Premature platform-building; use a shared secret |
| Sub-workflows | Complex correctness; solve with worker-initiated `StartRun` |
| Multiple projection modes | You have `GetRun` and `ListEvents`; that's enough |
| Admission queue | Hard reject at capacity; that's fine for v1 |
| Activity version ranges | Use activity type strings; breaking changes → new type name |
| Error handler nodes | Retry + compensation is sufficient for v1 |
| `CompensationPolicy.Parallel` | Sequential only |
| Workflow migration API | Not needed until you have users on v1 who need v2 |
| WebUI / dashboard | `ListEvents` is your UI for now |
| Outbound webhooks | Poll `GetRun` |
| The conformance test matrix as a doc | Write tests, not specs for tests |
| Any work on the spec docs | They are done. Writing more spec is procrastination. |

**The single most important rule:** If you find yourself designing a new abstraction instead of writing a test, stop. Write the test first.

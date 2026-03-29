# Grael — Architecture

> Note
> Memory-layer material in this document is historical and no longer part of Grael's active product scope.
> The memory/knowledge product now lives separately as `OmnethDB`.
> In Grael, integration with OmnethDB happens only through explicit workflow input, node input, and external worker/API calls.

## What Grael is

Grael is a workflow engine built for AI agents.

It runs a **living graph** — a DAG that agents can extend at runtime. Steps can spawn new steps. Fan-outs emerge from data. The graph grows; it never shrinks or mutates existing structure.

Underneath is an append-only event log. Every state transition is an event. The current graph is always derived by replaying that log — which means the engine survives crashes and resumes exactly where it stopped.

Grael ships as a single binary with no external dependencies.

---

## Component Map

```
┌─────────────────────────────────────────────────────────────────┐
│                          Grael Engine                            │
│                                                                  │
│  ┌──────────────┐   ┌──────────────┐   ┌────────────────────┐  │
│  │   Scheduler  │   │   Executor   │   │  Saga Coordinator  │  │
│  │              │──▶│              │──▶│                    │  │
│  │ rehydrate()  │   │ dispatch()   │   │ compensation stack │  │
│  │ ready nodes  │   │ worker pool  │   │ reverse unwind     │  │
│  └──────┬───────┘   └──────┬───────┘   └────────────────────┘  │
│         │                  │                                     │
│  ┌──────▼───────────────────▼────────────────────────────────┐  │
│  │                      Event Log (WAL)                       │  │
│  │            append-only, CRC32, memory-mapped idx          │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌─────────────────────┐   ┌──────────────────────────────────┐ │
│  │   Memory Store      │   │   Graph Policy Enforcer          │ │
│  │                     │   │                                  │ │
│  │ bbolt + cosine/HNSW │   │ cycle detection, depth, budget   │ │
│  │ versioned KG        │   │ concurrency semaphore            │ │
│  └─────────────────────┘   └──────────────────────────────────┘ │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │   gRPC Server  │  Worker Registry  │  Event Subscription   │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
         ▲                        ▲
         │ gRPC                   │ gRPC (poll/complete)
┌────────┴──────────┐    ┌────────┴────────────────┐
│   Grael Client    │    │    Activity Workers      │
│ (TypeScript, Go)  │    │ (any language, any count)│
└───────────────────┘    └─────────────────────────┘
```

**Responsibilities:**

| Component | Owns |
|-----------|------|
| Scheduler | rehydration, ready-node detection, blocking on empty |
| Executor | worker dispatch, fan-out coordination, result ingestion |
| Saga Coordinator | compensation stack per run, reverse-order unwind |
| Event Log | durable append, CRC recovery, mmap index, pub/sub |
| Memory Store | versioned KG, embedding search, profile assembly |
| Graph Policy Enforcer | guardrails at spawn/fanout time, concurrency limits |
| Worker Registry | authentication, capability registration, heartbeat |

---

## 1. Execution Model

### Node Lifecycle — Formal State Machine

```
                     ┌─────────┐
            ┌───────▶│ PENDING │ (deps not satisfied)
            │        └────┬────┘
            │             │ all deps COMPLETED
            │        ┌────▼────┐
            │        │  READY  │ (queued for dispatch)
            │        └────┬────┘
            │             │ dispatched
            │        ┌────▼────┐
            │        │ RUNNING │◀──────────────────┐
            │        └────┬────┘                   │
            │      ┌──────┼───────┐                │ retry
            │      ▼      ▼       ▼                │
            │  COMPLETED FAILED TIMED_OUT ──────────┘
            │      │      │       │           (if attempts < max)
            │      │      ▼       ▼
            │      │  COMPENSATING (if compensate defined)
            │      │      │
            │      │      ▼
            │      │  COMPENSATED
            │      │
            │      └──▶ (spawn new nodes → they enter PENDING)
            │
            └──── SKIPPED (condition evaluated false)

Special:
  WAITING_APPROVAL  (human checkpoint — sub-state of RUNNING)
```

Transitions are events in the WAL. A node's state is always derived from the log — never stored independently.

### Graph Mutation Rules

**Append-only. Strictly.**

Existing nodes are immutable once written. A `NodeCompleted` event may carry a `spawn` list — these are new node definitions appended to the graph. The following invariants are enforced at spawn time by the Graph Policy Enforcer:

1. Spawned nodes may only declare dependencies on nodes that already exist in the graph (PENDING, RUNNING, COMPLETED, or the spawning node itself).
2. Adding spawned nodes must not introduce cycles (verified with incremental topological sort in O(k log k) where k = new edges).
3. Spawned node depth = max(deps depth) + 1. Rejected if exceeds `MaxDepth`.
4. Total node count after spawn must not exceed `MaxNodes`.
5. Remaining spawn budget must be > 0.

Violation of any rule raises a `GraphViolationError`, which fails the spawning node (not the entire workflow by default — configurable).

### Synchronization & Visibility

Graph mutations become visible **atomically within the same event**.

`NodeCompleted` carries both the output and the `spawn` list in a single msgpack payload. The rehydrator processes this in one pass — there is no window where the output is visible but the spawned nodes are not.

**Ordering guarantees:**
- Within a workflow: total ordering by `seq` in the WAL.
- Spawned nodes are always sequenced after the spawning node's `NodeCompleted` event.
- No cross-workflow ordering guarantees.

### Scheduler Loop — Precise Definition

```go
func (s *Scheduler) Run(ctx context.Context, runID string) {
    state := s.loadOrCreateState(runID) // snapshot + delta replay

    for {
        ready := state.Graph.ReadyNodes() // PENDING → all deps COMPLETED

        if len(ready) == 0 {
            if state.Graph.IsTerminal() { // all nodes COMPLETED/FAILED/SKIPPED
                return
            }
            // Block until WAL appends a new event for this run
            event := s.wal.WaitNext(ctx, runID, state.LastSeq)
            state.Apply(event)
            continue
        }

        for _, node := range ready {
            state.Graph.MarkReady(node.ID) // PENDING → READY
            s.executor.Enqueue(ctx, runID, node)
        }

        // Executor writes results back as events; Scheduler picks them up
        // on the next WaitNext call. No polling.
        event := s.wal.WaitNext(ctx, runID, state.LastSeq)
        state.Apply(event)
    }
}
```

`state.Apply(event)` mutates the in-memory graph incrementally — no full rebuild on every iteration (see §7).

---

## 2. Determinism vs Non-Determinism

### The Problem

Event sourcing requires determinism: replaying the same log must produce the same result. AI agents are non-deterministic. These two requirements conflict unless explicitly separated.

### The Solution: Two Layers

```
┌─────────────────────────────────────────────────────┐
│  Orchestration Layer  (must be deterministic)        │
│                                                      │
│  - graph traversal and scheduling                    │
│  - retry logic                                       │
│  - saga coordination                                 │
│  - checkpoint handling                               │
│  - fan-out coordination                              │
│                                                      │
│  Input: event log only. No I/O. No time.now().       │
│  Output: next actions (dispatch, compensate, wait)   │
└─────────────────────────────────────────────────────┘
                         ▲
                  events │
                         ▼
┌─────────────────────────────────────────────────────┐
│  Activity Layer  (explicitly non-deterministic)      │
│                                                      │
│  - AI agent calls                                    │
│  - external API calls                                │
│  - file system operations                            │
│  - anything with side effects                        │
│                                                      │
│  Results are recorded in the event log.              │
│  On replay: recorded result is used, not re-executed.│
└─────────────────────────────────────────────────────┘
```

### Replay Semantics

The engine passes a `Replay bool` flag in `StepContext`. When `Replay: true`:
- The executor does NOT dispatch the activity to a worker.
- It reads the recorded `NodeCompleted` output from the event log.
- The orchestration layer sees the same result as the original run.

This means: an AI agent's output (including spawn decisions) is recorded and reused on replay. Agents are never re-invoked during replay. This is the only way to make event-sourced AI workflows deterministic.

### Time

The orchestration layer never calls `time.Now()` directly. All timestamps come from events in the log. The engine exposes a `Clock` interface injected at startup — in production it wraps system time; in tests it is a controllable mock.

---

## 3. Failure Model & Retry

### What Constitutes a Failure

| Category | Definition | Default Action |
|----------|-----------|----------------|
| Transient | timeout, network error, rate limit, 5xx | retry |
| Permanent | invalid input, assertion failure, 4xx | fail node |
| Budget exceeded | token/cost limit hit | fail node |
| Timeout | node exceeded `Deadline` | fail node or compensate |
| Graph violation | spawn produced cycle or exceeded limits | fail spawning node |

Workers classify errors by returning an `ErrorCode` in `CompleteTask`. Grael maps codes to categories via a configurable `ErrorClassifier`.

### Retry Policy

Defined per node, with a workflow-level default:

```go
type RetryPolicy struct {
    MaxAttempts     int           // 0 = infinite (use MaxElapsedTime as bound)
    InitialInterval time.Duration // default: 1s
    MaxInterval     time.Duration // default: 5m
    Multiplier      float64       // default: 2.0 (exponential backoff)
    MaxElapsedTime  time.Duration // wall-clock budget for all retries
    NonRetryable    []string      // error codes that bypass retry immediately
    Jitter          float64       // fraction of interval to randomize (0–1)
}
```

Jitter is mandatory to prevent thundering herd when many nodes fail simultaneously.

### Error Propagation

```
Node failure (retries exhausted)
    ↓
Node enters FAILED state → NodeFailed event written
    ↓
Does node have compensate defined?
    Yes → SagaCoordinator adds to compensation stack
    No  → skip
    ↓
Is workflow configured with error handler node?
    Yes → error handler node becomes READY (receives failed node ID + error)
    No  → workflow transitions to FAILED
         → if sub-workflow: parent node that called it receives NodeFailed
         → parent applies its own retry/error handler logic
```

### Fan-out Partial Failure

Configurable per fan-out directive:

```go
type FanOutFailurePolicy string

const (
    FailFast   FanOutFailurePolicy = "fail_fast"   // cancel remaining, compensate completed
    FailSlow   FanOutFailurePolicy = "fail_slow"   // wait for all, then report all failures
    BestEffort FanOutFailurePolicy = "best_effort" // tolerate failures, pass partial results
)

type FanOutDirective struct {
    Items         []any
    Step          StepDefinition
    Concurrency   int                 // 0 = workflow default
    FailurePolicy FanOutFailurePolicy // default: FailFast
}
```

`BestEffort` result type:

```go
type FanOutResult struct {
    Results  []any    // nil for failed items
    Errors   []error  // nil for successful items
    FailedAt []int    // indices of failed items
}
```

---

## 4. Idempotency & Side Effects

### The Core Problem

After a worker completes an activity and before the engine records `NodeCompleted`, the engine may crash. On restart, it will re-dispatch the activity. The external side effect (API call, database write) would execute twice.

### Solution: Idempotency Keys + At-Least-Once with Worker-Side Dedup

The engine writes `NodeStarted` with an `idempotency_key` **before** dispatching:

```
idempotency_key = base64(sha256(workflowID + ":" + nodeID + ":" + attempt))
```

Workers receive this key and must use it when calling external systems. External systems that support idempotency keys (Stripe, most REST APIs) will deduplicate naturally.

For systems that don't support idempotency keys, workers are responsible for their own deduplication logic.

**Delivery guarantee:** at-least-once to workers. Exactly-once semantics for the workflow graph (the engine checks for existing `NodeCompleted` before re-dispatching after crash recovery).

### Replay Protection

On crash recovery, the engine scans the log for `NodeStarted` events without a subsequent `NodeCompleted`. Before re-dispatching, it checks:

1. Is there a `NodeCompleted` event? → use that, do not dispatch.
2. Is there a `NodeFailed` event? → apply retry policy.
3. Neither? → re-dispatch with the same `idempotency_key` and incremented attempt.

Workers that receive a duplicate dispatch (same `idempotency_key`) should detect this and return the previous result.

---

## 5. Graph Guardrails

### Policy Configuration

```go
type GraphPolicy struct {
    MaxNodes       int           // total nodes in graph, default: 1000
    MaxDepth       int           // longest dependency chain, default: 50
    MaxFanOutWidth int           // items per single fan-out, default: 200
    MaxSpawnBudget int           // total spawns across entire run, default: 500
    MaxRunDuration time.Duration // wall-clock limit, default: 24h
    MaxRetries     int           // per-node ceiling, overrides node config if lower
}
```

Policies are enforced at the point of mutation (spawn, fan-out), not lazily. Violations are synchronous errors.

### Cycle Detection

At spawn time, the Graph Policy Enforcer runs an incremental topological sort:

1. Temporarily add the new nodes and their dependency edges.
2. Attempt `TopoSort()` on the affected subgraph only (not the full graph).
3. If a cycle is detected, reject the spawn and raise `GraphViolationError`.
4. Cost: O(k log k) where k = edges added. Not O(n) on full graph.

### Spawn Budget Tracking

Every `NodeSpawned` event decrements the run's spawn budget. When budget reaches 0, further spawn attempts raise `SpawnBudgetExhausted`. This is a hard stop against agents in infinite loops.

### Fan-out Width Enforcement

Before creating fan-out workers, the engine checks `len(items) <= MaxFanOutWidth`. Violation raises `FanOutWidthExceeded`. Agents that need larger fan-outs must split into batches explicitly.

---

## 6. Concurrency & Backpressure

### Two-Level Semaphore Model

```
Level 1: Per-workflow concurrency
  → max N activities running simultaneously within one run
  → default: 20
  → set per workflow definition or run-level override

Level 2: Global concurrency
  → max M total activities across all runs on this engine instance
  → default: 200
  → hard cap, cannot be overridden per workflow
```

```go
type ConcurrencyManager struct {
    global   *semaphore.Weighted  // M slots total
    perRun   map[string]*semaphore.Weighted  // N slots per runID
}

func (c *ConcurrencyManager) Acquire(ctx context.Context, runID string) error {
    // Must acquire both; acquire global first to avoid deadlock
    if err := c.global.Acquire(ctx, 1); err != nil {
        return err // context cancelled = backpressure signal
    }
    run := c.getOrCreate(runID)
    if err := run.Acquire(ctx, 1); err != nil {
        c.global.Release(1)
        return err
    }
    return nil
}
```

### Backpressure

When both semaphores are full, `Acquire` blocks on `ctx`. The Scheduler's dispatch loop passes a context with the run's deadline. If the context expires while waiting for a slot, the node transitions to `TIMED_OUT`.

This is correct backpressure: slow consumers automatically throttle producers (spawning nodes) because the spawning node is RUNNING and blocking a slot itself.

### Fan-out Specific

Fan-out uses the same semaphore. If the fan-out has 100 items and only 20 slots are available, items dispatch incrementally as slots free up. The `errgroup` in the fan-out coordinator respects the semaphore:

```go
func (e *Executor) RunFanOut(ctx context.Context, directive FanOutDirective) ([]any, error) {
    g, ctx := errgroup.WithContext(ctx)
    results := make([]any, len(directive.Items))

    for i, item := range directive.Items {
        i, item := i, item
        g.Go(func() error {
            if err := e.concurrency.Acquire(ctx, runID); err != nil {
                return err
            }
            defer e.concurrency.Release(runID)

            result, err := e.dispatch(ctx, directive.Step, item)
            results[i] = result
            return err
        })
    }

    return results, g.Wait()
}
```

---

## 7. Event Replay & Performance

### The Problem

Replaying the full event log on every scheduler iteration is O(events) per iteration. For a workflow with 10,000 events, this is untenable.

### Solution: Incremental State + Periodic Snapshots

**Incremental state (primary mechanism):**

The `ExecutionState` is maintained in memory and updated incrementally by applying events one at a time:

```go
type ExecutionState struct {
    Graph   *LiveGraph
    LastSeq uint64
}

func (s *ExecutionState) Apply(event Event) {
    switch event.Type {
    case NodeCompleted:
        p := mustDecode[NodeCompletedPayload](event.Payload)
        s.Graph.MarkCompleted(p.NodeID, p.Output)
        for _, def := range p.SpawnedNodes {
            s.Graph.AddNode(def) // PENDING state
        }
    case NodeFailed:
        p := mustDecode[NodeFailedPayload](event.Payload)
        s.Graph.MarkFailed(p.NodeID, p.Error, p.Attempt)
    // ... other event types
    }
    s.LastSeq = event.Seq
}
```

On startup (or after a crash), the engine loads the latest snapshot and replays only the delta — events with `seq > snapshot.Seq`.

**Snapshots:**

A snapshot serializes the complete `ExecutionState` to a binary blob stored alongside the WAL.

```go
type Snapshot struct {
    Seq        uint64       // WAL position at snapshot time
    WorkflowID string
    State      []byte       // msgpack-encoded ExecutionState
    CRC        uint32       // integrity check
    CreatedAt  time.Time
}
```

**Snapshot trigger policy (hybrid):**

| Trigger | Condition |
|---------|-----------|
| Event count | every 100 events |
| Checkpoint | every time a `CheckpointReached` event is written |
| Time | if no snapshot in last 10 minutes and run is active |

Snapshots are written asynchronously. The engine never blocks execution waiting for a snapshot to complete. If a snapshot write fails, the engine logs the error and continues — worst case is a slower recovery.

**Recovery path:**

```
1. Find latest valid snapshot (CRC check)
2. Deserialize ExecutionState
3. Open WAL, seek to snapshot.Seq
4. Apply all events from snapshot.Seq to current tail
5. Hand off to Scheduler
```

Delta replay is O(events since last snapshot) — bounded by the snapshot interval.

---

## 8. Memory Layer Reliability

### Relationship Detection — Explainability & Confidence

The original design silently classifies relationships. This is unacceptable for production: an agent's context can be silently wrong.

Every relationship now carries explicit metadata:

```go
type MemoryRelationship struct {
    Type       RelationType  // Updates | Extends | Derives
    Confidence float32       // cosine similarity score, 0–1
    Reason     string        // "similarity 0.91 to memory abc123: 'deploy takes ~5min'"
    Manual     bool          // set by agent or human explicitly, bypasses threshold
}
```

Relationships below `MinConfidence` (default: 0.75) are not automatically created. They are surfaced as `CandidateRelationship` — stored separately, visible in the admin UI, awaiting confirmation.

### Consistency Model

| Operation | Consistency | Notes |
|-----------|-------------|-------|
| `Remember` write | synchronous, strong | bbolt transaction committed before return |
| Embedding generation | asynchronous, eventual | embedding computed in background |
| `Recall` / `GetProfile` | reads `isLatest=true` + `hasEmbedding=true` | skips entries awaiting embedding |
| Profile injection | once per run, at start | cached for the entire run duration |

**Critical rule:** memories written during a run are NOT visible to that same run. The profile is assembled once at `WorkflowStarted` and is stable. New memories take effect in the next run. This prevents mid-run context instability and makes the agent's world model deterministic within a run.

### Memory Store Interface (revised)

```go
type MemoryStore interface {
    Remember(ctx context.Context, entry MemoryInput) (*Memory, error)
    Recall(ctx context.Context, q RecallQuery) ([]ScoredMemory, error)
    GetProfile(ctx context.Context, spaceID, taskDescription string) (*MemoryProfile, error)
    Forget(ctx context.Context, id, reason string) error
    GetRelationships(ctx context.Context, id string) ([]MemoryRelationship, error)
    ConfirmRelationship(ctx context.Context, fromID, toID string, rel RelationType) error
}

type ScoredMemory struct {
    Memory     Memory
    Score      float32  // cosine similarity to query
    Highlights []string // matched phrases, for explainability
}

type RecallQuery struct {
    SpaceID    string
    Query      string
    TopK       int
    MinScore   float32
    OnlyStatic bool
    ExcludeForgotten bool
}
```

---

## 9. External Events

### Delivery Guarantee

External events arrive via webhooks (HTTP push) or polling adapters. Webhooks are inherently at-least-once.

**Deduplication window:**

```go
type DeduplicationStore struct {
    // key: externalEventID (provided by source, e.g. GitHub delivery ID)
    // value: processedAt timestamp
    // TTL: 24h (configurable per source)
}
```

Before processing: check if `externalEventID` exists in dedup store. If yes: ack and drop. If no: process, then insert into dedup store. This is a synchronous check inside a bbolt transaction — consistent with the rest of the storage layer.

### Exactly-Once Workflow Trigger

Idempotent workflow creation:

```
workflowRunID = base64(sha256(triggerID + workflowName + workflowVersion + inputHash))
```

`StartWorkflow` is idempotent — if a run with this ID already exists, return it. This prevents double-triggers even if the dedup window expires and the same event is re-delivered weeks later.

### Event Ordering

External events from the same source are ordered by sequence number assigned at ingestion time (monotonic counter in the WAL). Events from different sources are not globally ordered — this is declared and accepted.

For sources where ordering matters (e.g., PR events must be processed in the order they were emitted), the source must provide a sequence number and Grael will buffer out-of-order events up to a configurable window (`MaxReorderWindow`, default: 30s).

---

## 10. Workflow Versioning

### Version Pinning

Every workflow definition is identified by `(name, version)`. Versions follow semver.

```go
type WorkflowRun struct {
    ID              string
    WorkflowName    string
    WorkflowVersion string  // semver, pinned at run creation
    StartedAt       time.Time
    // ...
}
```

A run is pinned to the workflow version active at creation. It will execute that version to completion regardless of subsequent definition changes.

### Compatibility Contract

| Change type | Version bump | In-flight runs |
|-------------|-------------|----------------|
| Bug fix in activity logic | patch (1.0.x) | unaffected (activity logic is in workers, not in the run) |
| Add optional node | minor (1.x.0) | unaffected |
| Remove or rename node | major (x.0.0) | continue on old version |
| Change node dependency structure | major (x.0.0) | continue on old version |

### Migration

For major version upgrades where in-flight runs must be moved:

```go
type WorkflowMigration struct {
    FromVersion string
    ToVersion   string
    Migrate     func(oldState ExecutionState) (ExecutionState, error)
}
```

Migrations are opt-in and explicitly registered. Without a registered migration, old runs run to completion on the old version. The engine simultaneously serves multiple versions of the same workflow.

---

## 11. Security & Isolation

### Worker Authentication

Workers authenticate with mTLS. The engine is the CA; it issues short-lived certificates (24h TTL, auto-rotated) per worker identity. A worker identity declares its `capabilities` — the activity types it can execute.

```go
type WorkerIdentity struct {
    ID           string
    Capabilities []string  // e.g., ["scout", "executor.codex", "reviewer.copilot"]
    TenantID     string
    CertExpiry   time.Time
}
```

The engine only dispatches an activity to a worker whose capabilities include the activity type. A worker cannot receive tasks outside its declared capabilities.

### Multi-Tenant Isolation

All data is namespaced by `tenantID`. The tenantID is embedded in all WAL file paths, bbolt bucket keys, and memory spaceIDs. Cross-tenant data access is impossible by construction — there are no cross-tenant queries, only per-tenant operations.

```
WAL: data/tenants/<tenantID>/events/<workflowID>.log
Memory: bbolt key = tenantID + ":" + spaceID + ":" + memoryID
```

Tenant resource quotas are enforced at the semaphore and policy level:

```go
type TenantQuotas struct {
    MaxConcurrentRuns       int
    MaxConcurrentActivities int
    MaxStorageBytes         int64
    MaxMemoryEntries        int
    MaxMonthlyTokens        int64
}
```

### RBAC

```
Roles:
  grael:admin          full access to everything
  workflow:operator    start/cancel runs, approve checkpoints
  workflow:viewer      read runs and stream events, read-only
  worker               register, poll tasks, complete tasks
  memory:writer        remember, forget
  memory:reader        recall, get profile
```

RBAC is enforced at the gRPC interceptor layer. Every RPC carries a signed JWT; the interceptor validates it and checks the required role before the handler runs.

---

## 12. Observability

Observability is first-class. It is not an afterthought.

### Distributed Tracing (OpenTelemetry)

Every workflow run is a trace. Every node execution is a span. Spans are nested to reflect the graph structure.

```
Trace: workflow run xyz
  Span: scout (duration: 4.2s)
  Span: design-council (duration: 12.1s)
    Span: alice-perspective (duration: 3.8s)
    Span: bob-perspective (duration: 4.1s)
    Span: synthesis (duration: 4.2s)
  Span: executor (duration: 4m32s, attempt: 2)
  Span: review + security (concurrent)
    Span: copilot-review (duration: 8.2s)
    Span: semgrep-scan (duration: 6.1s)
  Span: arbiter (duration: 3.1s)
```

Spans are exported via the standard OTLP exporter. Compatible with Jaeger, Tempo, Honeycomb, Datadog.

### Metrics (Prometheus)

```
# Workflow lifecycle
grael_workflow_started_total{name, version, tenant}
grael_workflow_completed_total{name, version, tenant, status}
grael_workflow_duration_seconds{name, version, tenant}

# Node execution
grael_node_started_total{workflow, node_type, tenant}
grael_node_completed_total{workflow, node_type, tenant, status}
grael_node_duration_seconds{workflow, node_type, tenant}
grael_node_retries_total{workflow, node_type, tenant}
grael_node_attempt_number{workflow, node_type}

# Dynamic graph
grael_spawn_total{workflow, spawning_node}
grael_fanout_width{workflow, node}

# Concurrency
grael_concurrent_activities{tenant}
grael_queue_depth
grael_semaphore_wait_seconds

# Memory
grael_memory_recall_duration_seconds{tenant}
grael_memory_store_entries{tenant, space_type}
grael_memory_relationship_auto_created{type, tenant}

# Storage
grael_wal_size_bytes{workflow}
grael_snapshot_age_seconds{workflow}

# Cost (AI-specific)
grael_tokens_consumed_total{workflow, node_type, tenant, model}
grael_estimated_cost_usd{workflow, tenant}
```

### Debug CLI

```
grael run show <id>               full run state, graph, current positions
grael run events <id>             stream raw events from WAL
grael run replay <id> --to <seq>  rebuild state at specific point in history
grael run graph <id>              export graph as DOT for visualization
grael memory show <space>         all memories in a space with relationships
grael memory search <space> <q>   test recall query against live data
```

---

## 13. Human-in-the-Loop Checkpoints

### Timeout & Escalation

A checkpoint that blocks forever is a production incident. Every checkpoint requires an explicit timeout policy:

```go
type CheckpointConfig struct {
    Timeout         time.Duration     // required, no default
    OnTimeout       TimeoutAction     // Fail | AutoApprove | Escalate
    EscalateTo      []NotificationTarget
    EscalationDelay time.Duration     // wait before escalating, default: Timeout/2
    ReminderEvery   time.Duration     // periodic reminders before timeout, 0 = disabled
}

type TimeoutAction string

const (
    TimeoutFail        TimeoutAction = "fail"         // node fails, saga unwinds
    TimeoutAutoApprove TimeoutAction = "auto_approve" // implicit approval
    TimeoutEscalate    TimeoutAction = "escalate"     // notify + extend timeout once
)
```

**Escalation flow:**

```
Checkpoint reached
    ↓ notify primary approvers
    ↓ [ReminderEvery] → send reminders
    ↓ [EscalationDelay] → notify EscalateTo + extend timeout by EscalationDelay
    ↓ [Timeout reached] → apply OnTimeout action
```

`TimeoutEscalate` extends the deadline exactly once. If the extended deadline also expires, the node fails unconditionally.

### Approval Audit Trail

Every checkpoint approval is recorded as a `CheckpointApproved` event with:

```go
type CheckpointApprovedPayload struct {
    NodeID    string
    ApprovedBy string    // user ID
    Comment   string     // optional
    Method    string     // "manual" | "auto_timeout" | "api"
    ApprovedAt time.Time
}
```

This is part of the immutable event log — approval history cannot be modified.

---

## 14. Cost & Resource Control

### Budget Enforcement

```go
type CostBudget struct {
    MaxInputTokens  int64    // across all AI calls in the run
    MaxOutputTokens int64
    MaxTotalUSD     float64
    OnExceeded      BudgetAction // Fail | Pause | Warn
}

type BudgetAction string

const (
    BudgetFail  BudgetAction = "fail"   // fail current node, saga unwinds
    BudgetPause BudgetAction = "pause"  // pause run, await human decision
    BudgetWarn  BudgetAction = "warn"   // continue but emit warning metric
)
```

Workers report token usage in `CompleteTaskRequest`:

```go
type CompleteTaskRequest struct {
    TaskID    string
    Output    []byte
    Error     *TaskError
    TokenUsage *TokenUsage  // nil for non-AI activities
}

type TokenUsage struct {
    InputTokens  int64
    OutputTokens int64
    Model        string
    EstimatedUSD float64
}
```

The engine accumulates usage into `CostTracker` (stored in the run state, persisted in the WAL). Before dispatching any AI activity, the engine checks the remaining budget. If the budget is already exceeded, the node is failed immediately without dispatch.

```go
type CostTracker struct {
    InputTokens  int64
    OutputTokens int64
    TotalUSD     float64
    ByNode       map[string]NodeCost
}
```

### Resource Limits per Run

```go
type RunLimits struct {
    MaxDuration     time.Duration
    MaxNodes        int
    MaxSpawnBudget  int
    MaxFanOutWidth  int
    CostBudget      *CostBudget
    // Tenant quotas are applied on top — stricter bound wins
}
```

---

## Storage

### Event Store — Custom WAL

```
data/tenants/<tenantID>/events/
  <workflowID>.log    binary append-only, CRC32-framed records
  <workflowID>.idx    memory-mapped flat array: (seq uint64, offset uint64)
  <workflowID>.snap   latest snapshot (msgpack ExecutionState + CRC)
```

**Record format:**
```
┌──────────┬────────┬──────────┬────────────┬──────────┬────────┐
│ 4B magic │ 8B seq │ 8B ts_ns │ 4B pay_len │ payload  │ 4B CRC │
└──────────┴────────┴──────────┴────────────┴──────────┴────────┘
```

Magic = `0xGRAE` — distinguishes Grael records from filesystem garbage.

**Crash recovery:** on open, scan from tail to first valid CRC. Truncate past the last valid record. Index rebuild from scratch if `.idx` is missing or its last entry doesn't match the log tail.

**File rotation:** when `.log` exceeds `MaxSegmentSize` (default: 256MB), it is sealed (renamed `.log.1`) and a new `.log` is started. The index spans segments. Old segments are retained for the snapshot window and then compacted.

**Single writer invariant:** all appends go through a dedicated goroutine with an unbuffered channel. Readers use the memory-mapped index — concurrent reads are lock-free.

### Memory Store — bbolt + Pure Go Vector Math

```
data/tenants/<tenantID>/memory.db   (bbolt)

Buckets:
  memories/    id → MemoryRecord (msgpack)
  spaces/      spaceID → sorted []memoryID
  relations/   "fromID:type" → []toID (msgpack)
  embeddings/  id → []float32 (little-endian binary)
  dedup/       externalEventID → processedAt (TTL via ForgetAfter scan)
```

Vector search: load all embeddings for `spaceID` where `isLatest=true` and `hasEmbedding=true`, rank by cosine similarity, return top-K. For datasets up to ~50k entries, brute-force at ~5ms is acceptable.

Beyond 50k: build an in-memory HNSW index at startup (`coder/hnsw`, pure Go, zero CGO). The index is rebuilt from bbolt on restart — no separate persistence needed. Rebuild time for 100k vectors: ~2s.

### Summary

|                    | Event Store         | Memory Store              |
|--------------------|---------------------|---------------------------|
| Implementation     | Custom WAL          | bbolt + pure Go cosine    |
| CGO                | none                | none                      |
| Write              | append + fsync      | bbolt transaction         |
| Read               | mmap sequential     | bbolt + vector scan       |
| Crash recovery     | CRC32 + truncate    | bbolt ACID                |
| Snapshots          | per-run .snap file  | n/a (bbolt is durable)    |
| Scale limit        | effectively unbounded | ~50k/space before HNSW  |

---

## gRPC API

```protobuf
service Grael {
    // Workflow management
    rpc StartWorkflow(StartWorkflowRequest)   returns (WorkflowRun);
    rpc GetRun(GetRunRequest)                 returns (WorkflowRun);
    rpc CancelRun(CancelRequest)              returns (google.protobuf.Empty);
    rpc ListRuns(ListRunsRequest)             returns (ListRunsResponse);

    // Human-in-the-loop
    rpc ApproveCheckpoint(ApproveRequest)     returns (google.protobuf.Empty);
    rpc RejectCheckpoint(RejectRequest)       returns (google.protobuf.Empty);

    // Real-time
    rpc StreamEvents(StreamRequest)           returns (stream Event);
    rpc StreamRunGraph(StreamRequest)         returns (stream GraphSnapshot);

    // Activity worker protocol
    rpc RegisterWorker(WorkerInfo)            returns (WorkerRegistration);
    rpc Heartbeat(HeartbeatRequest)           returns (google.protobuf.Empty);
    rpc PollTask(PollRequest)                 returns (stream Task);
    rpc CompleteTask(CompleteTaskRequest)     returns (google.protobuf.Empty);

    // Memory
    rpc Remember(RememberRequest)             returns (Memory);
    rpc Recall(RecallRequest)                 returns (RecallResponse);
    rpc GetProfile(GetProfileRequest)         returns (MemoryProfile);
    rpc Forget(ForgetRequest)                 returns (google.protobuf.Empty);
}
```

Workers maintain a heartbeat (default: every 10s). If the engine does not receive a heartbeat within `HeartbeatTimeout` (default: 30s), the worker is considered dead and its in-progress tasks are re-queued.

---

## Repository Structure

```
grael/
  cmd/
    grael/           main.go — server bootstrap, config, signal handling
  internal/
    core/
      graph.go       LiveGraph — mutable structure, ReadyNodes(), Apply(event)
      scheduler.go   main execution loop, WaitNext, Apply delta
      rehydrator.go  snapshot load + WAL delta replay
      state.go       ExecutionState, incremental Apply
    execution/
      executor.go    worker dispatch, result ingestion
      fanout.go      fan-out coordinator, errgroup + semaphore
      saga.go        SagaCoordinator, compensation stack, reverse unwind
      checkpoint.go  checkpoint state, timeout ticker, escalation
    policy/
      graph.go       cycle detection, depth/node/spawn limit enforcement
      concurrency.go two-level semaphore, backpressure
      cost.go        budget tracker, pre-dispatch budget check
    events/
      wal.go         append, read, mmap index, single writer goroutine
      recovery.go    CRC scan, truncation, index rebuild
      snapshot.go    write/read/validate .snap files
      types.go       all EventType constants and payload structs
    memory/
      store.go       MemoryStore interface
      bbolt.go       bbolt-backed implementation
      cosine.go      pure Go dot product, ranking
      hnsw.go        optional HNSW index, built in-memory from bbolt
      relations.go   auto-detection: Updates/Extends/Derives + confidence
      profile.go     GetProfile assembly — static + episodic
    worker/
      registry.go    worker identity, capabilities, certificate validation
      heartbeat.go   heartbeat tracking, dead worker detection
      poller.go      task queue, dispatch, re-queue on worker death
    security/
      rbac.go        role definitions, permission checks
      jwt.go         JWT validation middleware for gRPC
      mtls.go        certificate issuance and validation
    observability/
      tracing.go     OpenTelemetry span creation and propagation
      metrics.go     Prometheus metric definitions and recording
    external/
      dedup.go       external event deduplication window
      ingestion.go   webhook receiver, sequence assignment
  proto/
    grael.proto      gRPC contract — source of truth for all clients
  pkg/
    client/          generated Go client + thin helpers
  config/
    grael.yaml       default configuration with all limits documented
```

---

## Tech Stack

| Component       | Choice                              | Reason                                      |
|-----------------|-------------------------------------|---------------------------------------------|
| Language        | Go 1.22+                            | goroutines, single binary, context          |
| Event store     | Custom WAL                          | zero CGO, mmap reads, perfect access fit    |
| Memory store    | bbolt + pure Go cosine / HNSW       | zero CGO, ACID, scales to 50k+ gracefully   |
| Serialization   | msgpack (`vmihailenco/msgpack`)      | compact, fast, no schema required           |
| gRPC            | `google.golang.org/grpc`            | typed API, bidirectional streaming, mTLS    |
| Concurrency     | `golang.org/x/sync/errgroup` + `semaphore` | fan-out + backpressure              |
| Tracing         | `go.opentelemetry.io/otel`          | vendor-neutral, standard                    |
| Metrics         | `github.com/prometheus/client_golang` | standard Go metrics                       |
| Logging         | `uber-go/zap`                       | structured, zero-allocation hot path       |
| Config          | `github.com/spf13/viper`            | env + file, standard Go ecosystem           |
| HNSW            | `coder/hnsw`                        | pure Go, no CGO                             |

---

## What Changed From the Original Specification

| Area | Original | Revised |
|------|----------|---------|
| Scheduler | full rehydrate on every iteration | incremental `state.Apply(event)` + snapshots |
| Snapshots | not specified | hybrid trigger: every 100 events or at checkpoint |
| Node lifecycle | no formal states | explicit 9-state machine with typed transitions |
| Spawn rules | append-only implied | formally specified: validation at spawn time, cycle detection, budget |
| Fan-out | unlimited goroutines | semaphore-bounded, `FailurePolicy` per directive |
| Retry | not specified | `RetryPolicy` per node with jitter, `NonRetryable` error codes |
| Idempotency | not specified | idempotency keys on `NodeStarted`, at-least-once with worker dedup |
| Memory consistency | not specified | async embedding, stable profile per run, `MinConfidence` threshold |
| Memory relations | silent auto-detection | `Confidence` + `Reason` on every relation, `CandidateRelationship` below threshold |
| External events | not specified | deduplication window (bbolt), idempotent workflow creation |
| Workflow versioning | not specified | semver pinning at run creation, compatibility contract, migration hooks |
| Security | not mentioned | mTLS worker auth, RBAC on all RPCs, per-tenant namespacing |
| Observability | not mentioned | OpenTelemetry traces, Prometheus metrics, debug CLI |
| Checkpoints | no timeout | required `Timeout`, escalation chain, audit trail in WAL |
| Cost control | not mentioned | `CostBudget` per run, pre-dispatch budget check, per-node token tracking |
| Determinism | implicit | formal two-layer model: deterministic orchestration / recorded activities |
| Worker liveness | not specified | heartbeat with 30s timeout, automatic task re-queue |

---

## Competitive Landscape

|                        | Temporal | LangGraph | Restate | **Grael** |
|------------------------|----------|-----------|---------|-----------|
| Dynamic DAG (spawn)    | —        | ✓         | —       | **✓**     |
| Event sourcing         | ✓        | —         | partial | **✓**     |
| Agent memory layer     | —        | ✓         | —       | **✓**     |
| Single binary          | —        | —         | ✓       | **✓**     |
| Saga / compensation    | ✓        | —         | partial | **✓**     |
| Fan-out first-class    | —        | —         | —       | **✓**     |
| Formal retry policy    | ✓        | —         | ✓       | **✓**     |
| Cost budget tracking   | —        | —         | —       | **✓**     |
| Checkpoint escalation  | —        | —         | —       | **✓**     |
| Workflow versioning    | ✓        | —         | —       | **✓**     |
| mTLS worker auth       | ✓        | —         | ✓       | **✓**     |
| OpenTelemetry native   | ✓        | —         | ✓       | **✓**     |

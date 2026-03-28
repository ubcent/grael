# Grael v1 UAT Matrix

This file is the planning index for user acceptance testing in Grael v1.

It maps capabilities to observable scenarios. Each row should eventually point to one or more concrete UAT specs created from `docs/UAT_TEMPLATE.md`.

The matrix is intentionally capability-first. It is not a task list and it is not a unit test inventory.

---

## Status Legend

- `planned`: scenario identified but not yet written in full
- `specified`: scenario written as a full UAT spec
- `automated`: scenario covered by an executable acceptance/integration test
- `deferred`: intentionally outside the current wave

---

## Matrix

| Capability | UAT ID | Scenario | Why it matters | Primary surfaces | Status |
|---|---|---|---|---|---|
| `C1` | `UAT-C1-01` | [Restart from WAL and snapshot resumes a run without losing committed progress](docs/uat/UAT-C1-01-restart-recovery.md) | Proves durability and recovery | `StartRun`, process restart, `GetRun`, `ListEvents` | `specified` |
| `C1` | `UAT-C1-02` | [Corrupt WAL tail does not invalidate valid prior history](docs/uat/UAT-C1-02-corrupt-wal-tail.md) | Proves storage resilience | startup behavior, `GetRun`, `ListEvents` | `specified` |
| `C2` | `UAT-C2-01` | [Completed nodes do not re-dispatch after restart or replay](docs/uat/UAT-C2-01-no-redispatch-after-completion.md) | Proves terminal-state correctness | process restart, worker polling, `ListEvents` | `specified` |
| `C2` | `UAT-C2-02` | [Ready dependency unblocking follows recorded event history only](docs/uat/UAT-C2-02-dependency-unblocking-from-recorded-history.md) | Proves deterministic orchestration | `GetRun`, `ListEvents` | `specified` |
| `C3` | `UAT-C3-01` | [A simple linear workflow progresses from start to completion through the run loop](docs/uat/UAT-C3-01-linear-run-loop.md) | Proves the engine can drive execution end to end | `StartRun`, worker RPCs, `GetRun`, `ListEvents` | `specified` |
| `C4` | `UAT-C4-01` | [Worker polls, receives a task, completes it, and the run finishes successfully](docs/uat/UAT-C4-01-worker-success.md) | Proves the worker execution model works | `PollTask`, `CompleteTask`, `GetRun` | `specified` |
| `C4` | `UAT-C4-02` | [Heartbeat loss expires leases and prevents a task from hanging forever](docs/uat/UAT-C4-02-heartbeat-lease-expiry.md) | Proves liveness detection | `Heartbeat`, `GetRun`, `ListEvents` | `specified` |
| `C4` | `UAT-C4-03` | [Late `CompleteTask` after lease expiry is rejected](docs/uat/UAT-C4-03-late-complete-rejected.md) | Proves lease finality | `CompleteTask`, `ListEvents` | `specified` |
| `C4` | `UAT-C4-04` | [A networked worker can register, poll, heartbeat, and complete work over gRPC](docs/uat/UAT-C4-04-network-worker-over-grpc.md) | Proves the worker contract survives transport, not only in-process calls | gRPC worker RPCs, `GetRun`, `ListEvents` | `specified` |
| `C5` | `UAT-C5-01` | [Retryable failure leads to backoff and successful re-execution](docs/uat/UAT-C5-01-retry-backoff-success.md) | Proves automatic retry | `FailTask`, `GetRun`, `ListEvents` | `specified` |
| `C5` | `UAT-C5-02` | [Overdue retry timer fires after restart and the node continues](docs/uat/UAT-C5-02-overdue-retry-after-restart.md) | Proves timer durability | process restart, `GetRun`, `ListEvents` | `specified` |
| `C5` | `UAT-C5-03` | [Execution deadline failure marks the node failed with timeout semantics](docs/uat/UAT-C5-03-execution-deadline-timeout.md) | Proves stuck work cannot stall forever | `GetRun`, `ListEvents` | `specified` |
| `C5` | `UAT-C5-04` | [Absolute deadline still fires while a node awaits approval](docs/uat/UAT-C5-04-absolute-deadline-during-approval.md) | Proves checkpoints cannot bypass hard deadlines | `ApproveCheckpoint`, `GetRun`, `ListEvents` | `specified` |
| `C6` | `UAT-C6-01` | [A completed node spawns new nodes and the graph grows during execution](docs/uat/UAT-C6-01-living-dag-spawn.md) | Proves the living DAG differentiator | `CompleteTask`, `GetRun`, `ListEvents` | `specified` |
| `C6` | `UAT-C6-02` | [Spawned graph survives restart and rehydrates identically](docs/uat/UAT-C6-02-spawned-graph-restart-durability.md) | Proves dynamic graph durability | process restart, `GetRun`, `ListEvents` | `specified` |
| `C6` | `UAT-C6-03` | [Invalid spawn that creates a cycle is rejected cleanly](docs/uat/UAT-C6-03-cycle-spawn-rejected.md) | Proves graph safety | `CompleteTask`, `GetRun`, `ListEvents` | `specified` |
| `C7` | `UAT-C7-01` | [Graceful cancel stops remaining work and reaches a terminal cancelled outcome](docs/uat/UAT-C7-01-graceful-cancel.md) | Proves run controllability | `CancelRun`, `GetRun`, `ListEvents` | `specified` |
| `C7` | `UAT-C7-02` | [Permanent failure triggers sequential compensation of completed nodes only](docs/uat/UAT-C7-02-failure-triggers-compensation.md) | Proves basic saga semantics | `FailTask`, `GetRun`, `ListEvents` | `specified` |
| `C7` | `UAT-C7-03` | [Compensation resumes correctly after restart mid-unwind](docs/uat/UAT-C7-03-compensation-resumes-after-restart.md) | Proves compensation durability | process restart, `GetRun`, `ListEvents` | `specified` |
| `C8` | `UAT-C8-01` | [A checkpoint pauses one node while unrelated nodes keep running](docs/uat/UAT-C8-01-checkpoint-pauses-one-node.md) | Proves non-blocking human approval | `CompleteTask`, `ApproveCheckpoint`, `GetRun` | `specified` |
| `C8` | `UAT-C8-02` | [Approval after restart resumes the waiting node correctly](docs/uat/UAT-C8-02-approval-after-restart.md) | Proves checkpoint durability | process restart, `ApproveCheckpoint`, `GetRun` | `specified` |
| `C8` | `UAT-C8-03` | [Checkpoint timeout fails the waiting node when approval never arrives](docs/uat/UAT-C8-03-checkpoint-timeout.md) | Proves bounded waiting | `GetRun`, `ListEvents` | `specified` |
| `C9` | `UAT-C9-01` | [`GetRun` returns a coherent current-state view during active execution](docs/uat/UAT-C9-01-get-run-coherent-view.md) | Proves minimal operational visibility | `GetRun` | `specified` |
| `C9` | `UAT-C9-02` | [`ListEvents` returns the raw execution history in causal order](docs/uat/UAT-C9-02-list-events-causal-history.md) | Proves forensic visibility | `ListEvents` | `specified` |
| `C9` | `UAT-C9-03` | [`StartRun` rejects immediately when the engine is at hard capacity](docs/uat/UAT-C9-03-start-run-capacity-reject.md) | Proves the no-admission-queue contract | `StartRun` | `specified` |
| `C9` | `UAT-C9-04` | [Remote orchestration over gRPC can start, control, and inspect a run without changing semantics](docs/uat/UAT-C9-04-grpc-orchestration-surface.md) | Proves the public API is really available over the intended network transport | gRPC orchestration RPCs, `GetRun`, `ListEvents` | `specified` |
| `C9` | `UAT-C9-05` | [`StreamEvents` replays committed history from `from_seq` and then streams live committed appends](docs/uat/UAT-C9-05-streamevents-follows-committed-history.md) | Proves live observation does not require a second source of truth | `StreamEvents`, `ListEvents` | `specified` |
| `C10` | `UAT-C10-01` | [A workflow definition with retry, deadline, checkpoint, and compensation metadata executes as declared](docs/uat/UAT-C10-01-definition-metadata-executes-as-declared.md) | Proves the minimum authoring contract is sufficient | `StartRun`, worker RPCs, `GetRun` | `specified` |
| `C10` | `UAT-C10-02` | [A Go worker registered by activity type can handle tasks through the thin SDK seam](docs/uat/UAT-C10-02-go-sdk-worker-seam.md) | Proves the SDK boundary is viable | worker registration, `PollTask`, `CompleteTask` | `specified` |
| `C10` | `UAT-C10-03` | [A node with declared input reaches the worker with the same per-node payload across spawn and restart](docs/uat/UAT-C10-03-node-input-reaches-worker.md) | Proves node-scoped work context is explicit, durable, and restart-safe | `StartRun`, `CompleteTask`, `PollTask`, `GetRun`, `ListEvents` | `specified` |
| `C10` | `UAT-C10-04` | [An SDK fan-out helper expands into ordinary spawned nodes without changing execution semantics](docs/uat/UAT-C10-04-sdk-fanout-helper-lowers-to-spawn.md) | Proves SDK convenience does not introduce a second orchestration model | Go SDK, `CompleteTask`, `GetRun`, `ListEvents` | `specified` |
| `C11` | `UAT-C11-01` | [The core demo workflow completes with dynamic spawn, retry, approval, and recovery](docs/uat/UAT-C11-01-core-demo-e2e.md) | Proves the v1 product story works end to end | full API surface, worker RPCs, restart behavior | `specified` |

---

## Usage Rules

When adding a full UAT spec:

1. Create a dedicated document from `docs/UAT_TEMPLATE.md`.
2. Add or update the corresponding row in this matrix.
3. Keep the scenario phrased in user-visible terms.
4. Reference only observable system surfaces in the matrix.

---

## Recommended Spec Location

If UAT specs start multiplying, store them under a dedicated directory such as:

- `docs/uat/`
- `docs/specs/uat/`

For now, this repository can stay lightweight and use the matrix plus individual top-level files as needed.

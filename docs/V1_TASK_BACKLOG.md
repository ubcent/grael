# Grael v1 Spec-Driven Task Backlog

This document turns the v1 capability map and UAT set into an implementation backlog.

Each task is defined as a verifiable slice of capability, not as a vague code-area placeholder. A task is considered complete only when its stated acceptance outcome is implemented and the linked UATs can pass.

Primary planning inputs:

- `docs/V1_CANONICAL_BASELINE.md`
- `docs/V1_CAPABILITY_MAP.md`
- `docs/UAT_MATRIX.md`
- `docs/uat/`

---

## Backlog Rules

Each task should:

- map to one primary capability
- have explicit dependencies
- have a concrete implementation slice
- point to one or more UATs as definition of done
- avoid pulling cut or post-v1 scope into the task

Status model for future tracking:

- `todo`
- `in_progress`
- `blocked`
- `done`

---

## Wave 1: Runtime Bedrock

### T1. WAL append format and CRC validation

- `Status`: `todo`
- `Capability`: `C1`
- `Goal`: Define and implement append-only WAL records with per-event integrity checking.
- `Scope`:
  - append encoded event records to per-run WAL
  - store monotonic sequence per appended event
  - validate CRC on scan
- `Depends on`: none
- `Definition of Done`:
  - supports the storage foundation required by [UAT-C1-01](docs/uat/UAT-C1-01-restart-recovery.md)
  - supports corruption detection required by [UAT-C1-02](docs/uat/UAT-C1-02-corrupt-wal-tail.md)

### T2. WAL scan and corruption-boundary recovery

- `Status`: `todo`
- `Capability`: `C1`
- `Goal`: Recover valid event history up to the last good WAL record without treating a corrupt tail as full-run loss.
- `Scope`:
  - sequential WAL scan
  - stop safely on corrupt tail
  - expose valid event prefix for recovery
- `Depends on`: `T1`
- `Definition of Done`:
  - [UAT-C1-02](docs/uat/UAT-C1-02-corrupt-wal-tail.md)

### T3. Snapshot write and snapshot-plus-delta restore

- `Status`: `todo`
- `Capability`: `C1`
- `Goal`: Support restoring run state from the latest valid snapshot plus WAL delta.
- `Scope`:
  - snapshot persistence
  - snapshot integrity validation
  - restore from snapshot and replay remaining events
- `Depends on`: `T1`, `T2`
- `Definition of Done`:
  - [UAT-C1-01](docs/uat/UAT-C1-01-restart-recovery.md)

### T4. ExecutionState skeleton and event application core

- `Status`: `todo`
- `Capability`: `C2`
- `Goal`: Build the derived runtime state model driven entirely by event application.
- `Scope`:
  - `ExecutionState`
  - basic node lifecycle projection
  - run lifecycle projection
  - `Apply(event)`
- `Depends on`: `T1`
- `Definition of Done`:
  - enables [UAT-C2-01](docs/uat/UAT-C2-01-no-redispatch-after-completion.md)
  - enables [UAT-C2-02](docs/uat/UAT-C2-02-dependency-unblocking-from-recorded-history.md)

### T5. Deterministic dependency readiness and terminal-state enforcement

- `Status`: `todo`
- `Capability`: `C2`
- `Goal`: Make readiness and terminal-node behavior fully derived from recorded state.
- `Scope`:
  - dependency satisfaction rules
  - no redispatch after terminal completion
  - deterministic node readiness transitions
- `Depends on`: `T4`
- `Definition of Done`:
  - [UAT-C2-01](docs/uat/UAT-C2-01-no-redispatch-after-completion.md)
  - [UAT-C2-02](docs/uat/UAT-C2-02-dependency-unblocking-from-recorded-history.md)

### T6. Pure scheduler and deterministic command ordering

- `Status`: `todo`
- `Capability`: `C2`
- `Goal`: Implement `Scheduler.Decide(state) -> []Command` with no I/O and stable ordering.
- `Scope`:
  - pure scheduling interface
  - deterministic command precedence
  - no wall-clock reads in scheduler
- `Depends on`: `T4`, `T5`
- `Definition of Done`:
  - supports [UAT-C2-02](docs/uat/UAT-C2-02-dependency-unblocking-from-recorded-history.md)
  - supports [UAT-C3-01](docs/uat/UAT-C3-01-linear-run-loop.md)

### T7. Run loop and command processor baseline

- `Status`: `todo`
- `Capability`: `C3`
- `Goal`: Connect rehydrated state, scheduler decisions, and event persistence into one execution loop.
- `Scope`:
  - `RunLoop`
  - command execution path
  - wait for new run events
  - continue until terminal outcome
- `Depends on`: `T3`, `T6`
- `Definition of Done`:
  - [UAT-C3-01](docs/uat/UAT-C3-01-linear-run-loop.md)
  - [UAT-C1-01](docs/uat/UAT-C1-01-restart-recovery.md)

---

## Wave 2: Real Execution Reliability

### T8. Worker registry by activity type

- `Status`: `done`
- `Capability`: `C4`
- `Goal`: Track available workers by activity type and route dispatch to compatible handlers.
- `Scope`:
  - worker registration lifecycle
  - activity-type lookup
  - task routing hooks for scheduler/processor
- `Depends on`: `T7`
- `Definition of Done`:
  - supports [UAT-C4-01](docs/uat/UAT-C4-01-worker-success.md)
  - supports [UAT-C10-02](docs/uat/UAT-C10-02-go-sdk-worker-seam.md)

### T9. Public worker RPC surface: PollTask, CompleteTask, FailTask, Heartbeat

- `Status`: `done`
- `Capability`: `C4`
- `Goal`: Expose the minimal worker protocol required by v1.
- `Scope`:
  - long-poll `PollTask`
  - success completion RPC
  - failure completion RPC
  - heartbeat RPC
- `Depends on`: `T8`
- `Definition of Done`:
  - [UAT-C4-01](docs/uat/UAT-C4-01-worker-success.md)
  - [UAT-C4-02](docs/uat/UAT-C4-02-heartbeat-lease-expiry.md)

### T10. Lease grant on dispatch and active-attempt tracking

- `Status`: `done`
- `Capability`: `C4`
- `Goal`: Make task ownership explicit and persistently tracked per attempt.
- `Scope`:
  - lease grant events
  - active attempt tracking
  - node start linkage to lease ownership
- `Depends on`: `T8`, `T9`
- `Definition of Done`:
  - [UAT-C4-01](docs/uat/UAT-C4-01-worker-success.md)
  - [UAT-C4-03](docs/uat/UAT-C4-03-late-complete-rejected.md)

### T11. Heartbeat timeout and lease expiry monitor

- `Status`: `done`
- `Capability`: `C4`
- `Goal`: Detect lost worker liveness and expire held leases.
- `Scope`:
  - heartbeat freshness tracking
  - lease expiry emission
  - bulk expiry for dead worker ownership
- `Depends on`: `T10`
- `Definition of Done`:
  - [UAT-C4-02](docs/uat/UAT-C4-02-heartbeat-lease-expiry.md)

### T12. Reject stale worker results after lease expiry

- `Status`: `done`
- `Capability`: `C4`
- `Goal`: Ensure expired attempts can no longer complete successfully.
- `Scope`:
  - attempt validity checks on `CompleteTask`
  - attempt validity checks on `FailTask`
  - stale-result rejection path
- `Depends on`: `T10`, `T11`
- `Definition of Done`:
  - [UAT-C4-03](docs/uat/UAT-C4-03-late-complete-rejected.md)
  - supports [UAT-C2-01](docs/uat/UAT-C2-01-no-redispatch-after-completion.md)

### T13. Timer scheduling and firing engine

- `Status`: `done`
- `Capability`: `C5`
- `Goal`: Persist timers and fire them from a recoverable timer manager.
- `Scope`:
  - `TimerScheduled`
  - `TimerFired`
  - in-memory min-heap rebuilt from WAL
  - timer catch-up on restart
- `Depends on`: `T7`
- `Definition of Done`:
  - [UAT-C5-01](docs/uat/UAT-C5-01-retry-backoff-success.md)
  - [UAT-C5-02](docs/uat/UAT-C5-02-overdue-retry-after-restart.md)

### T14. Retry policy and retry backoff scheduling

- `Status`: `done`
- `Capability`: `C5`
- `Goal`: Turn retryable failures into scheduled re-execution attempts.
- `Scope`:
  - retry policy evaluation
  - retry timer scheduling
  - retry requeue path
- `Depends on`: `T9`, `T13`
- `Definition of Done`:
  - [UAT-C5-01](docs/uat/UAT-C5-01-retry-backoff-success.md)
  - [UAT-C5-02](docs/uat/UAT-C5-02-overdue-retry-after-restart.md)

### T15. Execution deadline enforcement

- `Status`: `done`
- `Capability`: `C5`
- `Goal`: Prevent stuck running nodes from blocking the workflow forever.
- `Scope`:
  - execution deadline timer scheduling
  - timeout-to-failure semantics
  - post-timeout retry or terminal handling
- `Depends on`: `T10`, `T13`
- `Definition of Done`:
  - [UAT-C5-03](docs/uat/UAT-C5-03-execution-deadline-timeout.md)

### T16. Absolute deadline enforcement

- `Status`: `done`
- `Capability`: `C5`
- `Goal`: Enforce a hard node deadline that continues across approval waiting.
- `Scope`:
  - absolute deadline timer scheduling
  - timeout behavior while node is active or awaiting approval
- `Depends on`: `T13`, `T15`
- `Definition of Done`:
  - [UAT-C5-04](docs/uat/UAT-C5-04-absolute-deadline-during-approval.md)

Implementation note:

- runtime-level absolute-deadline timer scheduling and timeout failure semantics are in place
- checkpoint/`AWAITING_APPROVAL` behavior now exists, and absolute deadline is enforced while a node waits for approval

---

## Wave 3: Product Differentiator

### T17. Runtime spawn payload handling

- `Status`: `done`
- `Capability`: `C6`
- `Goal`: Allow successful node completion to declare spawned nodes.
- `Scope`:
  - completion payload support for `SpawnedNodes`
  - event application that inserts new nodes into run graph
  - initial visibility of spawned nodes in derived state
- `Depends on`: `T9`, `T10`, `T4`
- `Definition of Done`:
  - [UAT-C6-01](docs/uat/UAT-C6-01-living-dag-spawn.md)

### T18. Dynamic graph scheduling and persisted rehydration

- `Status`: `done`
- `Capability`: `C6`
- `Goal`: Make spawned nodes runnable and durable across restart.
- `Scope`:
  - readiness for spawned nodes
  - persisted reconstruction of expanded graph
  - restart continuity for mutated graph
- `Depends on`: `T17`, `T3`, `T6`
- `Definition of Done`:
  - [UAT-C6-01](docs/uat/UAT-C6-01-living-dag-spawn.md)
  - [UAT-C6-02](docs/uat/UAT-C6-02-spawned-graph-restart-durability.md)

### T19. Spawn validation and cycle rejection

- `Status`: `done`
- `Capability`: `C6`
- `Goal`: Reject invalid graph mutations before they corrupt the run graph.
- `Scope`:
  - dependency reference validation
  - cycle detection
  - rejection or failure path for invalid spawn submissions
- `Depends on`: `T17`
- `Definition of Done`:
  - [UAT-C6-03](docs/uat/UAT-C6-03-cycle-spawn-rejected.md)

---

## Wave 4: Operational Control Flow

### T20. CancelRun API and cancellation request persistence

- `Status`: `done`
- `Capability`: `C7`
- `Goal`: Let operators request graceful cancellation for an active run.
- `Scope`:
  - `CancelRun` API
  - cancellation request event
  - run-level cancellation tracking
- `Depends on`: `T7`
- `Definition of Done`:
  - supports [UAT-C7-01](docs/uat/UAT-C7-01-graceful-cancel.md)

### T21. Graceful cancel propagation by node state

- `Status`: `done`
- `Capability`: `C7`
- `Goal`: Apply cancellation coherently to running, ready, pending, and waiting nodes.
- `Scope`:
  - cancel pending/ready nodes
  - propagate cancellation to running work
  - close run once outstanding work reaches cancel handling
- `Depends on`: `T20`, `T10`, `T11`
- `Definition of Done`:
  - [UAT-C7-01](docs/uat/UAT-C7-01-graceful-cancel.md)

### T22. Compensation stack construction from completed nodes

- `Status`: `done`
- `Capability`: `C7`
- `Goal`: Record which completed nodes are compensable and in what unwind order.
- `Scope`:
  - compensable-node registration on completion
  - reverse-order unwind stack derivation
  - exclusion of non-completed nodes
- `Depends on`: `T4`, `T10`
- `Definition of Done`:
  - supports [UAT-C7-02](docs/uat/UAT-C7-02-failure-triggers-compensation.md)

### T23. Sequential compensation execution

- `Status`: `done`
- `Capability`: `C7`
- `Goal`: Execute compensation actions in reverse order after permanent failure.
- `Scope`:
  - trigger compensation after permanent failure
  - sequential compensation activity dispatch
  - compensation terminal outcomes
- `Depends on`: `T22`, `T9`, `T14`
- `Definition of Done`:
  - [UAT-C7-02](docs/uat/UAT-C7-02-failure-triggers-compensation.md)

### T24. Compensation recovery after restart

- `Status`: `done`
- `Capability`: `C7`
- `Goal`: Resume unfinished compensation from persisted progress after process restart.
- `Scope`:
  - persist compensation progress
  - resume remaining compensation actions after recovery
  - avoid repeating already completed compensation steps
- `Depends on`: `T23`, `T3`
- `Definition of Done`:
  - [UAT-C7-03](docs/uat/UAT-C7-03-compensation-resumes-after-restart.md)

### T25. Checkpoint request and awaiting-approval state

- `Status`: `done`
- `Capability`: `C8`
- `Goal`: Let a worker request approval and move only that node into waiting state.
- `Scope`:
  - checkpoint request result shape
  - `CheckpointReached`
  - `AWAITING_APPROVAL`
  - lease release on checkpoint wait
- `Depends on`: `T9`, `T10`, `T4`
- `Definition of Done`:
  - [UAT-C8-01](docs/uat/UAT-C8-01-checkpoint-pauses-one-node.md)

### T26. ApproveCheckpoint API and resume flow

- `Status`: `done`
- `Capability`: `C8`
- `Goal`: Resume a waiting node after explicit approval.
- `Scope`:
  - `ApproveCheckpoint` API
  - approval event
  - redispatch after approval
- `Depends on`: `T25`
- `Definition of Done`:
  - [UAT-C8-02](docs/uat/UAT-C8-02-approval-after-restart.md)

### T27. Checkpoint timeout handling

- `Status`: `done`
- `Capability`: `C8`
- `Goal`: Bound checkpoint waiting time if no approval arrives.
- `Scope`:
  - checkpoint timeout timer scheduling
  - timeout-to-failure semantics for waiting nodes
- `Depends on`: `T13`, `T25`
- `Definition of Done`:
  - [UAT-C8-03](docs/uat/UAT-C8-03-checkpoint-timeout.md)
  - supports [UAT-C5-04](docs/uat/UAT-C5-04-absolute-deadline-during-approval.md)

### T28. Checkpoint recovery across restart

- `Status`: `done`
- `Capability`: `C8`
- `Goal`: Preserve waiting approval state and allow post-restart approval.
- `Scope`:
  - rehydrate `AWAITING_APPROVAL`
  - preserve approval eligibility across restart
  - resume correctly after restart-time approval
- `Depends on`: `T25`, `T26`, `T3`
- `Definition of Done`:
  - [UAT-C8-02](docs/uat/UAT-C8-02-approval-after-restart.md)

---

## Wave 5: Product Surface and Proof

### T29. StartRun and workflow bootstrap contract

- `Status`: `todo`
- `Capability`: `C9`
- `Goal`: Start a run from a workflow definition and create initial persisted run state.
- `Scope`:
  - `StartRun` API
  - initial `WorkflowStarted` event
  - initial graph/bootstrap state creation
- `Depends on`: `T1`, `T4`, `T7`
- `Definition of Done`:
  - supports [UAT-C3-01](docs/uat/UAT-C3-01-linear-run-loop.md)
  - supports [UAT-C9-03](docs/uat/UAT-C9-03-start-run-capacity-reject.md)

### T30. GetRun current-state API

- `Status`: `todo`
- `Capability`: `C9`
- `Goal`: Expose the derived current view of a run.
- `Scope`:
  - run lookup by `RunID`
  - node state projection for current run
  - coherent active and terminal read behavior
- `Depends on`: `T4`, `T7`, `T29`
- `Definition of Done`:
  - [UAT-C9-01](docs/uat/UAT-C9-01-get-run-coherent-view.md)

### T31. ListEvents forensic history API

- `Status`: `todo`
- `Capability`: `C9`
- `Goal`: Expose raw event history in recorded order.
- `Scope`:
  - event listing by run
  - stable recorded order
  - no projection-only flattening of history
- `Depends on`: `T1`, `T29`
- `Definition of Done`:
  - [UAT-C9-02](docs/uat/UAT-C9-02-list-events-causal-history.md)

### T32. Hard-capacity rejection on StartRun

- `Status`: `todo`
- `Capability`: `C9`
- `Goal`: Enforce the v1 rule that capacity exhaustion rejects immediately instead of queuing.
- `Scope`:
  - capacity evaluation at run start
  - immediate API rejection
  - no hidden queued-run behavior
- `Depends on`: `T29`
- `Definition of Done`:
  - [UAT-C9-03](docs/uat/UAT-C9-03-start-run-capacity-reject.md)

### T33. Minimal workflow definition contract and definition hash capture

- `Status`: `todo`
- `Capability`: `C10`
- `Goal`: Freeze the minimum authoring contract for nodes, dependencies, and node policies.
- `Scope`:
  - workflow definition shape
  - node definition shape
  - definition hash captured at start
  - retry/deadline/checkpoint/compensation metadata fields
- `Depends on`: `T29`
- `Definition of Done`:
  - [UAT-C10-01](docs/uat/UAT-C10-01-definition-metadata-executes-as-declared.md)

### T34. Thin Go worker SDK seam

- `Status`: `todo`
- `Capability`: `C10`
- `Goal`: Provide a minimal Go worker integration layer over the public worker protocol.
- `Scope`:
  - activity handler registration
  - polling loop abstraction
  - success/failure completion helpers
- `Depends on`: `T9`, `T10`
- `Definition of Done`:
  - [UAT-C10-02](docs/uat/UAT-C10-02-go-sdk-worker-seam.md)

### T35. Composite demo workflow and end-to-end acceptance harness

- `Status`: `in_progress`
- `Capability`: `C11`
- `Goal`: Compose core v1 behaviors into one demonstrable end-to-end workflow.
- `Scope`:
  - discovery/spawn behavior
  - retry behavior
  - checkpoint behavior
  - restart continuation
  - final successful completion path
- `Depends on`: `T18`, `T14`, `T26`, `T28`, `T30`, `T31`
- `Definition of Done`:
  - [UAT-C11-01](docs/uat/UAT-C11-01-core-demo-e2e.md)
- `Progress Note`:
  - Sprint 3 stretch scaffolding is in place via a file-based `living-dag` example and CLI demo-worker path that exercises runtime spawn end-to-end.
  - Full `T35` remains open until retry, checkpoint, restart-continuation, and final composite acceptance are composed into one coherent demo flow.

---

## Suggested First Implementation Sequence

Recommended order for execution:

1. `T1` to `T7`
2. `T8` to `T16`
3. `T17` to `T19`
4. `T29` to `T32`
5. `T20` to `T28`
6. `T33` to `T35`

This order prioritizes a working durable execution core before deeper control-flow features and before the thin SDK/demo layer.

---

## Fastest Path To First Wow Demo

If the immediate goal is the first compelling demo rather than full v1 closure, focus on:

1. `T1` to `T18`
2. `T29` to `T31`
3. `T25` and `T26`
4. `T35`

This yields:

- durability
- live execution
- retries
- living DAG
- basic read APIs
- approval flow
- end-to-end story

without waiting for full cancellation, compensation, or SDK maturity.

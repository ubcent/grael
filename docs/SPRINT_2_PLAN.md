# Grael v1 Sprint 2 Plan

This document defines the second implementation sprint for Grael v1.

Sprint 2 turns the runtime skeleton from Sprint 1 into a real execution engine by adding worker dispatch, leases, timers, retries, and timeout handling.

---

## Sprint Goal

Build the first truly operational Grael runtime that can:

- dispatch work to workers over the public worker surface
- track active task ownership through leases
- detect lost worker liveness
- retry retryable failures automatically
- enforce execution deadlines
- survive restart while preserving retry and timeout behavior

At the end of Sprint 2, Grael should be able to run real single-node and simple multi-node workloads with durable recovery and basic reliability semantics.

---

## In Scope

The sprint includes the following tasks from `docs/V1_TASK_BACKLOG.md`:

1. `T8` Worker registry by activity type
2. `T9` Public worker RPC surface: `PollTask`, `CompleteTask`, `FailTask`, `Heartbeat`
3. `T10` Lease grant on dispatch and active-attempt tracking
4. `T11` Heartbeat timeout and lease expiry monitor
5. `T12` Reject stale worker results after lease expiry
6. `T13` Timer scheduling and firing engine
7. `T14` Retry policy and retry backoff scheduling
8. `T15` Execution deadline enforcement

---

## Why This Slice

Sprint 1 creates the runtime skeleton. Sprint 2 makes it executable.

It gives:

- real worker-based task execution
- recoverable ownership and liveness semantics
- retry automation
- time-based failure handling
- stronger recovery guarantees under runtime stress

without yet dragging in:

- dynamic graph mutation
- checkpoints
- cancellation
- compensation
- SDK concerns
- demo composition

This is the right next slice because once workers, leases, timers, and retries are real, Grael starts behaving like a durable workflow engine instead of a runtime shell.

---

## Out of Scope

Do not pull these into Sprint 2:

- runtime node spawn
- cycle detection
- absolute deadline during approval
- cancellation
- compensation
- checkpoints and approval APIs
- hard-capacity start rejection
- workflow definition richness beyond what Sprint 1 already established
- Go SDK
- end-to-end demo harness

If one of these becomes necessary to finish Sprint 2, the scope should be corrected explicitly rather than expanded implicitly.

---

## Expected End State

By the end of Sprint 2:

- a worker can poll and receive work
- a worker can complete or fail work through the public RPCs
- Grael tracks active attempts through leases
- heartbeat loss leads to lease expiry
- retryable failure schedules a retry automatically
- expired attempts cannot later complete successfully
- execution deadlines prevent stuck work from running forever
- retry and timeout behavior survives process restart

This is the point where Grael should be able to run simple real workloads, not just expose state and history.

---

## Exit Criteria

Sprint 2 is complete when all of the following are true:

- the in-scope tasks are implemented to a usable baseline
- worker execution happens through the public worker contract, not hidden stubs
- retry and timeout behavior is driven by persisted timers rather than in-memory shortcuts
- the following UATs can pass:
  - [UAT-C4-01-worker-success.md](docs/uat/UAT-C4-01-worker-success.md)
  - [UAT-C4-02-heartbeat-lease-expiry.md](docs/uat/UAT-C4-02-heartbeat-lease-expiry.md)
  - [UAT-C4-03-late-complete-rejected.md](docs/uat/UAT-C4-03-late-complete-rejected.md)
  - [UAT-C5-01-retry-backoff-success.md](docs/uat/UAT-C5-01-retry-backoff-success.md)
  - [UAT-C5-02-overdue-retry-after-restart.md](docs/uat/UAT-C5-02-overdue-retry-after-restart.md)
  - [UAT-C5-03-execution-deadline-timeout.md](docs/uat/UAT-C5-03-execution-deadline-timeout.md)

Current status:

- `T8` through `T15` are implemented to the current baseline
- worker execution now flows through the public worker contract rather than hidden internal completion shortcuts
- lease expiry, stale-result rejection, retry timers, restart catch-up, and execution-deadline timeout coverage are exercised in integration tests
- absolute-deadline runtime groundwork also exists, but the checkpoint-dependent acceptance scenario remains part of Sprint 4

---

## Stretch Goal

If Sprint 2 finishes early, the best stretch target is:

- `T16` Absolute deadline enforcement

Why:

- it stays inside the same timer/deadline slice
- it strengthens future checkpoint semantics immediately
- it avoids context-switching into cancellation or living DAG work

Do not treat the stretch goal as part of the committed sprint scope.

---

## Risks

The main Sprint 2 risks are:

- leaking attempt/lease semantics across too many layers instead of keeping them crisp
- allowing stale worker results to mutate state after expiry
- implementing timers as convenience state instead of durable event-driven infrastructure
- tying retry behavior to process continuity rather than persisted timer state
- conflating heartbeat liveness with task success or task ownership semantics

---

## Review Questions

At the midpoint and end of the sprint, the team should ask:

1. Can a real worker execute tasks end to end without hidden shortcuts?
2. If a worker disappears, does Grael visibly recover instead of hanging forever?
3. If Grael restarts after a retry was scheduled, does the retry still happen?
4. Would dynamic graph work feel like an additive feature on top of a stable execution core?

If the answer to any of these is "no", the sprint likely needs scope correction before proceeding.

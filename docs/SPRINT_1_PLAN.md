# Grael v1 Sprint 1 Plan

This document defines the first implementation sprint for Grael v1.

Sprint 1 is intentionally narrow. Its purpose is to produce the first vertically useful execution skeleton rather than to maximize feature count.

---

## Sprint Goal

Build the first end-to-end Grael runtime skeleton that can:

- accept `StartRun`
- persist run history to the WAL
- derive current state through event application
- drive execution through the run loop and scheduler boundary
- expose current state through `GetRun`
- expose raw history through `ListEvents`

At the end of Sprint 1, Grael should feel like a real system, even if workers, retries, dynamic graph growth, and checkpoints are not implemented yet.

---

## In Scope

The sprint includes the following tasks from `docs/V1_TASK_BACKLOG.md`:

1. `T1` WAL append format and CRC validation
2. `T3` Snapshot write and snapshot-plus-delta restore
3. `T4` ExecutionState skeleton and event application core
4. `T6` Pure scheduler and deterministic command ordering
5. `T7` Run loop and command processor baseline
6. `T29` StartRun and workflow bootstrap contract
7. `T30` GetRun current-state API
8. `T31` ListEvents forensic history API

---

## Why This Slice

This sprint is the smallest slice that creates a usable product skeleton.

It gives:

- durable persisted history
- snapshot-backed recovery
- derived runtime state
- an execution loop
- public visibility into state and events

without dragging in:

- worker protocol complexity
- lease semantics
- timer semantics
- dynamic graph mutation
- cancellation and compensation
- checkpoint handling

This is the right first cut because it creates a stable base that later capabilities can attach to.

---

## Out of Scope

Do not pull these into Sprint 1:

- worker RPCs
- worker registry
- lease grant and expiry
- retries and timers
- deadlines
- living DAG spawn
- cancellation
- compensation
- checkpoints
- capacity rejection
- workflow definition richness beyond the minimum bootstrap contract
- Go SDK
- demo harness

If one of these becomes necessary to "finish" Sprint 1, the scope is wrong and should be reconsidered rather than silently expanded.

---

## Expected End State

By the end of Sprint 1:

- a client can call `StartRun`
- the run is written to WAL
- snapshots can persist current derived execution state
- run state can be rebuilt from snapshot plus WAL delta
- `ExecutionState` can be derived from persisted events
- `Scheduler.Decide(state)` can be invoked from the run loop
- `GetRun` returns a coherent current-state view
- `ListEvents` returns the raw recorded history in order

It is acceptable if execution is still partially stubbed internally, as long as the runtime skeleton is real and externally inspectable.

---

## Exit Criteria

Sprint 1 is complete when all of the following are true:

- the in-scope tasks are implemented to a usable baseline
- no out-of-scope capability was partially dragged in as hidden coupling
- the following UATs can pass, even if initially through a thin or stubbed execution path:
  - [UAT-C1-01-restart-recovery.md](docs/uat/UAT-C1-01-restart-recovery.md)
  - [UAT-C3-01-linear-run-loop.md](docs/uat/UAT-C3-01-linear-run-loop.md)
  - [UAT-C9-01-get-run-coherent-view.md](docs/uat/UAT-C9-01-get-run-coherent-view.md)
  - [UAT-C9-02-list-events-causal-history.md](docs/uat/UAT-C9-02-list-events-causal-history.md)

---

## Risks

The main Sprint 1 risks are:

- overbuilding the workflow definition contract too early
- letting scheduler purity erode by leaking I/O into the decision layer
- treating `GetRun` as a second source of truth instead of a derived view
- accidentally coupling the run loop to future worker/timer semantics before the runtime skeleton is stable

---

## Review Questions

At the midpoint and end of the sprint, the team should ask:

1. Is the runtime skeleton getting more real, or are we just producing abstractions?
2. Can we inspect a started run meaningfully through `GetRun` and `ListEvents`?
3. Would adding worker dispatch next feel like attaching capability to a stable base?

If the answer to any of these is "no", the sprint likely needs scope correction before proceeding.

---

## Manual Verification

You can verify snapshot-backed recovery manually with the CLI:

```bash
make build
RUN_ID=$(./bin/grael start -example linear-noop)
./bin/grael snapshot -run-id "$RUN_ID"
./bin/grael status -run-id "$RUN_ID"
./bin/grael events -run-id "$RUN_ID"
```

What to check:

- `snapshot` reports `exists: true`
- `snapshot.seq` is non-zero
- `status` returns the expected derived run state
- `events` returns the full raw event history

To verify recovery rather than in-memory state, rerun `status`, `events`, and `snapshot` in a fresh process against the same `-data-dir`.

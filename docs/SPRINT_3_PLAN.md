# Grael v1 Sprint 3 Plan

This document defines the third implementation sprint for Grael v1.

Sprint 3 adds the living DAG behavior that makes Grael meaningfully different: nodes can complete, discover new work, and grow the graph at runtime in a durable and recoverable way.

---

## Sprint Goal

Build the first real dynamic-graph version of Grael that can:

- accept spawned nodes from task completion
- add those nodes into the active run graph
- schedule and execute spawned work in the same run
- recover the expanded graph after restart
- reject invalid spawn shapes that would create cycles

At the end of Sprint 3, Grael should be able to demonstrate its core differentiator: a workflow whose structure grows during execution and still behaves like a durable engine.

---

## In Scope

The sprint includes the following tasks from `docs/V1_TASK_BACKLOG.md`:

1. `T17` Runtime spawn payload handling
2. `T18` Dynamic graph scheduling and persisted rehydration
3. `T19` Spawn validation and cycle rejection

---

## Why This Slice

Sprint 1 creates the runtime skeleton. Sprint 2 makes it execute real work. Sprint 3 makes the product recognizably Grael.

It gives:

- runtime graph growth
- durable dynamic structure
- same-run fan-out via node spawn
- graph safety against cyclic corruption

without yet dragging in:

- checkpoints
- cancellation
- compensation
- SDK concerns
- full end-to-end demo composition

This is the right third slice because once dynamic graph mutation is real, Grael can show the thing that most alternatives do not handle cleanly in a single-binary durable engine.

---

## Out of Scope

Do not pull these into Sprint 3:

- checkpoints and approval APIs
- cancellation
- compensation
- absolute deadline during approval
- Go SDK
- final demo composition

Also avoid overbuilding:

- fan-out policy systems
- graph policy engines
- generalized map-reduce abstractions
- sub-workflows

Sprint 3 should solve runtime spawn cleanly, not design a broader orchestration framework.

---

## Expected End State

By the end of Sprint 3:

- a worker can complete a node and return spawned node definitions
- newly spawned nodes appear in `GetRun`
- spawned nodes become runnable and complete in the same run
- restart preserves the expanded graph shape
- invalid cycle-producing spawn attempts are rejected safely

This is the point where Grael should support the first compelling living-graph demos.

---

## Exit Criteria

Sprint 3 is complete when all of the following are true:

- the in-scope tasks are implemented to a usable baseline
- dynamic graph growth is persisted, not held only in memory
- invalid spawn attempts fail safely before corrupting active graph state
- the following UATs can pass:
  - [UAT-C6-01-living-dag-spawn.md](docs/uat/UAT-C6-01-living-dag-spawn.md)
  - [UAT-C6-02-spawned-graph-restart-durability.md](docs/uat/UAT-C6-02-spawned-graph-restart-durability.md)
  - [UAT-C6-03-cycle-spawn-rejected.md](docs/uat/UAT-C6-03-cycle-spawn-rejected.md)

---

## Stretch Goal

If Sprint 3 finishes early, the best stretch target is:

- begin scaffolding `T35` demo workflow composition around the now-working spawn path

Why:

- it immediately converts the new capability into a visible product story
- it does not force context-switching into cancellation or checkpoints yet

Do not treat the stretch goal as part of the committed sprint scope.

---

## Risks

The main Sprint 3 risks are:

- recording invalid graph mutations before validation is complete
- treating spawned nodes as a view concern instead of persisted execution state
- rebuilding the graph incorrectly after restart
- coupling dynamic spawn too tightly to specific demo logic rather than generic node-completion semantics

---

## Review Questions

At the midpoint and end of the sprint, the team should ask:

1. Does the graph really grow during execution, or are we just simulating fan-out externally?
2. If Grael restarts after spawn, does the expanded graph come back exactly as expected?
3. Can an invalid cycle-producing spawn ever enter active graph state?
4. Would a user watching `GetRun` immediately understand that the graph is changing live?

If the answer to any of these is "no", the sprint likely needs scope correction before proceeding.

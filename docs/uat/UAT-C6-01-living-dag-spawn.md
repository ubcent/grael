# UAT-C6-01 Living DAG Spawn

## Metadata

- `UAT ID`: `UAT-C6-01`
- `Title`: A completed node spawns new nodes and the graph grows during execution
- `Capability`: `C6`
- `Priority`: `core`
- `Related task(s)`: `C6` dynamic graph mutation tasks
- `Depends on`: `C1`, `C2`, `C3`, `C4`, `C6`, `C9`

---

## Intent

This UAT proves the central Grael v1 differentiator: the workflow graph can expand at runtime based on worker-discovered work.

If this UAT passes, an operator can trust that:

- a worker can complete one node and declare additional work
- new nodes become visible in the run after execution has started
- the spawned work can be scheduled and completed as part of the same run

---

## Setup

- engine state: Grael server is running
- worker state: one worker is running for activity types `discover` and `analyze`
- workflow shape: initial workflow contains one `discover` node
- test input: `discover` returns three spawned `analyze` nodes
- clock/timer assumptions: no retry, timeout, or checkpoint behavior required

---

## Action

1. Start Grael.
2. Start a worker registered for activity types `discover` and `analyze`.
3. Submit a workflow with a single `discover` node.
4. Let the worker complete the `discover` node and return three spawned `analyze` nodes.
5. Query `GetRun` immediately after the `discover` node completes.
6. Allow the worker to complete the spawned `analyze` nodes.
7. Query `GetRun` after all spawned nodes complete.
8. Query `ListEvents`.

---

## Expected Visible Outcome

- the run begins with one visible node
- after `discover` completes, `GetRun` shows three new nodes that were not part of the original visible graph
- the spawned nodes become runnable and complete successfully
- the workflow reaches successful terminal completion after spawned work finishes

Checklist:

- expected API state: graph size increases during execution
- expected terminal outcome: workflow completes successfully
- expected event-family visibility: original completion plus spawned-node execution
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted` for `discover`
3. `NodeStarted` for `discover`
4. `NodeCompleted` for `discover` with spawned node definitions
5. New spawned nodes become visible in `GetRun`
6. `LeaseGranted` and `NodeStarted` for spawned nodes
7. `NodeCompleted` for each spawned node
8. `WorkflowCompleted`

---

## Failure Evidence

- spawned nodes never appear in `GetRun`
- spawned nodes appear but are never scheduled
- the workflow completes before spawned nodes finish
- spawned nodes execute as a separate run instead of within the original run

---

## Pass Criteria

- graph growth is externally visible after the parent node completes
- spawned nodes execute within the same run and reach completion
- the workflow completes only after the dynamically added work is done

---

## Notes

- This scenario should be part of every public Grael demo once implemented.
- A later companion UAT should cover restart durability for spawned graphs.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is suitable for automation with a deterministic worker fixture that returns a fixed spawn set.

# UAT-C6-02 Spawned Graph Restart Durability

## Metadata

- `UAT ID`: `UAT-C6-02`
- `Title`: Spawned graph survives restart and rehydrates identically
- `Capability`: `C6`
- `Priority`: `core`
- `Related task(s)`: `C6` graph mutation durability tasks
- `Depends on`: `C1`, `C3`, `C4`, `C6`, `C9`

---

## Intent

This UAT proves that dynamically spawned graph structure is durable and reconstructable after restart.

If this UAT passes, an operator can trust that:

- runtime graph growth is not just an in-memory effect
- spawned nodes remain part of the run after process restart
- execution can continue from the dynamically expanded graph

---

## Setup

- engine state: Grael server is running with a clean data directory
- worker state: one worker is running for activity types `discover` and `analyze`
- workflow shape: initial workflow contains one `discover` node
- test input: `discover` spawns three `analyze` nodes
- clock/timer assumptions: no retry or deadline behavior required

Additional setup constraint:

- the test harness must be able to restart Grael after the spawn has been committed but before all spawned nodes complete

---

## Action

1. Start Grael.
2. Start one worker registered for `discover` and `analyze`.
3. Submit a workflow containing one `discover` node.
4. Let `discover` complete and return three spawned `analyze` nodes.
5. Confirm through `GetRun` that the new nodes are visible.
6. Stop Grael before all spawned nodes finish.
7. Restart Grael using the same data directory.
8. Query `GetRun` after restart.
9. Allow the spawned nodes to complete.
10. Query `GetRun` and `ListEvents` after terminal completion.

---

## Expected Visible Outcome

- the spawned nodes remain visible after restart
- Grael does not revert to the original one-node graph
- the restarted engine can continue scheduling and completing the spawned nodes
- the workflow reaches successful terminal completion after the spawned work finishes

Checklist:

- expected API state: expanded graph remains present across restart
- expected terminal outcome: workflow completes successfully
- expected event-family visibility: discover completion with spawn, restart, spawned-node execution
- expected behavior after restart: graph shape matches previously committed dynamic structure

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `NodeCompleted` for `discover` with spawned node definitions
3. Spawned nodes become visible in `GetRun`
4. Grael process stops
5. Grael process restarts
6. Spawned nodes are still visible in `GetRun`
7. Spawned-node execution events continue
8. `WorkflowCompleted`

---

## Failure Evidence

- spawned nodes disappear after restart
- Grael rebuilds only the original static graph
- spawned nodes must be rediscovered manually after restart
- workflow completes without accounting for already spawned pending work

---

## Pass Criteria

- dynamically added nodes survive restart as part of the same run state
- execution continues from the expanded graph after recovery
- workflow completes only after the persisted spawned work is handled

---

## Notes

- This is the durability companion to the basic living DAG happy-path UAT.
- It should remain focused on graph persistence, not on retries or checkpoints.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is automatable if the harness can restart the engine at a controlled point after spawn commitment.

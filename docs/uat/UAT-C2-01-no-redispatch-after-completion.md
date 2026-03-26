# UAT-C2-01 No Redispatch After Completion

## Metadata

- `UAT ID`: `UAT-C2-01`
- `Title`: Completed nodes do not re-dispatch after restart or replay
- `Capability`: `C2`
- `Priority`: `core`
- `Related task(s)`: `C2` state machine and terminal-state correctness tasks
- `Depends on`: `C1`, `C2`, `C3`, `C4`, `C9`

---

## Intent

This UAT proves that once a node has reached completion and that fact is persisted, Grael does not dispatch it again during recovery or continued execution.

If this UAT passes, an operator can trust that:

- terminal completion is durable
- replay and restart do not cause duplicate task execution
- orchestration respects persisted terminal node state

---

## Setup

- engine state: Grael server is running with a clean data directory
- worker state: one worker is running for activity type `step`
- workflow shape: workflow with at least one completed node and at least one remaining node
- test input: node `A` completes successfully; additional work remains in the run
- clock/timer assumptions: no retry behavior required

Additional setup constraint:

- the test harness must be able to restart Grael after node `A` completion has been committed

---

## Action

1. Start Grael.
2. Start one worker for activity type `step`.
3. Submit a workflow where node `A` completes before later nodes run.
4. Let node `A` complete successfully.
5. Stop Grael after `NodeCompleted` for node `A` has been persisted.
6. Restart Grael using the same data directory.
7. Continue the run to completion.
8. Inspect worker deliveries, `GetRun`, and `ListEvents`.

---

## Expected Visible Outcome

- node `A` is visible as completed after restart
- node `A` is not dispatched again after restart
- later nodes can continue normally
- the workflow completes without duplicate execution of node `A`

Checklist:

- expected API state: node `A` remains terminally completed after restart
- expected terminal outcome: workflow completes successfully
- expected event-family visibility: one completion record for node `A`
- expected behavior after restart: recovery continues from persisted completion, not from pre-completion state

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted` for node `A`
3. `NodeStarted` for node `A`
4. `NodeCompleted` for node `A`
5. Grael restarts
6. No second dispatch/start sequence appears for node `A`
7. Later node events occur
8. `WorkflowCompleted`

---

## Failure Evidence

- node `A` is delivered to a worker a second time after restart
- multiple accepted completion paths exist for the same happy-path node instance
- recovery behaves as if node `A` never completed

---

## Pass Criteria

- a persisted completed node is never re-dispatched after restart
- event history contains one accepted completion path for that node
- remaining workflow execution proceeds without duplicate work

---

## Notes

- This scenario is one of the key practical checks for "rehydration is state recovery, not re-execution."

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is deterministic and should automate well with a restart-capable harness.

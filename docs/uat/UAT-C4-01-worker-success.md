# UAT-C4-01 Worker Success Path

## Metadata

- `UAT ID`: `UAT-C4-01`
- `Title`: Worker polls, receives a task, completes it, and the run finishes successfully
- `Capability`: `C4`
- `Priority`: `core`
- `Related task(s)`: `C4` worker dispatch and lease tasks
- `Depends on`: `C1`, `C3`, `C4`, `C9`

---

## Intent

This UAT proves that Grael has a functioning worker execution model over the public worker surface.

If this UAT passes, an operator can trust that:

- workers can obtain work from Grael
- successful task completion is accepted and persisted
- node completion drives run completion

---

## Setup

- engine state: Grael server is running and reachable
- worker state: one worker process registered for activity type `hello`
- workflow shape: single-node workflow with one `hello` activity
- test input: input that causes the worker to return success immediately
- clock/timer assumptions: no retry or timeout behavior required

---

## Action

1. Start Grael.
2. Start a worker registered for activity type `hello`.
3. Submit a workflow containing one `hello` node.
4. Wait for the worker to receive the task through `PollTask`.
5. Complete the task with `CompleteTask`.
6. Query `GetRun`.
7. Query `ListEvents`.

---

## Expected Visible Outcome

- the worker receives exactly one task for the run
- `CompleteTask` is accepted successfully
- `GetRun` shows the node as completed
- the workflow reaches successful terminal completion

Checklist:

- expected API state: run visible and terminal after completion
- expected terminal outcome: workflow completes successfully
- expected event-family visibility: task dispatch, node start, node completion, workflow completion
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted`
3. `NodeStarted`
4. `NodeCompleted`
5. `WorkflowCompleted`

---

## Failure Evidence

- worker never receives a task
- more than one task is delivered for the same single-node happy-path run
- `CompleteTask` succeeds but `GetRun` does not show node completion
- workflow remains non-terminal after successful task completion

---

## Pass Criteria

- worker polling results in one valid task delivery
- successful completion is persisted and visible through read APIs
- the workflow finishes successfully with the expected event sequence

---

## Notes

- This scenario is the baseline acceptance test for the worker contract.
- It should stay intentionally simple and avoid retry, cancellation, or checkpoint behavior.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is compact and should be easy to automate with a deterministic test worker fixture.

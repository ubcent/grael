# UAT-C3-01 Linear Run Loop

## Metadata

- `UAT ID`: `UAT-C3-01`
- `Title`: A simple linear workflow progresses from start to completion through the run loop
- `Capability`: `C3`
- `Priority`: `core`
- `Related task(s)`: `C2` and `C3` runtime tasks
- `Depends on`: `C1`, `C2`, `C3`, `C4`, `C9`

---

## Intent

This UAT proves that Grael can drive a simple workflow from start to finish using its normal event-driven run loop.

If this UAT passes, an operator can trust that:

- starting a run actually causes execution to progress
- dependency progression works for a basic linear graph
- the engine can reach terminal completion without manual intervention

---

## Setup

- engine state: Grael server is running with an empty or reusable healthy data directory
- worker state: one worker is running for activity type `step`
- workflow shape: linear workflow `A -> B -> C`
- test input: success path input for all nodes
- clock/timer assumptions: no retry, timeout, or checkpoint behavior required

---

## Action

1. Start Grael.
2. Start one worker registered for activity type `step`.
3. Submit the workflow `A -> B -> C`.
4. Let the worker complete each task successfully as it is delivered.
5. Query `GetRun` during execution.
6. Query `GetRun` after the run finishes.
7. Query `ListEvents`.

---

## Expected Visible Outcome

- `GetRun` shows the run progressing through non-terminal states toward completion
- node `B` does not begin before node `A` completes
- node `C` does not begin before node `B` completes
- the run reaches successful terminal completion
- `ListEvents` reflects a valid linear progression

Checklist:

- expected API state: current run state changes over time
- expected terminal outcome: workflow completes successfully
- expected event-family visibility: workflow start, dispatch, start, completion, workflow completion
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted` for node `A`
3. `NodeStarted` for node `A`
4. `NodeCompleted` for node `A`
5. `LeaseGranted` for node `B`
6. `NodeStarted` for node `B`
7. `NodeCompleted` for node `B`
8. `LeaseGranted` for node `C`
9. `NodeStarted` for node `C`
10. `NodeCompleted` for node `C`
11. `WorkflowCompleted`

---

## Failure Evidence

- node `B` starts before node `A` completes
- node `C` starts before node `B` completes
- the run stalls indefinitely after `WorkflowStarted`
- the workflow reaches terminal failure on the happy path

---

## Pass Criteria

- workflow progresses from `WorkflowStarted` to `WorkflowCompleted`
- node order respects the declared dependency chain
- all externally visible state transitions are coherent with a linear run

---

## Notes

- This is the smallest end-to-end proof that the run loop is operational.
- It should be one of the first UATs automated in the project.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is deterministic and should be easy to automate once a basic local test harness exists.

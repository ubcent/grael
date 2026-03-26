# UAT-C1-01 Restart Recovery

## Metadata

- `UAT ID`: `UAT-C1-01`
- `Title`: Restart from WAL and snapshot resumes a run without losing committed progress
- `Capability`: `C1`
- `Priority`: `core`
- `Related task(s)`: `C1` storage and recovery tasks
- `Depends on`: `C1`, `C3`, `C9`

---

## Intent

This UAT proves that Grael can recover an in-progress run after process termination using only persisted state.

If this UAT passes, an operator can trust that:

- committed workflow progress is not lost on restart
- already completed nodes do not need to be re-executed
- the engine can continue a partially executed run from persisted history

---

## Setup

- engine state: Grael server starts with a clean data directory and snapshots enabled
- worker state: one worker is running for activity type `step`
- workflow shape: linear workflow `A -> B -> C`
- test input: normal input that causes all three nodes to succeed
- clock/timer assumptions: no retry or deadline behavior required for this scenario

Additional setup constraint:

- the test harness must be able to terminate and restart Grael against the same data directory

---

## Action

1. Start Grael with an empty data directory.
2. Start one worker registered for activity type `step`.
3. Submit the linear workflow `A -> B -> C`.
4. Allow node `A` to complete successfully.
5. Allow node `B` to start, but terminate the Grael process before the run reaches terminal completion.
6. Restart Grael using the same data directory.
7. Wait for the run to resume and reach terminal completion.
8. Query `GetRun`.
9. Query `ListEvents`.

---

## Expected Visible Outcome

- `GetRun` after restart shows node `A` as already completed
- the run resumes from persisted progress instead of restarting from scratch
- the workflow eventually reaches successful terminal completion
- `ListEvents` shows a continuous event history spanning pre-restart and post-restart execution

Checklist:

- expected API state: run visible before and after restart under the same `RunID`
- expected terminal outcome: workflow completes successfully
- expected event-family visibility: workflow start, node progress, restart-resumed completion
- expected behavior after restart: Grael continues from prior committed history

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted` for node `A`
3. `NodeStarted` for node `A`
4. `NodeCompleted` for node `A`
5. `LeaseGranted` for node `B`
6. `NodeStarted` for node `B`
7. Grael process terminates
8. Grael process restarts
9. Remaining valid node progress events appear
10. `WorkflowCompleted`

---

## Failure Evidence

- node `A` is dispatched again after restart
- the run is missing from `GetRun` after restart
- valid events written before restart disappear from `ListEvents`
- the run restarts from the beginning as if no progress had been committed

---

## Pass Criteria

- all expected visible outcomes are observed
- no previously completed node is re-executed
- the run reaches terminal success using the same persisted run history
- event history remains continuous and causally valid across restart

---

## Notes

- This UAT should use a real process restart, not a mocked restart path.
- The exact restart point can vary as long as at least one node is already committed before the crash.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is a strong candidate for automated integration coverage because restart behavior is externally observable and deterministic enough for harness-driven testing.

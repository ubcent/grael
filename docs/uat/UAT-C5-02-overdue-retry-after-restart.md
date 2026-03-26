# UAT-C5-02 Overdue Retry After Restart

## Metadata

- `UAT ID`: `UAT-C5-02`
- `Title`: Overdue retry timer fires after restart and the node continues
- `Capability`: `C5`
- `Priority`: `core`
- `Related task(s)`: `C5` timer durability and restart catch-up tasks
- `Depends on`: `C1`, `C4`, `C5`, `C9`

---

## Intent

This UAT proves that persisted retry timers survive restart and overdue timers are caught up when Grael comes back.

If this UAT passes, an operator can trust that:

- retries are not lost when the process is down
- timer behavior is driven by persisted state, not in-memory scheduler continuity
- a temporarily stopped Grael instance can resume retry logic correctly

---

## Setup

- engine state: Grael server is running with a clean data directory
- worker state: one worker is running for activity type `flaky`
- workflow shape: single-node workflow with retry policy
- test input: first attempt fails retryably, second attempt should succeed
- clock/timer assumptions: retry timer delay is short and measurable

Additional setup constraint:

- the test harness must be able to stop Grael after retry scheduling and restart it after the retry deadline has already passed

---

## Action

1. Start Grael.
2. Start one worker registered for activity type `flaky`.
3. Submit a single-node workflow with retry enabled.
4. On the first attempt, have the worker return a retryable failure.
5. Confirm that a retry timer has been scheduled.
6. Stop Grael before the timer fires.
7. Wait until the scheduled retry time is in the past.
8. Restart Grael using the same data directory.
9. Allow the retried attempt to run and complete successfully.
10. Query `GetRun`.
11. Query `ListEvents`.

---

## Expected Visible Outcome

- the retry does not disappear because the process was stopped
- after restart, the overdue retry fires and dispatch resumes
- the second attempt can complete successfully
- the workflow reaches successful terminal completion

Checklist:

- expected API state: run remains recoverable across restart
- expected terminal outcome: workflow completes successfully
- expected event-family visibility: first failure, retry scheduling, restart, timer catch-up, second dispatch, completion
- expected behavior after restart: overdue timer is caught up and acted on

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted`
3. `NodeStarted`
4. `NodeFailed` with retryable failure
5. `TimerScheduled` for retry backoff
6. Grael process stops before timer firing
7. Grael process restarts after scheduled time has passed
8. `TimerFired` for overdue retry
9. Retry dispatch events occur
10. `NodeCompleted`
11. `WorkflowCompleted`

---

## Failure Evidence

- no retry occurs after restart
- the run remains stuck waiting for a timer that should already be overdue
- the retry timer is effectively forgotten across process restart

---

## Pass Criteria

- persisted retry schedule survives restart
- overdue timer is turned into visible retry progress after restart
- successful retried completion leads to workflow success

---

## Notes

- This scenario directly exercises timer durability, not just retry policy semantics.
- The restart must happen after timer scheduling and before timer firing.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is automatable with a harness that controls process lifecycle and wall-clock timing windows.

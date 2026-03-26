# UAT-C5-01 Retry Backoff Success

## Metadata

- `UAT ID`: `UAT-C5-01`
- `Title`: Retryable failure leads to backoff and successful re-execution
- `Capability`: `C5`
- `Priority`: `core`
- `Related task(s)`: `C5` timer and retry tasks
- `Depends on`: `C1`, `C3`, `C4`, `C5`, `C9`

---

## Intent

This UAT proves that Grael can recover automatically from a retryable task failure without manual intervention.

If this UAT passes, an operator can trust that:

- retryable failures do not permanently fail the run immediately
- backoff behavior is persisted and observable
- a later successful attempt can complete the node and the run

---

## Setup

- engine state: Grael server is running
- worker state: one worker is running for activity type `flaky`
- workflow shape: single-node workflow with retry policy allowing at least one retry
- test input: worker fixture fails once with a retryable error, then succeeds
- clock/timer assumptions: retry backoff interval is short but non-zero

---

## Action

1. Start Grael.
2. Start a worker registered for activity type `flaky`.
3. Submit a workflow with one `flaky` node and a retry policy.
4. On first delivery, have the worker report failure through `FailTask` using a retryable error.
5. Wait for the retry backoff period to elapse.
6. On second delivery, have the worker complete successfully through `CompleteTask`.
7. Query `GetRun`.
8. Query `ListEvents`.

---

## Expected Visible Outcome

- the first attempt does not terminally fail the workflow
- a retry occurs without manual operator action
- the second successful attempt completes the node
- the workflow reaches successful terminal completion

Checklist:

- expected API state: run remains active after first failure and later reaches success
- expected terminal outcome: workflow completes successfully
- expected event-family visibility: failure, retry timer scheduling/firing, second dispatch, completion
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted`
3. `NodeStarted`
4. `NodeFailed` with retryable failure
5. `TimerScheduled` for retry backoff
6. `TimerFired` for retry backoff
7. `LeaseGranted` for retry attempt
8. `NodeStarted` for retry attempt
9. `NodeCompleted`
10. `WorkflowCompleted`

---

## Failure Evidence

- the workflow fails permanently after the first retryable failure
- no second attempt is dispatched
- retry occurs only after manual intervention
- a successful second attempt does not move the run to completion

---

## Pass Criteria

- first failure is recorded as retryable, not terminal to the workflow
- retry backoff is externally visible through event history
- second attempt completes successfully and leads to workflow completion

---

## Notes

- This UAT should eventually be paired with a restart variant to prove timer durability under crash conditions.
- The worker fixture should be deterministic: fail exactly once, then succeed.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is highly automatable because the worker behavior can be scripted exactly.

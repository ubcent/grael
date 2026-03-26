# UAT-C4-03 Late Complete Rejected

## Metadata

- `UAT ID`: `UAT-C4-03`
- `Title`: Late `CompleteTask` after lease expiry is rejected
- `Capability`: `C4`
- `Priority`: `core`
- `Related task(s)`: `C4` lease validity and stale result rejection tasks
- `Depends on`: `C1`, `C4`, `C5`, `C9`

---

## Intent

This UAT proves that once a lease is expired, the original worker can no longer complete the old attempt successfully.

If this UAT passes, an operator can trust that:

- expired leases are permanently dead
- stale worker results cannot overwrite newer orchestration decisions
- late completion races do not corrupt run state

---

## Setup

- engine state: Grael server is running
- worker state: one worker is running for activity type `slow_step`
- workflow shape: single-node workflow
- test input: worker begins work but intentionally delays completion until after lease expiry
- clock/timer assumptions: lease timeout is short enough to test reliably

---

## Action

1. Start Grael.
2. Start one worker registered for activity type `slow_step`.
3. Submit a single-node workflow using `slow_step`.
4. Wait for the worker to receive the task and begin execution.
5. Allow the lease to expire before the worker sends `CompleteTask`.
6. After expiry, have the original worker send `CompleteTask` for the expired attempt.
7. Query `GetRun`.
8. Query `ListEvents`.

---

## Expected Visible Outcome

- the late `CompleteTask` call is rejected
- the node does not transition to `COMPLETED` because of the expired attempt
- `ListEvents` shows lease expiry before any later completion attempt is accepted
- run state follows retry or failure handling, not stale success

Checklist:

- expected API state: no stale completion is reflected in current state
- expected terminal outcome: depends on retry policy, but stale completion must not win
- expected event-family visibility: dispatch, start, lease expiry, rejection path
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted`
3. `NodeStarted`
4. `LeaseExpired`
5. Original worker sends late `CompleteTask`
6. No `NodeCompleted` is accepted for that expired attempt

---

## Failure Evidence

- late `CompleteTask` is accepted after lease expiry
- node becomes `COMPLETED` from the stale attempt
- stale completion suppresses retry or failure handling

---

## Pass Criteria

- expired attempt cannot complete successfully
- stale worker result is rejected externally and has no effect on run state
- visible event history preserves lease finality

---

## Notes

- This UAT is one of the most important correctness checks in the worker protocol.
- It should later gain a restart variant to cover the same behavior across process recovery.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is automatable if the harness can control completion timing relative to lease expiry.

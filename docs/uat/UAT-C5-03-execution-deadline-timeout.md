# UAT-C5-03 Execution Deadline Timeout

## Metadata

- `UAT ID`: `UAT-C5-03`
- `Title`: Execution deadline failure marks the node failed with timeout semantics
- `Capability`: `C5`
- `Priority`: `core`
- `Related task(s)`: `C5` deadline enforcement tasks
- `Depends on`: `C1`, `C4`, `C5`, `C9`

---

## Intent

This UAT proves that a stuck running node cannot block the workflow forever and is converted into timeout failure when its execution deadline is exceeded.

If this UAT passes, an operator can trust that:

- execution deadlines are enforced
- long-running stuck work cannot silently stall a run forever
- timeout handling is visible through the normal read surfaces

---

## Setup

- engine state: Grael server is running
- worker state: one worker is running for activity type `hanging_step`
- workflow shape: single-node workflow with an execution deadline
- test input: worker accepts the task and does not complete within deadline
- clock/timer assumptions: execution deadline is short enough for test runtime

---

## Action

1. Start Grael.
2. Start one worker registered for activity type `hanging_step`.
3. Submit a single-node workflow with an execution deadline.
4. Wait for the worker to receive the task and begin execution.
5. Do not complete or fail the task before the deadline expires.
6. Wait for deadline handling to occur.
7. Query `GetRun`.
8. Query `ListEvents`.

---

## Expected Visible Outcome

- the node does not remain running forever
- after deadline expiry, the node is shown as failed with timeout semantics
- the workflow proceeds to retry or terminal failure according to policy
- `ListEvents` makes the timeout path visible

Checklist:

- expected API state: node no longer shown as indefinitely running
- expected terminal outcome: depends on retry policy, but timeout semantics must be visible
- expected event-family visibility: start, deadline-related timer behavior, timeout failure outcome
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted`
3. `NodeStarted`
4. `TimerScheduled` for execution deadline
5. `TimerFired` for execution deadline
6. Failure event representing timeout semantics

---

## Failure Evidence

- node remains running forever after deadline expiry
- no timeout-driven failure becomes visible
- workflow remains stalled despite expired deadline

---

## Pass Criteria

- execution deadline expiry produces visible timeout failure semantics
- the node is no longer treated as active indefinitely
- subsequent handling follows retry or terminal failure policy rather than silent hang

---

## Notes

- In v1 planning, timeout semantics map into node failure rather than a separate node state.
- This UAT should assert visible behavior, not a specific internal event type if the implementation chooses one representation.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is deterministic if the deadline window is kept short and the worker fixture reliably hangs.

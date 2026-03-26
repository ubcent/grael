# UAT-C4-02 Heartbeat Lease Expiry

## Metadata

- `UAT ID`: `UAT-C4-02`
- `Title`: Heartbeat loss expires leases and prevents a task from hanging forever
- `Capability`: `C4`
- `Priority`: `core`
- `Related task(s)`: `C4` heartbeat and lease monitor tasks
- `Depends on`: `C1`, `C4`, `C5`, `C9`

---

## Intent

This UAT proves that loss of worker liveness is detected and converted into recoverable lease expiry rather than indefinite task hang.

If this UAT passes, an operator can trust that:

- a disappeared worker does not hold a task forever
- heartbeat timeout causes lease expiry
- the run can continue or retry after worker loss

---

## Setup

- engine state: Grael server is running
- worker state: one worker is running for activity type `slow_step` and sends heartbeats while alive
- workflow shape: single-node workflow or short workflow where one node can remain running long enough to observe heartbeat loss
- test input: worker starts processing but does not finish before heartbeat timeout
- clock/timer assumptions: heartbeat timeout is configured short enough for test execution

---

## Action

1. Start Grael.
2. Start one worker registered for activity type `slow_step`.
3. Submit a workflow containing one `slow_step` node.
4. Wait for the worker to receive the task and begin execution.
5. Stop the worker or stop its heartbeats without completing the task.
6. Wait for the heartbeat timeout window to pass.
7. Query `GetRun`.
8. Query `ListEvents`.

---

## Expected Visible Outcome

- the task does not remain stuck in perpetual running state
- after heartbeat loss, the active lease expires
- the node becomes eligible for retry or failure handling according to policy
- `ListEvents` shows lease expiry as part of the recovery path

Checklist:

- expected API state: node no longer held indefinitely by the lost worker
- expected terminal outcome: depends on retry policy; the key behavior is lease expiry and progress recovery
- expected event-family visibility: dispatch, node start, heartbeat loss consequence, lease expiry
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted`
3. `NodeStarted`
4. Worker stops heartbeating
5. `LeaseExpired`
6. Follow-on retry or failure events according to policy

---

## Failure Evidence

- node remains `RUNNING` forever after worker disappearance
- no lease expiry becomes visible after heartbeat timeout
- run makes no further progress and no terminal handling occurs

---

## Pass Criteria

- worker liveness loss results in visible lease expiry
- the node is no longer blocked forever by the missing worker
- run handling continues according to retry or failure semantics

---

## Notes

- This UAT validates liveness detection, not the specific retry policy outcome.
- Use a deterministic worker fixture that can stop heartbeats on command.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is a good candidate for automation once the test harness can control worker heartbeats precisely.

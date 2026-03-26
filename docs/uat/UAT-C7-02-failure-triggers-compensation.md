# UAT-C7-02 Failure Triggers Compensation

## Metadata

- `UAT ID`: `UAT-C7-02`
- `Title`: Permanent failure triggers sequential compensation of completed nodes only
- `Capability`: `C7`
- `Priority`: `late-v1`
- `Related task(s)`: `C7` compensation stack and unwind tasks
- `Depends on`: `C4`, `C7`, `C9`

---

## Intent

This UAT proves that when a workflow fails after some nodes have completed, Grael can unwind completed work in reverse order using compensation handlers.

If this UAT passes, an operator can trust that:

- compensation applies only to completed work
- unwind order is sequential and consistent
- permanent failure can lead to a controlled compensation path

---

## Setup

- engine state: Grael server is running
- worker state: workers are running for forward and compensation activities
- workflow shape: multi-node workflow where nodes `A` and `B` complete with compensation defined, and node `C` fails permanently
- test input: `A` and `B` succeed, `C` fails non-retryably
- clock/timer assumptions: retries for `C` are disabled or exhausted to force permanent failure

---

## Action

1. Start Grael.
2. Start workers for the forward and compensation activity types.
3. Submit a workflow where completed nodes have compensation handlers.
4. Allow nodes `A` and `B` to complete successfully.
5. Have node `C` fail permanently.
6. Wait for compensation to execute.
7. Query `GetRun`.
8. Query `ListEvents`.

---

## Expected Visible Outcome

- the workflow does not simply stop at the first permanent failure
- compensation begins after the permanent failure path is established
- compensation executes only for nodes that previously completed
- compensation runs in reverse order of completed compensable work

Checklist:

- expected API state: run enters visible compensation path and reaches compensation-related terminal outcome
- expected terminal outcome: compensated or compensation-partial/failure according to configured behavior
- expected event-family visibility: failure, compensation start, compensation actions, compensation completion
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. Forward execution completes for `A`
2. Forward execution completes for `B`
3. Permanent failure occurs for `C`
4. `CompensationStarted`
5. Compensation action for `B`
6. Compensation action for `A`
7. `CompensationCompleted` or compensation terminal equivalent

---

## Failure Evidence

- compensation runs for nodes that never completed
- compensation order is not reverse to completed work order
- permanent failure leaves previously completed compensable work untouched when compensation is configured

---

## Pass Criteria

- permanent failure triggers visible compensation flow
- only completed nodes are compensated
- compensation order is reverse to the forward completion order

---

## Notes

- This UAT should avoid mixing cancellation and compensation in the same initial scenario.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is automatable with deterministic worker fixtures for both forward and compensation handlers.

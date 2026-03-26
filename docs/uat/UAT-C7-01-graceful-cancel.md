# UAT-C7-01 Graceful Cancel

## Metadata

- `UAT ID`: `UAT-C7-01`
- `Title`: Graceful cancel stops remaining work and reaches a terminal cancelled outcome
- `Capability`: `C7`
- `Priority`: `late-v1`
- `Related task(s)`: `C7` cancellation propagation tasks
- `Depends on`: `C3`, `C4`, `C7`, `C9`

---

## Intent

This UAT proves that an operator can request cancellation and Grael will stop remaining work in a controlled way.

If this UAT passes, an operator can trust that:

- a running workflow is stoppable
- pending and ready work does not continue indefinitely after cancellation
- the run reaches a clear cancelled terminal outcome

---

## Setup

- engine state: Grael server is running
- worker state: one or more workers are running
- workflow shape: multi-node workflow with at least one running node and at least one not-yet-started node
- test input: workers are able to cooperate with graceful cancellation behavior
- clock/timer assumptions: grace period is configured and observable

---

## Action

1. Start Grael.
2. Start workers for the workflow activity types.
3. Submit a workflow with multiple nodes so that some work is active and some work is pending.
4. Wait until at least one node is running.
5. Call `CancelRun`.
6. Allow the system to propagate graceful cancellation.
7. Query `GetRun`.
8. Query `ListEvents`.

---

## Expected Visible Outcome

- pending or ready work does not continue to dispatch after cancellation
- running work is signaled through the graceful cancel path
- the run reaches a cancelled terminal outcome
- event history shows a coherent cancellation sequence

Checklist:

- expected API state: run transitions toward cancellation and then terminal cancelled outcome
- expected terminal outcome: cancelled
- expected event-family visibility: cancellation request plus downstream cancel handling
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. `WorkflowStarted`
2. Normal execution events begin
3. `CancellationRequested`
4. Node-level cancellation handling occurs for remaining work
5. `CancellationCompleted`

---

## Failure Evidence

- new work continues dispatching after cancellation
- the run remains indefinitely active after `CancelRun`
- cancellation leaves nodes stranded without a terminal run outcome

---

## Pass Criteria

- cancellation request leads to visible suppression of remaining work
- run reaches a coherent terminal cancelled outcome
- cancellation behavior is externally visible through read APIs and event history

---

## Notes

- This UAT validates graceful cancellation only; force-revoke behavior is out of v1 scope.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is automatable with cooperative worker fixtures and a controlled grace-period window.

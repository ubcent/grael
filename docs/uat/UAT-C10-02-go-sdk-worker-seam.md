# UAT-C10-02 Go SDK Worker Seam

## Metadata

- `UAT ID`: `UAT-C10-02`
- `Title`: A Go worker registered by activity type can handle tasks through the thin SDK seam
- `Capability`: `C10`
- `Priority`: `late-v1`
- `Related task(s)`: `C10` SDK seam tasks
- `Depends on`: `C4`, `C10`

---

## Intent

This UAT proves that the thin Go worker SDK seam is sufficient for a worker author to register handlers and process tasks without dropping into low-level protocol details.

If this UAT passes, an operator or integrator can trust that:

- a worker can be implemented through the intended SDK boundary
- activity-type registration works through the SDK
- task handling through the SDK results in normal Grael execution outcomes

---

## Setup

- engine state: Grael server is running
- worker state: a Go worker built using the thin SDK seam is available
- workflow shape: one or more nodes whose activity types are handled by the SDK-based worker
- test input: success-path input
- clock/timer assumptions: no special timing requirements

---

## Action

1. Start Grael.
2. Start a Go worker implemented through the SDK seam.
3. Register an activity handler by activity type through the SDK.
4. Submit a workflow that targets that activity type.
5. Allow the SDK worker to receive and complete the task.
6. Query `GetRun`.
7. Query `ListEvents`.

---

## Expected Visible Outcome

- the SDK-based worker successfully registers for its activity type
- the worker receives tasks through the SDK
- successful task completion through the SDK has the same visible result as direct worker protocol usage
- the run reaches the expected successful outcome

Checklist:

- expected API state: normal run progression and completion
- expected terminal outcome: successful completion
- expected event-family visibility: standard dispatch/start/completion events
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. `WorkflowStarted`
2. Task dispatch events for the SDK-handled activity
3. Completion events resulting from the SDK worker
4. `WorkflowCompleted`

---

## Failure Evidence

- SDK-based worker cannot receive tasks that direct workers could handle
- SDK abstraction changes visible execution semantics
- successful SDK handler execution does not result in normal node/run completion

---

## Pass Criteria

- SDK-based worker can register, receive, and complete work successfully
- externally visible behavior matches the intended worker contract

---

## Notes

- This scenario validates the seam, not the richness of the SDK surface.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is automatable once the Go SDK exists and a small reference worker can be built in the test harness.

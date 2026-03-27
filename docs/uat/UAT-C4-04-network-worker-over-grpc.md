# UAT-C4-04 Network Worker Over gRPC

## Metadata

- `UAT ID`: `UAT-C4-04`
- `Title`: A networked worker can register, poll, heartbeat, and complete work over gRPC
- `Capability`: `C4`
- `Priority`: `late-v1`
- `Related task(s)`: `T41`, `T43`, `T45`
- `Depends on`: `C4`, `C9`

---

## Intent

This UAT proves that Grael's worker protocol is not only callable in-process, but is honestly available over the intended network transport.

If this UAT passes, an operator or integrator can trust that:

- a remote worker can register by activity type over gRPC
- the worker can receive work through long-polling
- heartbeats over gRPC preserve lease semantics
- successful completion over gRPC produces the same visible run behavior as local service calls

---

## Setup

- engine state: Grael gRPC server is running locally
- worker state: a test worker process or fixture can connect to the gRPC endpoint
- workflow shape: one or more nodes whose activity types are handled by the remote worker
- test input: success-path input
- clock/timer assumptions: heartbeat interval is short enough to observe within the test window

---

## Action

1. Start Grael with the gRPC server enabled.
2. Connect a worker client over gRPC.
3. Register the worker for one or more activity types.
4. Submit a workflow that targets those activity types.
5. Poll for a task over gRPC.
6. Send at least one heartbeat while the task is active.
7. Complete the task over gRPC.
8. Query `GetRun` and `ListEvents`.

---

## Expected Visible Outcome

- the remote worker is accepted by the registry
- a task is delivered through gRPC polling
- heartbeat activity is reflected in the run history
- task completion through gRPC advances the run exactly as local completion would
- the run reaches the expected outcome

Checklist:

- expected API state: normal node progression through running and completion
- expected terminal outcome: successful completion for the chosen workflow
- expected event-family visibility: lease grant, node start, heartbeat, node completion, workflow completion
- expected behavior after restart, if relevant: not required for this baseline scenario

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted`
3. `NodeStarted`
4. `HeartbeatRecorded`
5. `NodeCompleted`
6. `WorkflowCompleted`

---

## Failure Evidence

- the remote worker can register but cannot receive tasks
- gRPC completion changes behavior relative to local service completion
- heartbeats over gRPC do not preserve lease ownership correctly
- the worker protocol behaves differently depending on transport

---

## Pass Criteria

- the worker protocol functions correctly over gRPC
- externally visible execution semantics match the intended worker contract

---

## Notes

- This scenario validates transport honesty, not language-specific worker SDK ergonomics.

---

## Automation Readiness

- `Suitable for CI integration`

This should be automatable with a local in-process test server and a gRPC client fixture.

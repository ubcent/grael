# UAT-C9-04 gRPC Orchestration Surface

## Metadata

- `UAT ID`: `UAT-C9-04`
- `Title`: Remote orchestration over gRPC can start, control, and inspect a run without changing semantics
- `Capability`: `C9`
- `Priority`: `late-v1`
- `Related task(s)`: `T41`, `T42`, `T45`
- `Depends on`: `C1`, `C4`, `C8`, `C9`

---

## Intent

This UAT proves that Grael's orchestration and inspection surface is available over gRPC as a transport, not as a second runtime interpretation.

If this UAT passes, an operator or API-layer integrator can trust that:

- a remote client can start a run over gRPC
- the same client can inspect state and raw history remotely
- control actions such as checkpoint approval and cancellation retain their existing semantics
- network transport does not widen or reinterpret the runtime contract

---

## Setup

- engine state: Grael gRPC server is running locally
- worker state: workers are available for the chosen workflow
- workflow shape: workflow that exercises start, inspect, and at least one control action
- test input: deterministic input
- clock/timer assumptions: no special constraints beyond normal local execution

---

## Action

1. Start Grael with the gRPC server enabled.
2. Connect a remote orchestration client.
3. Call `StartRun` over gRPC.
4. Query `GetRun` during active execution.
5. Query `ListEvents` during or after execution.
6. If the workflow includes a checkpoint, call `ApproveCheckpoint` over gRPC.
7. If the workflow includes cancellation coverage, call `CancelRun` over gRPC in the relevant scenario.

---

## Expected Visible Outcome

- `StartRun` returns a real run id
- `GetRun` returns the same derived state semantics as the in-process service surface
- `ListEvents` returns the same raw causal history semantics as the in-process service surface
- control operations change the run only through the normal evented runtime path

Checklist:

- expected API state: remote client can start and inspect the run
- expected terminal outcome: depends on the chosen workflow and control action
- expected event-family visibility: workflow start plus the relevant control/read families for the chosen scenario
- expected behavior after restart, if relevant: not required for this baseline scenario

---

## Expected Event Sequence

The exact event sequence depends on the workflow, but it must include:

1. `WorkflowStarted`
2. normal execution events for the chosen workflow
3. any approved control-path event caused by the remote gRPC call
4. a coherent `GetRun` and `ListEvents` view of that committed history

---

## Failure Evidence

- gRPC orchestration calls succeed but produce different behavior than the in-process service surface
- `GetRun` over gRPC returns a transport-specific projection instead of the normal read model
- `ListEvents` over gRPC omits or reshapes committed history
- control actions require transport-local logic to appear correct

---

## Pass Criteria

- remote orchestration and inspection work correctly over gRPC
- semantics remain aligned with the existing service/runtime contract

---

## Notes

- This scenario validates transport integrity, not auth, TLS, or hosted deployment concerns.

---

## Automation Readiness

- `Suitable for CI integration`

This should be automatable with a local gRPC client fixture and deterministic workflows.

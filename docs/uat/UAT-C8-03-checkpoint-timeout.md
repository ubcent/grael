# UAT-C8-03 Checkpoint Timeout

## Metadata

- `UAT ID`: `UAT-C8-03`
- `Title`: Checkpoint timeout fails the waiting node when approval never arrives
- `Capability`: `C8`
- `Priority`: `late-v1`
- `Related task(s)`: `C5` checkpoint timeout timer tasks, `C8` approval timeout tasks
- `Depends on`: `C5`, `C8`, `C9`

---

## Intent

This UAT proves that a checkpoint cannot wait forever when approval never arrives.

If this UAT passes, an operator can trust that:

- `AWAITING_APPROVAL` is bounded by timeout behavior
- unattended approval gates do not stall runs indefinitely
- timeout outcome is externally visible

---

## Setup

- engine state: Grael server is running
- worker state: one worker is running for the checkpointing activity type
- workflow shape: workflow with a node that requests checkpoint approval and has a configured approval timeout
- test input: worker returns a checkpoint request and no approval is ever sent
- clock/timer assumptions: checkpoint timeout is short enough for test runtime

---

## Action

1. Start Grael.
2. Start the worker.
3. Submit the workflow.
4. Let the node enter `AWAITING_APPROVAL`.
5. Do not call `ApproveCheckpoint`.
6. Wait for the checkpoint timeout to expire.
7. Query `GetRun`.
8. Query `ListEvents`.

---

## Expected Visible Outcome

- the node does not remain indefinitely in `AWAITING_APPROVAL`
- after timeout, the node transitions into failure semantics
- the run proceeds into retry or terminal failure handling according to policy
- timeout path is visible in event history

Checklist:

- expected API state: node leaves waiting state after timeout
- expected terminal outcome: depends on retry policy, but waiting is bounded
- expected event-family visibility: checkpoint reached, timeout-triggered failure handling
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. `CheckpointReached`
2. Node enters `AWAITING_APPROVAL`
3. Timeout-related timer fires
4. Failure event representing checkpoint timeout semantics occurs

---

## Failure Evidence

- node remains indefinitely in `AWAITING_APPROVAL`
- no timeout-driven failure path becomes visible
- run is permanently stalled by a missing approval

---

## Pass Criteria

- checkpoint waiting is bounded by timeout handling
- timeout outcome is externally visible
- run no longer depends forever on absent approval

---

## Notes

- This scenario is about bounded waiting, not the separate absolute deadline rule.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario should automate well with a short timeout window and a deterministic checkpointing worker.

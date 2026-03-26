# UAT-C8-02 Approval After Restart

## Metadata

- `UAT ID`: `UAT-C8-02`
- `Title`: Approval after restart resumes the waiting node correctly
- `Capability`: `C8`
- `Priority`: `late-v1`
- `Related task(s)`: `C8` checkpoint durability and approval tasks
- `Depends on`: `C1`, `C4`, `C8`, `C9`

---

## Intent

This UAT proves that checkpoint waiting state is durable and can be resumed by approval after process restart.

If this UAT passes, an operator can trust that:

- approval-required state survives restart
- Grael does not lose track of the waiting node
- approval after recovery correctly resumes execution

---

## Setup

- engine state: Grael server is running with a clean data directory
- worker state: worker is running for the checkpointing node's activity type
- workflow shape: workflow containing one node that requests checkpoint approval and can later resume to success
- test input: worker first returns a checkpoint request, then resumes successfully after approval
- clock/timer assumptions: timeout window is long enough to allow restart and approval

Additional setup constraint:

- the test harness must be able to restart Grael while the node is in `AWAITING_APPROVAL`

---

## Action

1. Start Grael.
2. Start the worker.
3. Submit the workflow.
4. Let the node enter `AWAITING_APPROVAL`.
5. Stop Grael while approval is still pending.
6. Restart Grael using the same data directory.
7. Query `GetRun` and confirm the node is still waiting.
8. Call `ApproveCheckpoint`.
9. Allow the worker to resume and complete the node.
10. Query `GetRun` and `ListEvents`.

---

## Expected Visible Outcome

- after restart, the node is still shown as waiting for approval
- approval is accepted after restart
- the node resumes execution and completes successfully
- the workflow reaches successful terminal completion if no other blockers exist

Checklist:

- expected API state: waiting state survives restart, then clears after approval
- expected terminal outcome: successful completion
- expected event-family visibility: checkpoint reached, restart, approval, resumed completion
- expected behavior after restart: approval remains possible and effective

---

## Expected Event Sequence

1. `CheckpointReached`
2. Node enters `AWAITING_APPROVAL`
3. Grael restarts
4. Node remains visible as awaiting approval
5. `CheckpointApproved`
6. Node resumes and completes
7. `WorkflowCompleted`

---

## Failure Evidence

- waiting node disappears after restart
- approval cannot be applied after restart
- node remains stuck waiting even after approval

---

## Pass Criteria

- waiting state persists across restart
- approval after restart resumes the correct node
- resumed execution leads to completion

---

## Notes

- This is the durability companion to the basic checkpoint pause scenario.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is automatable with a restart-capable harness and deterministic worker behavior around approval.

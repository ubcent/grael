# UAT-C8-01 Checkpoint Pauses One Node

## Metadata

- `UAT ID`: `UAT-C8-01`
- `Title`: A checkpoint pauses one node while unrelated nodes keep running
- `Capability`: `C8`
- `Priority`: `late-v1`
- `Related task(s)`: `C8` checkpoint and approval-flow tasks
- `Depends on`: `C4`, `C8`, `C9`

---

## Intent

This UAT proves that a checkpoint blocks only the node that requested approval, not the entire run.

If this UAT passes, an operator can trust that:

- a worker can request approval for a specific node
- that node enters waiting state
- unrelated runnable work continues

---

## Setup

- engine state: Grael server is running
- worker state: workers are running for relevant activity types
- workflow shape: graph with one node that requests a checkpoint and at least one unrelated node that can continue independently
- test input: checkpointing node returns an approval request; unrelated node succeeds normally
- clock/timer assumptions: checkpoint timeout is long enough not to interfere with the scenario

---

## Action

1. Start Grael.
2. Start workers for the workflow activity types.
3. Submit a workflow containing one checkpointing node and one unrelated runnable node.
4. Let the checkpointing node return a checkpoint request.
5. Query `GetRun`.
6. Observe execution of the unrelated node while approval has not yet been granted.
7. Query `ListEvents`.

---

## Expected Visible Outcome

- the checkpointing node enters `AWAITING_APPROVAL`
- unrelated node execution continues while approval is pending
- the run remains active rather than globally blocked

Checklist:

- expected API state: one node waiting, unrelated work still progressing
- expected terminal outcome: not required yet; this scenario focuses on the pause behavior
- expected event-family visibility: checkpoint reached plus unrelated node progress
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. Normal execution begins
2. Checkpointing node reaches `CheckpointReached`
3. Node enters `AWAITING_APPROVAL`
4. Independent node dispatch/completion events continue while approval is pending

---

## Failure Evidence

- the entire workflow stops progressing when one node requests approval
- unrelated runnable work does not continue
- checkpointing node continues without approval

---

## Pass Criteria

- exactly the checkpointing node pauses for approval
- unrelated work remains executable
- run state reflects selective waiting rather than global blocking

---

## Notes

- This scenario isolates the non-blocking property of checkpoints from the approval/resume path.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is automatable with a deterministic worker that emits a checkpoint request.

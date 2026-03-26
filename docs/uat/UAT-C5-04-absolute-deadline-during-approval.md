# UAT-C5-04 Absolute Deadline During Approval

## Metadata

- `UAT ID`: `UAT-C5-04`
- `Title`: Absolute deadline still fires while a node awaits approval
- `Capability`: `C5`
- `Priority`: `late-v1`
- `Related task(s)`: `C5` absolute deadline enforcement tasks, `C8` checkpoint timing tasks
- `Depends on`: `C5`, `C8`, `C9`

---

## Intent

This UAT proves that a checkpoint cannot be used to bypass the node's hard absolute deadline.

If this UAT passes, an operator can trust that:

- approval waiting does not extend hard execution limits
- a node can time out while still awaiting approval
- Grael enforces the absolute deadline independently of checkpoint pause semantics

---

## Setup

- engine state: Grael server is running
- worker state: one worker is running for the checkpointing activity type
- workflow shape: single-node or small workflow with a node that requests approval and has an absolute deadline
- test input: worker returns a checkpoint request quickly; approval is intentionally withheld
- clock/timer assumptions: absolute deadline is short enough to expire while the node is in `AWAITING_APPROVAL`

---

## Action

1. Start Grael.
2. Start the worker.
3. Submit a workflow containing a node with checkpoint behavior and an absolute deadline.
4. Let the node enter `AWAITING_APPROVAL`.
5. Do not approve the checkpoint.
6. Wait for the absolute deadline to pass.
7. Query `GetRun`.
8. Query `ListEvents`.

---

## Expected Visible Outcome

- the node enters `AWAITING_APPROVAL`
- after the absolute deadline passes, the node no longer remains in waiting state
- timeout/failure semantics become visible even though no approval decision was made
- the run proceeds according to retry or failure policy rather than waiting indefinitely

Checklist:

- expected API state: node leaves approval waiting due to deadline enforcement
- expected terminal outcome: depends on retry policy, but approval cannot keep the node alive past its hard limit
- expected event-family visibility: checkpoint reached, absolute deadline firing, timeout/failure consequence
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. `CheckpointReached`
2. Node enters `AWAITING_APPROVAL`
3. Absolute deadline-related timer fires
4. Failure event representing absolute-deadline timeout semantics occurs

---

## Failure Evidence

- node remains in `AWAITING_APPROVAL` past the absolute deadline
- approval waiting implicitly extends the hard deadline
- no timeout/failure path is visible after the absolute deadline has passed

---

## Pass Criteria

- absolute deadline is enforced even while the node awaits approval
- the node does not remain waiting past the hard deadline
- timeout/failure outcome is externally visible

---

## Notes

- This scenario should remain separate from checkpoint timeout UAT because it validates the stronger hard-limit rule.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is deterministic with a short deadline and a worker fixture that always requests approval.

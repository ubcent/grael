# UAT-C11-01 Core Demo E2E

## Metadata

- `UAT ID`: `UAT-C11-01`
- `Title`: The core demo workflow completes with dynamic spawn, retry, approval, and recovery
- `Capability`: `C11`
- `Priority`: `late-v1`
- `Related task(s)`: cross-capability demo and acceptance tasks
- `Depends on`: `C1`, `C4`, `C5`, `C6`, `C8`, `C9`, `C11`

---

## Intent

This UAT proves the Grael v1 product story end to end through one visible workflow that demonstrates its main differentiators.

If this UAT passes, an operator can trust that Grael v1 can:

- grow the graph dynamically during execution
- recover automatically from retryable failure
- pause one node for approval without stopping the rest
- recover from process restart without losing progress

---

## Setup

- engine state: Grael server is running with restart capability available
- worker state: workers are running for discovery, analysis, and summary activity types
- workflow shape: demo workflow that includes one discovery node, spawned analysis nodes, at least one retryable analysis failure, one checkpointing node, and a final gather/summary path
- test input: deterministic input that causes the intended demo behavior
- clock/timer assumptions: retry and approval windows are short enough to observe in one run

---

## Action

1. Start Grael.
2. Start the required workers.
3. Submit the demo workflow.
4. Let the discovery node spawn additional analysis nodes.
5. Allow one spawned node to fail retryably, then succeed on retry.
6. Allow one node to enter checkpoint approval waiting while other nodes continue.
7. Approve the checkpoint.
8. Restart Grael mid-execution after some progress is already committed.
9. Allow the workflow to resume and finish.
10. Query `GetRun` and `ListEvents`.

---

## Expected Visible Outcome

- the graph expands during execution
- retryable failure recovers automatically
- one node waits for approval while unrelated work continues
- restart does not lose committed progress
- the workflow reaches successful terminal completion

Checklist:

- expected API state: dynamic graph and live progress visible through `GetRun`
- expected terminal outcome: successful completion
- expected event-family visibility: spawn, retry, checkpoint, approval, restart-resumed completion
- expected behavior after restart: committed progress is preserved and execution resumes

---

## Expected Event Sequence

The exact sequence depends on the demo workflow, but it must include:

1. `WorkflowStarted`
2. Dynamic spawn from a completed node
3. Retryable failure followed by retry timer and successful retry
4. `CheckpointReached`
5. Continued progress from unrelated nodes during approval wait
6. `CheckpointApproved`
7. Grael restart
8. Resumed execution after restart
9. `WorkflowCompleted`

---

## Failure Evidence

- graph never grows at runtime
- retry requires manual intervention
- checkpoint blocks the whole run
- restart loses committed progress
- final workflow cannot complete after combining these behaviors

---

## Pass Criteria

- the demo workflow visibly exercises Grael's key v1 differentiators in one run
- all major visible behaviors complete successfully in a single end-to-end scenario
- execution remains durable across restart

---

## Notes

- This is the flagship acceptance scenario for Grael v1.
- It should be the last acceptance scenario automated, after the lower-level UATs are already green.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is automatable, but it is best treated as a composed end-to-end acceptance run rather than a fast smoke test.

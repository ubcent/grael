# Grael v1 Sprint 4 Plan

This document defines the fourth implementation sprint for Grael v1.

Sprint 4 adds the operational control-flow features that make Grael usable in real long-running runs: human approval gates, graceful cancellation, and basic sequential compensation.

---

## Sprint Goal

Build the first operator-aware version of Grael that can:

- pause one node for approval without blocking the whole run
- resume that node after approval, including after restart
- fail waiting approval nodes when timeout or hard deadline rules require it
- cancel active runs gracefully
- compensate previously completed work after permanent failure
- resume unfinished compensation after restart

At the end of Sprint 4, Grael should support the main operational intervention paths expected from a durable workflow engine for AI-agent workloads.

---

## In Scope

The sprint includes the following tasks from `docs/V1_TASK_BACKLOG.md`:

1. `T16` Absolute deadline enforcement
2. `T20` CancelRun API and cancellation request persistence
3. `T21` Graceful cancel propagation by node state
4. `T22` Compensation stack construction from completed nodes
5. `T23` Sequential compensation execution
6. `T24` Compensation recovery after restart
7. `T25` Checkpoint request and awaiting-approval state
8. `T26` ApproveCheckpoint API and resume flow
9. `T27` Checkpoint timeout handling
10. `T28` Checkpoint recovery across restart

Implementation note:

- runtime-layer absolute-deadline timer scheduling and timeout semantics already exist from post-Sprint-2 execution-core work
- Sprint 4 still owns the full operator-visible acceptance outcome because `UAT-C5-04` depends on checkpoint waiting semantics that do not exist before `T25` to `T28`

---

## Why This Slice

Sprints 1 through 3 create a durable engine that can execute real work and grow its graph dynamically. Sprint 4 makes that engine survivable under real operational conditions.

It gives:

- selective human-in-the-loop approval
- bounded waiting behavior
- graceful stopping
- unwind behavior after permanent failure
- restart-safe operational control flow

without yet dragging in:

- Go SDK polish
- final demo composition as a separate deliverable
- non-v1 cancellation modes
- broader platform features

This is the right fourth slice because once the execution core is solid, the next biggest gap is not raw capability, but operational control and failure handling.

---

## Out of Scope

Do not pull these into Sprint 4:

- `RevokeAndAbandon` style force-cancel behavior
- sub-workflows
- memory-layer behavior
- advanced policy engines
- richer compensation modes such as parallel unwind
- Go SDK expansion
- final product demo harness beyond what is necessary for acceptance validation

Also avoid overbuilding:

- approval escalation systems
- notification platforms
- generalized remediation branches outside basic compensation

Sprint 4 should solve the minimal v1 operational control flow cleanly, not invent a broader workflow-ops platform.

---

## Expected End State

By the end of Sprint 4:

- a node can enter `AWAITING_APPROVAL`
- unrelated work can continue while that node waits
- approval can resume the node, including after restart
- waiting nodes can time out
- absolute deadlines still apply during approval waiting
- an operator can cancel a run gracefully
- completed compensable nodes can unwind after permanent failure
- unfinished compensation can resume after restart

This is the point where Grael should feel viable for real long-running orchestration instead of only happy-path execution.

---

## Exit Criteria

Sprint 4 is complete when all of the following are true:

- the in-scope tasks are implemented to a usable baseline
- approval, cancel, and compensation flows are externally visible through APIs and event history
- restart-safe behavior exists for both checkpoints and compensation
- the following UATs can pass:
  - [UAT-C5-04-absolute-deadline-during-approval.md](docs/uat/UAT-C5-04-absolute-deadline-during-approval.md)
  - [UAT-C7-01-graceful-cancel.md](docs/uat/UAT-C7-01-graceful-cancel.md)
  - [UAT-C7-02-failure-triggers-compensation.md](docs/uat/UAT-C7-02-failure-triggers-compensation.md)
  - [UAT-C7-03-compensation-resumes-after-restart.md](docs/uat/UAT-C7-03-compensation-resumes-after-restart.md)
  - [UAT-C8-01-checkpoint-pauses-one-node.md](docs/uat/UAT-C8-01-checkpoint-pauses-one-node.md)
  - [UAT-C8-02-approval-after-restart.md](docs/uat/UAT-C8-02-approval-after-restart.md)
  - [UAT-C8-03-checkpoint-timeout.md](docs/uat/UAT-C8-03-checkpoint-timeout.md)

---

## Stretch Goal

If Sprint 4 finishes early, the best stretch target is:

- lightly exercise the new checkpoint and compensation flows inside the eventual `T35` demo composition

Why:

- it validates that the control-flow features compose with the execution core
- it gives early product confidence before the final demo sprint

Do not treat the stretch goal as part of the committed sprint scope.

---

## Risks

The main Sprint 4 risks are:

- checkpoint waiting accidentally blocking the whole run
- approval state not surviving restart cleanly
- timeout semantics becoming inconsistent between checkpoint timeout and absolute deadline
- compensation including nodes that never completed
- replaying already completed compensation actions after restart
- cancellation semantics becoming ambiguous across different node states

---

## Review Questions

At the midpoint and end of the sprint, the team should ask:

1. Can one node wait for approval while the rest of the run keeps making progress?
2. If the process restarts mid-approval or mid-compensation, do we resume cleanly instead of starting over?
3. Can a permanent failure unwind only the work that actually completed?
4. Does `CancelRun` produce a clear, externally understandable lifecycle rather than a vague partial stop?

If the answer to any of these is "no", the sprint likely needs scope correction before proceeding.

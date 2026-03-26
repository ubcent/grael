# UAT-C9-01 GetRun Coherent View

## Metadata

- `UAT ID`: `UAT-C9-01`
- `Title`: `GetRun` returns a coherent current-state view during active execution
- `Capability`: `C9`
- `Priority`: `core`
- `Related task(s)`: `C9` read API tasks
- `Depends on`: `C3`, `C4`, `C9`

---

## Intent

This UAT proves that `GetRun` gives an operator a coherent current view of the run while execution is still in progress.

If this UAT passes, an operator can trust that:

- current node states are externally inspectable during execution
- the read model reflects the run's live derived state
- `GetRun` is useful as the minimal operational visibility surface for v1

---

## Setup

- engine state: Grael server is running
- worker state: one or more workers are running
- workflow shape: workflow with at least two visible phases so state changes can be observed over time
- test input: normal success-path input
- clock/timer assumptions: no special timing requirements beyond being able to observe intermediate state

---

## Action

1. Start Grael.
2. Start the necessary workers.
3. Submit a workflow with multiple nodes.
4. Call `GetRun` shortly after start.
5. Call `GetRun` again while some nodes are active or waiting on dependencies.
6. Call `GetRun` after the workflow completes.

---

## Expected Visible Outcome

- `GetRun` returns the same `RunID` through the workflow lifecycle
- node states shown by `GetRun` evolve coherently over time
- intermediate `GetRun` calls reflect active execution rather than only start/end snapshots
- terminal `GetRun` reflects the final outcome consistently

Checklist:

- expected API state: coherent current run state at multiple observation points
- expected terminal outcome: visible in final `GetRun`
- expected event-family visibility: not primary for this scenario
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

Event sequence is not the primary focus. The key requirement is that `GetRun` reflects the derived state implied by the underlying execution history at each observation point.

---

## Failure Evidence

- `GetRun` returns stale or contradictory node states
- node states regress in ways unsupported by execution history
- `GetRun` is only usable before start or after completion, but not during active execution

---

## Pass Criteria

- `GetRun` provides a coherent current-state view throughout run execution
- intermediate and terminal views remain consistent with observable workflow progress

---

## Notes

- This scenario is intentionally API-centric and should not over-specify internal events.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is easy to automate with repeated API polling during a deterministic workflow.

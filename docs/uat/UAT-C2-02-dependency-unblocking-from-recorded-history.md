# UAT-C2-02 Dependency Unblocking From Recorded History

## Metadata

- `UAT ID`: `UAT-C2-02`
- `Title`: Ready dependency unblocking follows recorded event history only
- `Capability`: `C2`
- `Priority`: `core`
- `Related task(s)`: `C2` state application and scheduler correctness tasks
- `Depends on`: `C1`, `C2`, `C3`, `C9`

---

## Intent

This UAT proves that a downstream node becomes runnable because the required upstream events were recorded, not because of incidental in-memory state or external observation.

If this UAT passes, an operator can trust that:

- dependency unblocking is driven by persisted execution history
- Grael rebuilds readiness consistently after restart
- scheduler behavior is aligned with recorded facts

---

## Setup

- engine state: Grael server is running
- worker state: one worker is running for activity type `step`
- workflow shape: dependency graph `A -> B`
- test input: both nodes succeed
- clock/timer assumptions: no retry or timeout behavior required

Additional setup constraint:

- the test harness should observe readiness before and after a restart boundary

---

## Action

1. Start Grael.
2. Start one worker for activity type `step`.
3. Submit the workflow `A -> B`.
4. Confirm that node `B` is not runnable before node `A` completes.
5. Complete node `A`.
6. Observe that node `B` becomes runnable.
7. Restart Grael before node `B` completes.
8. Query `GetRun` after restart and continue the workflow.
9. Query `ListEvents`.

---

## Expected Visible Outcome

- node `B` is not dispatched before node `A` completion is recorded
- node `B` becomes dispatchable after node `A` completion
- after restart, node `B` remains in the correct readiness path based on recorded history
- the workflow completes successfully

Checklist:

- expected API state: readiness of `B` aligns with completion state of `A`
- expected terminal outcome: workflow completes successfully
- expected event-family visibility: upstream completion before downstream dispatch
- expected behavior after restart: readiness is reconstructed from persisted events

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted` and `NodeStarted` for `A`
3. `NodeCompleted` for `A`
4. `LeaseGranted` and `NodeStarted` for `B`
5. Grael restarts optionally between `B` readiness and completion
6. `NodeCompleted` for `B`
7. `WorkflowCompleted`

---

## Failure Evidence

- node `B` is dispatched before `A` completes
- node `B` loses readiness after restart despite `A` completion being persisted
- dependency handling differs before and after restart for the same recorded history

---

## Pass Criteria

- downstream readiness is observable only after the required upstream completion is recorded
- restart does not alter dependency-driven readiness derived from persisted history
- workflow completes with correct dependency order

---

## Notes

- This scenario should stay focused on recorded-history-driven readiness, not on parallel scheduling policy.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is deterministic and should be straightforward to automate.

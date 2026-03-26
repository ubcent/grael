# UAT-C7-03 Compensation Resumes After Restart

## Metadata

- `UAT ID`: `UAT-C7-03`
- `Title`: Compensation resumes correctly after restart mid-unwind
- `Capability`: `C7`
- `Priority`: `late-v1`
- `Related task(s)`: `C7` compensation durability tasks
- `Depends on`: `C1`, `C7`, `C9`

---

## Intent

This UAT proves that compensation progress is durable and can resume after Grael restarts in the middle of an unwind sequence.

If this UAT passes, an operator can trust that:

- restart does not reset compensation from scratch
- already completed compensation work is preserved
- unfinished unwind can continue after recovery

---

## Setup

- engine state: Grael server is running with a clean data directory
- worker state: workers are running for forward and compensation activities
- workflow shape: at least two compensable completed nodes followed by permanent failure
- test input: first compensation action finishes, restart occurs before the full unwind completes
- clock/timer assumptions: no special timer behavior required

Additional setup constraint:

- the harness must be able to restart Grael after at least one compensation action has been committed

---

## Action

1. Start Grael.
2. Start workers for forward and compensation handlers.
3. Submit a workflow that will enter compensation after a permanent failure.
4. Allow compensation to start and complete at least one compensation action.
5. Stop Grael before the full compensation sequence finishes.
6. Restart Grael using the same data directory.
7. Allow compensation to resume and finish.
8. Query `GetRun`.
9. Query `ListEvents`.

---

## Expected Visible Outcome

- compensation progress completed before restart remains visible after restart
- Grael resumes from the remaining unwind work instead of replaying already finished compensation actions
- the run reaches its compensation terminal outcome after restart

Checklist:

- expected API state: compensation remains in progress or resumable after restart
- expected terminal outcome: compensation-related terminal state
- expected event-family visibility: compensation start, at least one completed compensation action before restart, remaining actions after restart
- expected behavior after restart: compensation resumes rather than restarts from zero

---

## Expected Event Sequence

1. Permanent failure triggers compensation
2. `CompensationStarted`
3. First `CompensationActionCompleted`
4. Grael restarts
5. Remaining compensation action events occur
6. `CompensationCompleted` or compensation terminal equivalent

---

## Failure Evidence

- already completed compensation action is repeated after restart
- compensation progress is lost
- run becomes stuck in non-terminal compensation state after restart

---

## Pass Criteria

- completed compensation steps survive restart
- remaining compensation work resumes and finishes after recovery
- no already completed compensation action is replayed as new work

---

## Notes

- This is the durability companion to the main compensation happy-path UAT.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is automatable with a restart-capable harness and deterministic compensation fixtures.

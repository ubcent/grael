# UAT-C1-02 Corrupt WAL Tail

## Metadata

- `UAT ID`: `UAT-C1-02`
- `Title`: Corrupt WAL tail does not invalidate valid prior history
- `Capability`: `C1`
- `Priority`: `core`
- `Related task(s)`: `C1` WAL scanning and recovery tasks
- `Depends on`: `C1`, `C9`

---

## Intent

This UAT proves that Grael can recover from a partially corrupted WAL tail without discarding valid earlier run history.

If this UAT passes, an operator can trust that:

- a damaged final event does not destroy the entire run
- committed valid history remains observable after restart
- recovery logic stops at the corruption boundary instead of inventing state

---

## Setup

- engine state: Grael server starts with a clean data directory
- worker state: one worker is running for activity type `step`
- workflow shape: linear workflow with enough progress to produce multiple WAL events
- test input: success path input
- clock/timer assumptions: no retries or deadlines required

Additional setup constraint:

- the test harness must be able to stop Grael, corrupt the tail of a WAL file, and restart Grael on the same storage

---

## Action

1. Start Grael.
2. Start one worker for activity type `step`.
3. Submit a workflow and allow it to produce multiple committed events.
4. Stop Grael.
5. Corrupt only the tail of the corresponding WAL file so that the final event is no longer valid.
6. Restart Grael using the same data directory.
7. Query `GetRun`.
8. Query `ListEvents`.

---

## Expected Visible Outcome

- Grael starts successfully despite the corrupted tail
- `GetRun` reflects the last valid committed state before corruption
- `ListEvents` contains all valid prior events and excludes the corrupted tail fragment
- Grael does not fabricate additional state beyond the last valid event

Checklist:

- expected API state: run remains readable after restart
- expected terminal outcome: not required; run may remain in-progress or recoverable at the last valid point
- expected event-family visibility: valid prefix of recorded history only
- expected behavior after restart: startup tolerates tail corruption and preserves valid progress

---

## Expected Event Sequence

1. Valid sequence of run events is written before shutdown
2. WAL tail is corrupted offline
3. Grael restarts
4. Only the valid event prefix is visible through `ListEvents`

---

## Failure Evidence

- Grael refuses to start because of a single corrupt tail event
- all history for the run becomes unreadable
- `ListEvents` includes malformed or partially decoded data
- `GetRun` reflects state that would require applying the corrupted tail

---

## Pass Criteria

- Grael starts and exposes the run after restart
- only valid prior WAL history is retained in visible state and event listing
- no state beyond the corruption boundary becomes visible

---

## Notes

- This scenario should use real WAL corruption, not an injected parser mock.
- It is acceptable if the run remains unfinished as long as the valid prefix is preserved correctly.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is automatable as long as the harness can manipulate WAL bytes between process runs.

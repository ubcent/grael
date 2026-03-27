# UAT-C9-05 StreamEvents Follows Committed History

## Metadata

- `UAT ID`: `UAT-C9-05`
- `Title`: `StreamEvents` replays committed history from `from_seq` and then streams live committed appends
- `Capability`: `C9`
- `Priority`: `late-v1`
- `Related task(s)`: `T41`, `T44`, `T45`
- `Depends on`: `C1`, `C9`

---

## Intent

This UAT proves that Grael can expose live execution progress without inventing a second source of truth.

If this UAT passes, an operator or UI client can trust that:

- the stream can start from an earlier committed sequence
- previously committed history is replayed in recorded order
- newly committed events are delivered live after the replay boundary
- streamed events stay aligned with `ListEvents`

---

## Setup

- engine state: Grael gRPC server is running locally
- worker state: workers are available for the chosen workflow
- workflow shape: workflow that produces multiple observable events over time
- test input: deterministic input
- clock/timer assumptions: execution is paced enough to observe both replay and live append behavior

---

## Action

1. Start Grael with the gRPC server enabled.
2. Start a workflow that will emit several events.
3. After at least one event is committed, open `StreamEvents` with `from_seq` earlier than the latest committed sequence.
4. Observe the replayed committed events delivered first.
5. Keep the stream open while the run continues.
6. Observe newly committed events delivered as the run progresses.
7. Compare the stream output with `ListEvents`.

---

## Expected Visible Outcome

- the stream first delivers committed history after `from_seq`
- event order matches committed WAL order
- the stream then delivers new committed events as they appear
- the union of replayed and live-streamed events matches `ListEvents`

Checklist:

- expected API state: live event observation available without polling-only reconstruction
- expected terminal outcome: not required, but full completion makes validation easier
- expected event-family visibility: start plus whatever node/timer/checkpoint events the chosen workflow emits
- expected behavior after restart, if relevant: not required for this baseline scenario

---

## Expected Event Sequence

The stream must show:

1. committed replay beginning strictly after `from_seq`
2. replayed events in the same order they appear in `ListEvents`
3. subsequent live committed events in sequence order

No event may appear before it is committed.

---

## Failure Evidence

- the stream emits events that are not yet visible in `ListEvents`
- replay order differs from committed order
- `from_seq` is ignored or replays the wrong boundary
- live delivery requires a projection-specific event source instead of committed history

---

## Pass Criteria

- `StreamEvents` remains a committed-history-following surface
- clients can use it for truthful live progress observation

---

## Notes

- This is the live-read companion to the forensic `ListEvents` UAT.

---

## Automation Readiness

- `Suitable for CI integration`

This should be automatable with a paced test workflow and a gRPC streaming client fixture.

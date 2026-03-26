# UAT-C9-02 ListEvents Causal History

## Metadata

- `UAT ID`: `UAT-C9-02`
- `Title`: `ListEvents` returns the raw execution history in causal order
- `Capability`: `C9`
- `Priority`: `core`
- `Related task(s)`: `C9` event history API tasks
- `Depends on`: `C1`, `C9`

---

## Intent

This UAT proves that `ListEvents` exposes the raw run history in a form that is useful for forensic inspection.

If this UAT passes, an operator can trust that:

- event history is externally available
- event ordering follows the persisted execution sequence
- the raw history is sufficient to understand what happened in the run

---

## Setup

- engine state: Grael server is running
- worker state: workers are running as needed
- workflow shape: workflow that produces a non-trivial event history, such as dispatch and completion of multiple nodes
- test input: normal success-path input
- clock/timer assumptions: no special constraints

---

## Action

1. Start Grael.
2. Start the necessary workers.
3. Submit a workflow that produces multiple observable events.
4. Allow the run to progress or complete.
5. Call `ListEvents`.

---

## Expected Visible Outcome

- `ListEvents` returns the raw sequence of events for the run
- the order of events matches the causal execution flow
- history is detailed enough to explain how the run reached its current or terminal state

Checklist:

- expected API state: raw event history available for the requested run
- expected terminal outcome: not required, but often easier to validate after completion
- expected event-family visibility: workflow start, node progress, and outcome events visible in sequence
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

The exact sequence depends on the workflow used, but it must preserve the causal order of recorded events and include the expected major transitions for the chosen scenario.

---

## Failure Evidence

- `ListEvents` omits major persisted events that shaped the run
- event order does not match actual execution order
- output is collapsed into a projection rather than exposing raw history

---

## Pass Criteria

- `ListEvents` returns the raw run history in recorded order
- the returned history is sufficient to reconstruct the visible execution story of the run

---

## Notes

- This is the forensic companion to the operational `GetRun` UAT.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is straightforward to automate with a deterministic workflow and expected event-family assertions.

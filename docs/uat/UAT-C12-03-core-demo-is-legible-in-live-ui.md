# UAT-C12-03 Core Demo Is Legible In Live UI

## Metadata

- `UAT ID`: `UAT-C12-03`
- `Title`: The flagship `core-demo` scenario is understandable through the live UI without reading raw event JSON
- `Capability`: `C12`
- `Priority`: `post-v1`
- `Related task(s)`: `T38`, `T40`
- `Depends on`: `C5`, `C8`, `C11`, `C12`

---

## Intent

This UAT proves that the Grael visual demo is not only correct, but communicative.

If this UAT passes, a new viewer can understand the main Grael story by watching the UI:

- the graph grows
- one node retries
- one node pauses for approval
- the run eventually completes

---

## Setup

- engine state: Grael is running locally
- worker state: workers or demo worker path are available for `core-demo`
- workflow shape: `core-demo`
- test input: deterministic input that triggers the intended path
- clock/timer assumptions: local demo timing

Checklist:

- engine state: one local runnable demo
- worker state: active
- workflow shape: `core-demo`
- test input: deterministic
- clock/timer assumptions: short enough to watch end to end

---

## Action

1. Start the visual demo application.
2. Start the `core-demo` workflow.
3. Watch the graph and event panel through run completion.
4. Observe the retry, checkpoint approval, and final completion phases.

---

## Expected Visible Outcome

- the graph and event tape make the run understandable without opening raw JSON
- retry is visibly distinguishable from permanent failure
- approval waiting is visibly distinguishable from generic stall
- final completion is clearly visible as the terminal run outcome

Checklist:

- expected API state: the UI remains aligned with current read surfaces
- expected terminal outcome: successful completion
- expected event-family visibility: retry, checkpoint, approval, and completion are visibly represented
- expected behavior after restart, if relevant: not required for this baseline UAT

---

## Expected Event Sequence

The exact sequence depends on the current `core-demo`, but the visible UI story must include:

1. run start
2. dynamic spawn
3. retryable failure and later recovery
4. checkpoint reached
5. checkpoint approved
6. successful workflow completion

---

## Failure Evidence

- the demo requires reading raw event JSON to understand what happened
- retry looks the same as permanent failure
- approval waiting looks like a frozen run
- successful completion is not clearly visible

---

## Pass Criteria

- a new viewer can explain the `core-demo` run from the UI alone
- the live UI presents the core Grael story clearly and truthfully

---

## Notes

- This is the product-legibility UAT for the post-v1 visual layer.

---

## Automation Readiness

- `Manual only`

This should begin as a manual product-acceptance scenario even if parts of the data flow are automated.

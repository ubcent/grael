# UAT-C12-02 Refresh And Restart Preserve Demo Truth

## Metadata

- `UAT ID`: `UAT-C12-02`
- `Title`: Refreshing the demo or reconnecting after restart preserves the truthful story of committed progress
- `Capability`: `C13`
- `Priority`: `post-v1`
- `Related task(s)`: `T36`, `T39`
- `Depends on`: `C1`, `C9`, `C11`, `C13`

---

## Intent

This UAT proves that the visual demo is derived from committed Grael state rather than fragile browser-local or process-local state.

If this UAT passes, a viewer can trust that refresh and reconnect do not rewrite history or hide committed progress.

---

## Setup

- engine state: Grael is running with restart capability available
- worker state: workers are running for the `core-demo`
- workflow shape: `core-demo`
- test input: deterministic input that causes retry and approval progress before terminal completion
- clock/timer assumptions: short enough to observe in one local run

Checklist:

- engine state: restart-capable local harness
- worker state: active
- workflow shape: includes spawn, retry, and approval
- test input: deterministic
- clock/timer assumptions: short demo windows

---

## Action

1. Start the visual demo application.
2. Submit the `core-demo` workflow.
3. Let the run make visible progress.
4. Refresh the demo page or reconnect the demo client.
5. Optionally restart Grael while the run still has committed progress.
6. Reopen the same run in the demo.

---

## Expected Visible Outcome

- the demo reconstructs the graph and event story from committed progress
- visible node states after refresh or reconnect match the real run state
- the event tape does not lose already committed causal history
- resumed execution remains understandable after reconnect

Checklist:

- expected API state: the refreshed demo agrees with `GetRun` and `ListEvents`
- expected terminal outcome: not required
- expected event-family visibility: previously committed events remain visible
- expected behavior after restart, if relevant: the same run remains legible after reconnect

---

## Expected Event Sequence

The exact sequence depends on the run, but the visible demo after refresh must remain compatible with:

1. already committed events before refresh or restart
2. resumed event progression after reconnect

---

## Failure Evidence

- refresh loses nodes or events that were already committed
- reconnect shows a contradictory graph
- the demo depends on local transient state to explain what happened
- restart requires a fresh run to make the demo usable again

---

## Pass Criteria

- the demo remains truthful after refresh or reconnect
- the visible story is reconstructable from committed engine state
- no browser-local or process-local state is required for correctness

---

## Notes

- This is the key trust test for the demo adapter.

---

## Automation Readiness

- `Scriptable with local harness`

This can be exercised through deterministic refresh/reconnect steps in a local harness.

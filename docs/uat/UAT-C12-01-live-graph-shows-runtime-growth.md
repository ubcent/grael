# UAT-C12-01 Live Graph Shows Runtime Growth

## Metadata

- `UAT ID`: `UAT-C12-01`
- `Title`: The live demo graph shows nodes appearing and changing state as the run progresses
- `Capability`: `C12`
- `Priority`: `post-v1`
- `Related task(s)`: `T37`
- `Depends on`: `C6`, `C9`, `C11`, `C12`

---

## Intent

This UAT proves that Grael's visual demo makes the living-graph behavior visible rather than leaving it hidden in event logs.

If this UAT passes, a viewer can trust that the UI is showing real runtime graph growth and not a pre-baked animation.

---

## Setup

- engine state: Grael is running with the flagship `core-demo` scenario available
- worker state: the demo worker path or equivalent worker fixtures are running
- workflow shape: `core-demo` or another dynamic-spawn workflow
- test input: deterministic input that causes runtime spawn
- clock/timer assumptions: normal local demo timing

Checklist:

- engine state: single run available
- worker state: active for required activity types
- workflow shape: one discovery node that spawns additional work
- test input: deterministic
- clock/timer assumptions: short enough to observe visibly

---

## Action

1. Start the visual demo application.
2. Start or connect to Grael.
3. Submit the `core-demo` workflow.
4. Watch the graph during discovery and spawned-node execution.
5. Refresh the run view if needed during execution.

---

## Expected Visible Outcome

- the graph starts small and grows when the discovery step spawns new nodes
- spawned nodes appear with their real dependency links
- node states visibly change as work progresses
- the visible graph remains aligned with current `GetRun` state

Checklist:

- expected API state: `GetRun` and visible graph agree on current nodes and states
- expected terminal outcome: not required for this UAT
- expected event-family visibility: spawn-related progress is visible
- expected behavior after restart, if relevant: not required for this UAT

---

## Expected Event Sequence

The exact sequence depends on the workflow, but the visible graph change must correspond to:

1. `WorkflowStarted`
2. `NodeCompleted` for the discovery node
3. spawned nodes becoming visible in the live graph

---

## Failure Evidence

- the graph is fully pre-rendered before spawn happens
- spawned nodes never appear
- visible node states disagree with `GetRun`
- refresh replaces the graph with contradictory information

---

## Pass Criteria

- graph growth is visibly observable during run execution
- the graph matches the real run state
- no contradictory or fabricated graph changes are shown

---

## Notes

- This can start as a local harness UAT before becoming a richer UI test.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario can be exercised locally with a deterministic workflow and read-surface polling.

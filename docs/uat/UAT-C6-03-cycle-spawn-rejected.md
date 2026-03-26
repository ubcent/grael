# UAT-C6-03 Cycle Spawn Rejected

## Metadata

- `UAT ID`: `UAT-C6-03`
- `Title`: Invalid spawn that creates a cycle is rejected cleanly
- `Capability`: `C6`
- `Priority`: `core`
- `Related task(s)`: `C6` cycle detection and spawn validation tasks
- `Depends on`: `C2`, `C4`, `C6`, `C9`

---

## Intent

This UAT proves that Grael protects run integrity by rejecting runtime graph mutations that would create a cycle.

If this UAT passes, an operator can trust that:

- invalid dynamic graph mutations are not silently accepted
- cycle detection happens before the run enters unrecoverable graph state
- graph safety remains enforced even for worker-driven spawn behavior

---

## Setup

- engine state: Grael server is running
- worker state: one worker is running for activity type `discover`
- workflow shape: initial workflow where a spawn attempt can reference dependencies in a way that would introduce a cycle
- test input: worker fixture returns spawned node definitions that would create a graph cycle
- clock/timer assumptions: no retry or restart behavior required

---

## Action

1. Start Grael.
2. Start a worker registered for activity type `discover`.
3. Submit a workflow with a node that is allowed to spawn follow-up work.
4. Have the worker return a completion payload whose spawned nodes would introduce a cycle.
5. Query `GetRun`.
6. Query `ListEvents`.

---

## Expected Visible Outcome

- the invalid spawn is not accepted as normal graph growth
- the cycle-creating nodes do not appear as active valid nodes in `GetRun`
- the run surfaces a clear failure or rejection path instead of entering inconsistent execution

Checklist:

- expected API state: run remains in a coherent state after spawn rejection
- expected terminal outcome: failure or rejection path is acceptable, silent corruption is not
- expected event-family visibility: attempted parent completion plus visible rejection consequence
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

1. `WorkflowStarted`
2. `LeaseGranted`
3. `NodeStarted`
4. Worker returns completion payload containing invalid cycle spawn
5. Visible rejection or failure handling occurs
6. No valid cyclical graph becomes active

---

## Failure Evidence

- cycle-producing spawned nodes become active in `GetRun`
- the run proceeds with a graph that contains a dependency cycle
- restart or later execution reveals graph inconsistency caused by the accepted cycle

---

## Pass Criteria

- cycle-creating spawn is rejected before entering active graph state
- visible state remains coherent
- no cyclical execution graph is accepted into the run

---

## Notes

- The exact rejection representation may vary, but acceptance of the cycle is not allowed.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is automatable with a deterministic worker fixture that emits a known-invalid spawn shape.

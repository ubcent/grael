# UAT-C10-01 Definition Metadata Executes As Declared

## Metadata

- `UAT ID`: `UAT-C10-01`
- `Title`: A workflow definition with retry, deadline, checkpoint, and compensation metadata executes as declared
- `Capability`: `C10`
- `Priority`: `late-v1`
- `Related task(s)`: `C10` workflow definition contract tasks
- `Depends on`: `C5`, `C7`, `C8`, `C10`

---

## Intent

This UAT proves that the workflow definition contract is expressive enough to drive the major v1 control-flow behaviors without additional hidden configuration.

If this UAT passes, an operator can trust that:

- retry, deadline, checkpoint, and compensation behavior can be declared through workflow metadata
- Grael executes those declared policies consistently
- the definition contract is sufficient for real v1 workflows

---

## Setup

- engine state: Grael server is running
- worker state: workers are running for normal, checkpointing, and compensation activities
- workflow shape: one workflow definition containing nodes that exercise retry, deadline, checkpoint, and compensation semantics
- test input: crafted so each declared behavior becomes observable during execution
- clock/timer assumptions: timeouts and deadlines are short enough for test runtime

---

## Action

1. Start Grael.
2. Start workers for all activity types referenced by the workflow definition.
3. Submit the workflow definition and input.
4. Exercise the declared retry path.
5. Exercise the declared checkpoint path.
6. Exercise at least one declared deadline-bound path.
7. Exercise a permanent failure path that triggers declared compensation.
8. Query `GetRun` and `ListEvents` throughout execution.

---

## Expected Visible Outcome

- each major declared behavior becomes externally observable when triggered
- Grael does not require out-of-band configuration for those behaviors to take effect
- read APIs show outcomes aligned with the workflow's declared metadata

Checklist:

- expected API state: visible state transitions corresponding to declared behaviors
- expected terminal outcome: depends on the exact chosen scenario, but must be consistent with declaration-driven control flow
- expected event-family visibility: retry, checkpoint, deadline, and compensation families appear when triggered
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

The exact sequence depends on the chosen composite workflow, but the run must visibly exercise the event families corresponding to the declared retry, checkpoint, deadline, and compensation metadata.

---

## Failure Evidence

- declared metadata is ignored unless separate hidden config is added
- one or more declared behaviors cannot be triggered through the workflow definition alone
- observed behavior conflicts with declared policy

---

## Pass Criteria

- workflow metadata alone is sufficient to drive the major v1 control-flow behaviors
- externally visible outcomes align with the declared workflow definition

---

## Notes

- This UAT is intentionally authoring-contract focused, not implementation-structure focused.

---

## Automation Readiness

- `Scriptable with local harness`

This scenario is automatable but likely best as a composed acceptance test rather than an early CI smoke test.

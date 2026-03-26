# UAT-C9-03 StartRun Capacity Reject

## Metadata

- `UAT ID`: `UAT-C9-03`
- `Title`: `StartRun` rejects immediately when the engine is at hard capacity
- `Capability`: `C9`
- `Priority`: `core`
- `Related task(s)`: `C9` start API and capacity-limit tasks
- `Depends on`: `C9`

---

## Intent

This UAT proves the v1 no-admission-queue contract: when Grael is at hard capacity, `StartRun` rejects immediately instead of silently queueing the run.

If this UAT passes, an operator can trust that:

- capacity exhaustion is explicit
- no hidden admission queue exists in v1
- clients receive an immediate answer on run submission

---

## Setup

- engine state: Grael server is running with a small hard capacity limit configured
- worker state: not essential beyond keeping the engine in the full-capacity condition
- workflow shape: enough submitted or active runs to reach configured hard capacity
- test input: one additional run submission beyond capacity
- clock/timer assumptions: no special timing constraints

---

## Action

1. Start Grael with a small hard capacity configuration.
2. Fill the engine to its configured capacity using active runs.
3. Submit one additional `StartRun` request.
4. Observe the API response.
5. Optionally query `GetRun` or run listings if available to confirm no extra run was admitted.

---

## Expected Visible Outcome

- the additional `StartRun` request returns an immediate rejection
- no hidden queued run is created
- the system does not behave as if the request succeeded and will run later automatically

Checklist:

- expected API state: explicit rejection at submission time
- expected terminal outcome: no run is created for the rejected request
- expected event-family visibility: not required if the run never starts
- expected behavior after restart, if relevant: not required for this scenario

---

## Expected Event Sequence

No `WorkflowStarted` event should be created for the rejected submission.

---

## Failure Evidence

- request appears accepted but is only silently queued
- a new run is created despite hard capacity being exceeded
- delayed execution occurs for the rejected request without a fresh submission

---

## Pass Criteria

- `StartRun` fails immediately at hard capacity
- no run is created or queued for the rejected request
- behavior matches the explicit no-admission-queue v1 contract

---

## Notes

- This scenario should verify the product contract rather than internal backpressure mechanisms.

---

## Automation Readiness

- `Suitable for CI integration`

This scenario is automatable if the test harness can fill capacity deterministically.

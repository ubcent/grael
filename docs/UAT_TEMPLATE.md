# Grael UAT Template

Use this template for acceptance scenarios tied to a specific v1 capability or backlog item.

The goal is to define completion in terms of externally observable behavior, not internal implementation details.

---

## Metadata

- `UAT ID`:
- `Title`:
- `Capability`: `C1` - `C11`
- `Priority`: `core` | `late-v1`
- `Related task(s)`:
- `Depends on`:

---

## Intent

Describe the user-visible behavior this UAT proves.

Questions this section should answer:

- what capability is being validated
- why it matters for Grael v1
- what someone should be able to trust if this UAT passes

---

## Setup

Describe the environment and fixtures required before the scenario starts.

Include only observable prerequisites such as:

- Grael server running
- worker process running
- workflow definition loaded
- run input prepared
- storage directory initialized
- restart/crash test harness available

Checklist:

- engine state:
- worker state:
- workflow shape:
- test input:
- clock/timer assumptions:

---

## Action

Describe the steps the operator, client, or worker performs.

Write these as explicit ordered actions.

Example shape:

1. Start Grael with empty data directory.
2. Register worker for activity type `hello`.
3. Submit workflow `single_hello`.
4. Wait for worker to receive a task.
5. Complete the task successfully.

---

## Expected Visible Outcome

Describe what must be externally visible if the scenario passes.

Use only observable surfaces:

- API responses
- worker RPC outcomes
- `GetRun`
- `ListEvents`
- process restart behavior

Checklist:

- expected API state:
- expected terminal outcome:
- expected event-family visibility:
- expected behavior after restart, if relevant:

---

## Expected Event Sequence

List only the key events that must appear in order.

Do not over-specify fields that are irrelevant to the acceptance behavior.

Example:

1. `WorkflowStarted`
2. `LeaseGranted`
3. `NodeStarted`
4. `NodeCompleted`
5. `WorkflowCompleted`

---

## Failure Evidence

List what must not happen.

Examples:

- node is dispatched twice
- completed node becomes runnable again
- late `CompleteTask` is accepted after lease expiry
- restart loses previously committed progress

---

## Pass Criteria

State the precise rule for considering this UAT complete.

Recommended format:

- all expected visible outcomes are observed
- forbidden outcomes are not observed
- event sequence matches the intended causal flow

---

## Notes

Optional implementation or automation notes.

Examples:

- can start as manual UAT and later become an automated integration test
- requires deterministic worker fixture
- should run against a real restart, not a mocked restart

---

## Automation Readiness

- `Manual only`
- `Scriptable with local harness`
- `Suitable for CI integration`

Briefly explain the current status.

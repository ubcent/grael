# UAT-C10-04 SDK Fan-Out Helper Lowers To Spawn

- `Capability`: `C10`
- `Status`: `specified`
- `Title`: An SDK fan-out helper expands into ordinary spawned nodes without changing execution semantics

---

## Intent

Prove that SDK fan-out convenience does not create a second execution model and remains only a more ergonomic way to author living-DAG spawn.

---

## Setup

- Prepare one workflow whose worker uses an SDK fan-out helper to create several child tasks
- Prepare an equivalent workflow whose worker manually returns the same `spawned_nodes`
- Use the same worker behaviors and retry settings in both runs

---

## Steps

1. Start the helper-based run.
2. Start the manual-spawn run.
3. Let both runs execute to completion.
4. Inspect `GetRun` and `ListEvents` for both runs.
5. Compare the observed orchestration behavior.

---

## Expected Result

- both runs produce the same graph growth pattern
- both runs dispatch the same child work
- both runs obey the same retry and dependency semantics
- the helper-based path does not introduce special fan-out events or fan-out-only runtime states
- the difference is authoring convenience only

---

## Surfaces Exercised

- Go SDK
- `CompleteTask`
- `GetRun`
- `ListEvents`

---

## Failure Signals

- helper-based fan-out produces different runtime semantics than manual spawn
- helper-based fan-out introduces special event types or hidden coordination
- helper-based fan-out can do work that plain `spawned_nodes` cannot express

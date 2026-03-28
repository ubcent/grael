# UAT-C10-03 Node Input Reaches Worker

- `Capability`: `C10`
- `Status`: `specified`
- `Title`: A node with declared input reaches the worker with the same per-node payload across spawn and restart

---

## Intent

Prove that Grael can carry explicit node-scoped context as part of the workflow contract instead of forcing all task context into workflow-global input or an out-of-band convention.

---

## Setup

- Start a run whose workflow contains:
  - one static node with declared node input
  - one planner node that spawns a child node with its own declared input
- Use a worker that records the node input it receives for each task
- Restart the process after the spawned child is present but before all work completes

---

## Steps

1. Start the workflow.
2. Poll and run the static node that has explicit node input.
3. Confirm the worker receives both:
   - workflow-global input
   - node-specific input for that node
4. Let the planner node complete and spawn a child with explicit node input.
5. Restart Grael before the spawned child finishes.
6. Poll the spawned child after restart.
7. Confirm the worker receives the same declared node input for the spawned child.

---

## Expected Result

- static nodes can carry explicit node input
- spawned nodes can carry explicit node input
- node input is delivered through the normal task surface
- restart does not erase or mutate node-specific input
- no side channel is required to reconstruct task context

---

## Surfaces Exercised

- `StartRun`
- `CompleteTask`
- `PollTask`
- `GetRun`
- `ListEvents`

---

## Failure Signals

- worker receives only workflow-global input
- spawned child input disappears after restart
- node input differs before and after restart
- implementation relies on process-local memory rather than committed history

# Grael Post-v1 Task Backlog

This backlog starts after the completed Grael v1 baseline.

The goal is not to widen the core runtime contract immediately. The goal is to make the product more visible and demoable while preserving the trust guarantees already earned.

---

## T36. Read-only demo adapter over `GetRun` and `ListEvents`

- `Status`: `in_progress`
- `Capability`: `C13`
- `Goal`: Provide a small adapter that converts Grael read surfaces into a UI-friendly graph and timeline model.
- `Scope`:
  - read-only adapter layer
  - graph node/edge projection
  - event cursor support
  - polling refresh compatibility
- `Depends on`: `T30`, `T31`, `T35`
- `Definition of Done`:
  - [UAT-C12-02](docs/uat/UAT-C12-02-refresh-and-restart-preserve-demo-truth.md)

## T37. Live graph visualization of node states and graph growth

- `Status`: `in_progress`
- `Capability`: `C12`
- `Goal`: Render the run graph so dynamic spawn and state changes are visible as they happen.
- `Scope`:
  - graph canvas
  - node status rendering
  - dynamic node insertion
  - dependency edge rendering
- `Depends on`: `T36`
- `Definition of Done`:
  - [UAT-C12-01](docs/uat/UAT-C12-01-live-graph-shows-runtime-growth.md)

## T38. Event tape and run summary panel

- `Status`: `in_progress`
- `Capability`: `C12`
- `Goal`: Make the causal story visible beside the graph.
- `Scope`:
  - event list or timeline
  - run metadata header
  - current run state visibility
  - event-family highlighting for retry, checkpoint, and completion
- `Depends on`: `T36`
- `Definition of Done`:
  - [UAT-C12-03](docs/uat/UAT-C12-03-core-demo-is-legible-in-live-ui.md)

## T39. Replay-friendly visual progression for the flagship `core-demo`

- `Status`: `specified`
- `Capability`: `C13`
- `Goal`: Make the `core-demo` scenario understandable even if the page opens mid-run or after refresh.
- `Scope`:
  - event-cursor playback support
  - visual graph reconstruction from history
  - page refresh continuity
- `Depends on`: `T36`, `T37`, `T38`
- `Definition of Done`:
  - [UAT-C12-02](docs/uat/UAT-C12-02-refresh-and-restart-preserve-demo-truth.md)

## T40. Demo packaging and operator-facing docs

- `Status`: `specified`
- `Capability`: `C12`
- `Goal`: Make the visual demo runnable and explainable by someone who did not build it.
- `Scope`:
  - local demo start instructions
  - screenshot or walkthrough docs
  - one-command or near-one-command demo startup
- `Depends on`: `T37`, `T38`
- `Definition of Done`:
  - [UAT-C12-03](docs/uat/UAT-C12-03-core-demo-is-legible-in-live-ui.md)

---

## Suggested First Implementation Sequence

Recommended order for the first post-v1 demo cycle:

1. `T36`
2. `T37`
3. `T38`
4. `T39`
5. `T40`

This order prioritizes truth-preserving read adaptation before visual polish.

# Grael Sprint 6 Plan

This document defines the first post-v1 sprint for Grael.

Sprint 6 does not widen the core runtime contract. It turns the completed v1 engine into a live, visual product demo that makes Grael's differentiators obvious in real time.

---

## Sprint Goal

Build the first trustworthy visual demo for Grael that:

- shows the workflow graph as it grows and changes
- shows the causal event story beside the graph
- stays derived from `GetRun` and `ListEvents`
- makes the flagship `core-demo` scenario visible and understandable

At the end of Sprint 6, someone should be able to watch Grael work instead of only reading JSON and event logs.

---

## In Scope

The sprint includes the following tasks from `docs/POST_V1_TASK_BACKLOG.md`:

1. `T36` Read-only demo adapter over `GetRun` and `ListEvents`
2. `T37` Live graph visualization of node states and graph growth
3. `T38` Event tape and run summary panel

---

## Why This Slice

Grael now has a trustworthy runtime and a strong end-to-end demo flow, but the product story is still mostly legible to engineers who are willing to inspect event logs and JSON.

Sprint 6 turns that truth into a visual experience without weakening the guarantees that made the engine worth trusting in the first place.

This is the right next step because it:

- improves product comprehension immediately
- creates a far better internal and external demo story
- reuses the already-completed `core-demo` scenario
- does not require new correctness-sensitive runtime semantics

---

## Out of Scope

Do not pull these into Sprint 6:

- authoritative UI state outside the read surfaces
- demo-only runtime shortcuts
- control-plane write operations
- dashboard/platform multi-run management
- multi-user auth or hosting concerns
- mandatory WebSocket or SSE transport
- heavy analytics or secondary persistence

Sprint 6 should prove a trustworthy visual read layer, not start a platform.

---

## Expected End State

By the end of Sprint 6:

- a local demo app can visualize a single run
- the graph visibly grows when the workflow spawns new nodes
- node state changes are visible without reading raw event JSON
- an event tape shows the causal progression of the run
- the `core-demo` scenario is understandable through the UI

---

## Exit Criteria

Sprint 6 is complete when all of the following are true:

- the in-scope tasks are implemented to a usable baseline
- the demo remains read-only and derived from Grael read surfaces
- the `core-demo` scenario is visually understandable without engine-internal knowledge
- the following UATs can pass:
  - [UAT-C12-01-live-graph-shows-runtime-growth.md](docs/uat/UAT-C12-01-live-graph-shows-runtime-growth.md)
  - [UAT-C12-03-core-demo-is-legible-in-live-ui.md](docs/uat/UAT-C12-03-core-demo-is-legible-in-live-ui.md)

Current status:

- `T36` has started through a new read-only `demo/adapter` package
- the adapter already projects `GetRun` and `ListEvents` into a UI-friendly snapshot with graph nodes, edges, timeline events, and polling cursor/delta support
- a new `grael-demo` server now serves the first live UI with graph rendering, run summary cards, and event timeline panels
- the flagship `core-demo` now runs through an SDK-based local multi-worker harness instead of a bespoke in-process task loop
- the visual demo now shows a truthful morning incident briefing story with overlapping static prep work, mid-run spawn growth, retry recovery, approval waiting, and final publish
- spawned nodes can now legally depend on already-materialized static nodes, which lets dynamic work flow back into the remaining graph without demo-only shortcuts
- a dedicated operator-facing demo guide now exists in [docs/SPRINT_6_DEMO_RUNBOOK.md](docs/SPRINT_6_DEMO_RUNBOOK.md)
- `T37` and `T38` are now in progress through the stronger visual baseline above
- targeted adapter tests and `go test ./...` are green

Recommended closeout bar:

- run the manual visual pass from [docs/SPRINT_6_DEMO_RUNBOOK.md](docs/SPRINT_6_DEMO_RUNBOOK.md)
- confirm the UI still satisfies `UAT-C12-01`, `UAT-C12-02`, and `UAT-C12-03` in operator-facing terms
- do one final walkthrough using the default `showcase` pacing profile

---

## Stretch Goal

If Sprint 6 finishes early, the best stretch target is:

- start `T39` with basic replay or event-step viewing for the `core-demo`

Why:

- it makes the demo more teachable
- it stays inside the same read-only visual surface
- it does not require new runtime semantics

---

## Risks

The main Sprint 6 risks are:

- building a pretty UI that cannot be trusted
- overcomplicating transport before the basic visual story works
- coupling the demo too tightly to engine internals instead of read surfaces
- spending too much time polishing visuals before the graph story is legible

---

## Review Questions

At the midpoint and end of the sprint, the team should ask:

1. Is the demo still honestly derived from `GetRun` and `ListEvents`?
2. Can a new viewer understand spawn, retry, approval, and completion by watching the UI?
3. Would the demo still tell the truth after refresh or restart?
4. Are we building a visual explanation layer, or accidentally starting a control plane?

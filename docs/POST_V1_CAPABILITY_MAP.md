# Grael Post-v1 Capability Map

This document defines the first planning slice after the completed Grael v1 baseline.

Its goal is to extend the product in ways that make Grael more legible and compelling without weakening the runtime guarantees established in v1.

---

## Capability Overview

| ID | Capability | Goal |
|---|---|---|
| C12 | Visual Live Demo Surface | Make Grael execution visible as a live graph and event story |
| C13 | Demo Read Adapter and Replay UX | Translate read APIs into a UI-friendly timeline and graph model |

---

## C12. Visual Live Demo Surface

### Goal

Show Grael runs as a live, trustworthy visual experience where operators can see graph growth, node-state transitions, and event causality in real time.

### Scope

- local demo app in the same repository
- graph rendering for workflow nodes and dependencies
- visible node state transitions
- event tape or timeline beside the graph
- support for the flagship `core-demo` scenario
- run summary header with workflow and run metadata

### Acceptance Shape

- graph visibly grows when runtime spawn occurs
- retry, approval, and completion are legible from the UI
- the UI remains consistent with `GetRun` and `ListEvents`

### Key Risks

- inventing hidden UI state that disagrees with event history
- prioritizing animation over correctness
- coupling demo rendering too tightly to engine internals

---

## C13. Demo Read Adapter and Replay UX

### Goal

Provide the smallest honest translation layer that turns Grael read APIs into a demo-friendly model without creating a second source of truth.

### Scope

- read-only adapter over `GetRun` and `ListEvents`
- polling-based refresh
- graph-diff derivation from event history
- event cursoring or replay position support
- restart-tolerant page refresh behavior

### Acceptance Shape

- refreshing the UI does not lose the visible story of committed progress
- the adapter model can be re-derived from current read surfaces
- replay mode or event stepping is possible without new runtime semantics

### Key Risks

- building a projection that becomes authoritative
- over-designing transport before proving the demo path
- mixing demo UX logic with engine correctness logic

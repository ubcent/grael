# Grael Sprint 7.1 Plan

This document defines a small v1 follow-up slice after Sprint 7 transport completion.

Sprint 7.1 does not add a new execution model. It closes two authoring-contract gaps that became visible once the gRPC surface was frozen:

- workers need durable per-node input, not only workflow-global input
- the SDK needs a fan-out convenience that still lowers to ordinary spawned nodes

---

## Sprint Goal

Complete the v1 authoring and SDK seam so that:

- a node can carry its own explicit input payload
- workers receive that node input through the normal task contract
- spawned nodes can carry their own input payloads just like static nodes
- SDK fan-out remains a convenience over living-DAG spawn, not a new runtime primitive

---

## In Scope

The sprint includes the following v1 backlog tasks:

1. `T46` Node-scoped input in workflow definitions and worker tasks
2. `T47` SDK fan-out helper that lowers to ordinary spawn semantics

---

## Guardrails

Sprint 7.1 must preserve all of the following:

- node input is part of the declared workflow graph, not a hidden side channel
- node input is durable through existing event history and restart behavior
- the runtime still has one dynamic-graph mechanism: spawned nodes
- the SDK helper only lowers to `spawned_nodes`
- no separate fan-out event family, fan-out state machine, or failure policy is introduced

---

## Out of Scope

Do not pull these into Sprint 7.1:

- map-reduce runtime primitives
- dedicated fan-out failure policies
- per-item compensation tracking beyond ordinary spawned-node semantics
- dependency output aggregation redesign
- memory/profile injection beyond explicit workflow or node input fields

---

## Expected End State

By the end of Sprint 7.1:

- `NodeDefinition` can declare `input`
- `WorkerTask` includes both workflow-level input and node-level input
- spawned nodes can carry input explicitly
- gRPC transport exposes the same contract honestly
- the SDK can help author a fan-out expansion without adding new runtime semantics

---

## Exit Criteria

Sprint 7.1 is complete when all of the following are true:

- the in-scope tasks are implemented
- node-scoped input survives spawn and restart
- the SDK helper expands to ordinary `spawned_nodes`
- the following UATs can pass:
  - [UAT-C10-03-node-input-reaches-worker.md](docs/uat/UAT-C10-03-node-input-reaches-worker.md)
  - [UAT-C10-04-sdk-fanout-helper-lowers-to-spawn.md](docs/uat/UAT-C10-04-sdk-fanout-helper-lowers-to-spawn.md)

---

## Review Questions

At the midpoint and end of the sprint, the team should ask:

1. Is node input fully explainable from the workflow definition plus committed history?
2. Would a restart deliver the same node-specific task context again?
3. Does the SDK helper only produce plain spawned nodes?
4. Have we avoided inventing a second fan-out execution model?

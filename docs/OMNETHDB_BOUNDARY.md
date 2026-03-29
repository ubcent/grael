# Grael And OmnethDB Boundary

This document defines the active boundary between Grael and `OmnethDB`.

Use it when a feature touches workflow context, retrieval, knowledge recall, or any question that sounds like "should Grael own this, or should OmnethDB?"

---

## Product Split

`Grael` owns:

- durable workflow execution
- WAL, snapshots, replay, and restart safety
- worker leasing, retries, timers, checkpoints, cancellation, and compensation
- dynamic graph growth through spawned nodes
- explicit workflow input and node input transport

`OmnethDB` owns:

- memory storage and retrieval
- profile assembly
- embeddings and vector/text search
- relationship scoring
- freshness/refresh behavior for retrieved knowledge
- any persistent memory-specific indexing or retrieval policy

---

## Allowed Integration Shapes

The allowed Grael ↔ OmnethDB integration shapes are:

1. caller fetches from OmnethDB before `StartRun` and injects the needed context into workflow input
2. an upstream worker fetches from OmnethDB and injects the needed context into spawned node input
3. a running worker queries OmnethDB directly during task execution
4. an external API/service composes both products without putting OmnethDB semantics inside Grael

---

## Forbidden Integration Shapes

The following are out of scope for Grael:

- a Grael-owned memory store
- Grael runtime code calling OmnethDB from scheduler/state/rehydration logic
- Grael-owned memory refresh policies
- hidden memory/profile injection that is not visible in workflow input or node input
- event types whose primary purpose is maintaining OmnethDB state

---

## Design Rule

If a behavior changes orchestration truth, it belongs in Grael.

If a behavior changes what knowledge is retrieved, scored, remembered, refreshed, or searched, it belongs in OmnethDB.

If a feature touches both, the seam must stay explicit at the API/worker boundary.

---

## Reading Order

For current planning:

1. [docs/adr/0015-memory-layer-belongs-to-omnethdb-not-grael.md](docs/adr/0015-memory-layer-belongs-to-omnethdb-not-grael.md)
2. [docs/V1_CANONICAL_BASELINE.md](docs/V1_CANONICAL_BASELINE.md)
3. this document

For historical background only:

- [docs/archive/README.md](docs/archive/README.md)
- memory-related sections inside the older architecture/spec documents

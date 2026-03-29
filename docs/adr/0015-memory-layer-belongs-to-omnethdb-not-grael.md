# ADR 0015: Memory Layer Belongs To OmnethDB, Not Grael

- `Status`: `Accepted`
- `Date`: `2026-03-29`

---

## Context

Earlier Grael documents mixed two different products into one conceptual system:

- a durable workflow/orchestration engine
- a memory and knowledge layer with profile assembly, recall, embeddings, and refresh semantics

That second system is now being split out and named `OmnethDB`.

Without an explicit decision, old specs continue to blur the product boundary and create scope confusion in planning, implementation, and review.

---

## Decision

Grael does not own a memory layer.

The memory/knowledge product now lives separately as `OmnethDB`.

Within Grael:

1. memory retrieval, storage, embeddings, search, profile assembly, and refresh policy are out of scope
2. Grael may carry caller-supplied context through workflow input and node input
3. workers and external services may query OmnethDB directly outside the Grael runtime
4. any Grael documentation that still describes an internal memory subsystem must be treated as historical and superseded

---

## Consequences

### Positive

- Grael scope becomes much sharper
- roadmap discussions stop smuggling OmnethDB work into engine planning
- integration between the two products can stay explicit and honest

### Negative

- older architecture/spec documents become partially historical
- some long-term product differentiation now lives across two products instead of one
- integration ergonomics must be designed intentionally instead of assumed

---

## Guardrails

Any future work in this repository must preserve all of the following:

- no Grael-owned memory store
- no Grael-owned memory refresh policy
- no hidden OmnethDB coupling inside scheduler/runtime logic
- OmnethDB integration must happen through explicit external calls or explicit workflow/node input

---

## Supersedes

This ADR does not supersede a numbered ADR directly.

It narrows how memory-related material in older architecture documents should be interpreted and formalizes the product split already reflected in the v1 scope cuts.

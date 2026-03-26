# Architecture Decision Records

This directory contains Architecture Decision Records for Grael.

ADR exists to capture important technical and product-architecture decisions in a form that is:

- explicit
- reviewable
- historically traceable
- easy to revisit when assumptions change

These records are not general notes.
They are the written decisions that shape the system.

---

## Why ADRs Exist Here

Grael is a correctness-sensitive system.

Important decisions in this repository often affect:

- determinism
- recovery behavior
- worker protocol semantics
- API honesty
- scope boundaries
- restart guarantees
- operator trust

Those decisions should not live only in:

- chat history
- memory
- commit messages
- undocumented assumptions inside code

If a decision materially changes how the system behaves or how the team is allowed to build it, it should be written down as an ADR.

---

## What Deserves An ADR

Write an ADR when you decide any of the following:

- a source-of-truth rule
- a runtime invariant
- a new durable contract
- a worker/API semantic choice
- a restart/recovery behavior
- a major scope cut or scope inclusion
- a package/layer ownership boundary
- a tradeoff that intentionally rejects an alternative

Examples:

- why scheduler logic must stay pure
- why time enters only through persisted timer events
- why `TIMED_OUT` is or is not a separate state in v1
- why v1 uses activity type strings instead of capability version ranges
- why `StartRun` rejects instead of using admission queues

---

## What Does Not Need An ADR

Do not write an ADR for:

- trivial refactors
- obvious local naming decisions
- small implementation details without architectural impact
- temporary notes that do not define a durable team decision

If the decision will not matter in 2 months, it probably does not need an ADR.

---

## ADR Naming

Use this format:

- `NNNN-short-kebab-case-title.md`

Examples:

- `0001-v1-source-of-truth-order.md`
- `0002-scheduler-must-remain-pure.md`
- `0003-no-admission-queue-in-v1.md`

Numbering should be monotonic and never reused.

---

## ADR Status Values

Use one of:

- `Proposed`
- `Accepted`
- `Superseded`
- `Deprecated`

If an ADR is replaced, keep the old one and link both directions.

Never rewrite history by pretending an old decision never happened.

---

## ADR Writing Rules

Every ADR should answer:

1. What is the decision?
2. Why are we making it?
3. What alternatives did we reject?
4. What are the consequences?
5. How does it affect determinism, durability, recovery, scope, or operator trust?

Good ADRs are:

- short
- explicit
- opinionated
- honest about tradeoffs

Bad ADRs are:

- vague
- purely descriptive
- afraid to reject alternatives
- so broad that they stop helping

---

## Relationship To Other Docs

Use ADRs alongside:

- [V1 canonical baseline](../V1_CANONICAL_BASELINE.md)
- [Capability map](../V1_CAPABILITY_MAP.md)
- [UAT matrix](../UAT_MATRIX.md)
- [Task backlog](../V1_TASK_BACKLOG.md)
- [Sprint plans](../SPRINT_1_PLAN.md)

Rule of thumb:

- baseline docs say what v1 is
- UAT docs say how we know behavior is correct
- backlog/sprint docs say what we plan to build
- ADRs say why important decisions were made

---

## Operating Rule

If an important architectural decision is being repeated verbally more than once, it probably needs an ADR.

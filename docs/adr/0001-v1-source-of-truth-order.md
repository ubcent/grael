# ADR 0001: V1 Source Of Truth Order

- `Status`: `Accepted`
- `Date`: `2026-03-26`
- `Deciders`: `Project maintainers`
- `Related docs`: `docs/V1_CANONICAL_BASELINE.md`, `docs/V1_CAPABILITY_MAP.md`, `docs/UAT_MATRIX.md`, `AGENTS.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

The repository contains multiple documents that describe Grael from different angles:

- broad architecture
- corrections
- runtime specification
- v1 scope definition
- capability planning
- UAT coverage
- sprint execution plans

Without an explicit document precedence rule, contributors can make incompatible decisions while each believing they are following "the spec."

For Grael, this is especially dangerous because the project is correctness-sensitive and scope-sensitive. A contributor can accidentally pull broad runtime-spec semantics into v1 implementation work even when the v1 documents intentionally cut that behavior.

We need one explicit source-of-truth order for planning and implementation.

---

## Decision

For Grael v1, the repository will use the following source-of-truth order:

1. `docs/V1_CANONICAL_BASELINE.md`
2. `docs/V1_CAPABILITY_MAP.md`
3. `docs/UAT_MATRIX.md`
4. `docs/uat/`
5. `docs/V1_TASK_BACKLOG.md`
6. `docs/V1_TASK_OPERATING_PLAN.md`
7. `docs/SPRINT_1_PLAN.md` through `docs/SPRINT_5_PLAN.md`
8. `docs/V1_SCOPE.md`
9. `docs/ARCHITECTURE_CORRECTIONS.md`
10. `docs/GRAEL_RUNTIME_SPEC.md`

If broad architecture or runtime-spec material conflicts with the explicit v1 planning documents, v1 planning documents win.

This rule applies to both humans and AI agents working in the repository.

---

## Alternatives Considered

### Use `docs/GRAEL_RUNTIME_SPEC.md` as the only source of truth

Rejected because it is broader than practical v1 scope and would pull cut features back into planning and implementation.

### Treat all docs as equal and resolve conflicts informally

Rejected because informal precedence is not real precedence. It creates drift, repeated debate, and silent scope expansion.

### Use only sprint plans as the source of truth

Rejected because sprint plans are execution slices, not the full definition of product boundaries and behavior contracts.

---

## Consequences

Benefits:

- contributors have a clear interpretation order
- v1 scope remains protected from architectural drift
- backlog, UAT, and sprint planning stay aligned

Tradeoffs:

- some older broad docs become secondary references instead of primary design inputs
- contributors must check precedence before citing a document in design discussions

---

## Guardrails

The following rules now follow from this decision:

- do not justify v1 scope expansion by citing lower-priority documents alone
- do not import runtime-spec behavior into implementation unless it is still compatible with the v1 baseline
- if a higher-priority document becomes wrong or incomplete, update it instead of bypassing it
- if a new architectural decision changes precedence or interpretation, write a new ADR

---

## Validation

This decision is holding if:

- backlog and sprint work continue to map cleanly to the v1 capability/UAT documents
- scope debates can be resolved by pointing to explicit precedence rather than memory or preference
- `AGENTS.md` and daily engineering behavior remain aligned with the same document order

---

## Notes

This ADR formalizes a rule that was already being used implicitly while building the v1 planning stack.

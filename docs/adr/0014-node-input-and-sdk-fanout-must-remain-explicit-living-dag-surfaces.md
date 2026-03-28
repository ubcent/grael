# ADR 0014: Node Input And SDK Fan-Out Must Remain Explicit Living-DAG Surfaces

- `Status`: `Accepted`
- `Date`: `2026-03-28`

---

## Context

After Sprint 7, the gRPC transport made two contract gaps visible:

1. workers receive only workflow-global input and cannot receive durable node-specific input
2. the SDK has no first-class convenience for common fan-out authoring, even though v1 already treats fan-out as spawned nodes

Both gaps are real, but the response must stay inside v1 guardrails.

We must not:

- invent a hidden side channel for per-node task context
- introduce a separate fan-out runtime primitive
- add a second orchestration model beside living-DAG spawn

---

## Decision

Grael v1 will address these gaps with two explicit surfaces:

1. `NodeDefinition` may carry explicit node input.
2. `WorkerTask` may carry both workflow-global input and node-scoped input.
3. spawned nodes may carry their own input in the same definition shape.
4. any SDK fan-out helper must lower entirely to ordinary `spawned_nodes`.

No new runtime event family, fan-out state machine, or fan-out failure policy is introduced by this decision.

---

## Rationale

This keeps both features honest:

- node-specific task context becomes part of the declared graph contract
- restart behavior remains believable because the input is reconstructable from persisted history
- SDK convenience does not mutate the engine into a second execution model
- transport and SDK layers stay thinner than the runtime itself

---

## Consequences

### Positive

- workers can receive explicit per-node context for static and spawned work
- spawn-based fan-out becomes easier to author without widening runtime semantics
- gRPC and in-process task delivery stay aligned

### Negative

- `NodeDefinition` and `WorkerTask` become slightly larger
- transport mapping must carry an additional structured payload
- the SDK must document clearly that its helper is only syntactic lowering

---

## Guardrails

Any implementation of this decision must preserve all of the following:

- node input is never authoritative outside declared workflow definitions and committed history
- replay and restart reproduce the same node input
- the SDK helper does not introduce special-case scheduler or worker behavior
- fan-out still means "complete a node with spawned children"

---

## Supersedes

This ADR does not supersede an earlier ADR. It refines the v1 authoring contract under [ADR 0013](./0013-grpc-transport-must-remain-a-thin-layer-over-api-service.md) and the v1 scope choice that fan-out is implemented through spawned nodes.

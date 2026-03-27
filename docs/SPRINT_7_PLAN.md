# Grael Sprint 7 Plan

This document defines the next v1 sprint after the current visual demo line.

Sprint 7 does not widen Grael into a platform. It completes the intended v1 network transport story by exposing the existing service surface over gRPC and adding a truthful committed-event stream.

It should be treated as a bounded continuation of the original v1 transport promise, not as a post-v1 platform expansion.

---

## Sprint Goal

Build the first honest network transport for Grael so that:

- remote workers can execute tasks over gRPC
- a separate API server can start and inspect runs over gRPC
- clients can subscribe to committed live event progress through `StreamEvents`
- all business logic still lives in the existing engine and service layers

At the end of Sprint 7, Grael should still have one execution model, but it should no longer require all callers to be in-process.

---

## Why This Slice

This is still v1 work because the original v1 product shape already assumed:

- workers connect over gRPC
- the engine can be embedded or served
- operators and integrators can interact through a small honest public surface

What is missing today is not runtime semantics. It is the intended transport packaging of those semantics.

This is the right next slice because it:

- unlocks TypeScript workers without reworking the engine
- unlocks a separate API-layer process without inventing a control plane
- improves the live demo and UI story through committed-event subscriptions
- stays within the existing deterministic and recovery guardrails

---

## In Scope

The sprint includes the following v1 backlog tasks:

1. `T41` v1 gRPC proto contract and code generation
2. `T42` Thin gRPC orchestration and inspection server over `api.Service`
3. `T43` Thin gRPC worker transport over the existing worker protocol
4. `T44` Committed event subscription and `StreamEvents`
5. `T45` `grael serve` command and remote integration acceptance slice

---

## Explicit Guardrails

Sprint 7 must preserve all of the following:

- `engine.Engine` remains the semantic center of execution
- `api.Service` remains the Go-facing business boundary
- transport code is mapping only
- `StreamEvents` follows committed history and does not invent a side-channel truth source
- auth, TLS, RBAC, and cloud-control-plane concerns remain out of scope
- worker and orchestration semantics do not change by transport

---

## Out of Scope

Do not pull these into Sprint 7:

- TLS or mTLS
- auth tokens beyond trivial local-only configuration needs
- multi-tenant server concerns
- dashboard write operations
- query-specific projection databases
- a TypeScript SDK layer
- protocol-level semantics that require runtime rewrites

Sprint 7 is a transport completion sprint, not a platform sprint.

---

## Expected End State

By the end of Sprint 7:

- `proto/grael.proto` exists and defines the intended network contract
- Grael can run as a local gRPC server through `grael serve`
- remote orchestration clients can call `StartRun`, `CancelRun`, `ApproveCheckpoint`, `GetRun`, and `ListEvents`
- remote workers can call `RegisterWorker`, `PollTask`, `CompleteTask`, `FailTask`, and `Heartbeat`
- `StreamEvents` can replay from `from_seq` and then stream live committed events
- integration coverage proves transport honesty end to end

---

## Exit Criteria

Sprint 7 is complete when all of the following are true:

- the in-scope tasks are implemented
- the gRPC layer is still visibly thinner than `api.Service`
- `StreamEvents` is derived from committed event history
- the following UATs can pass:
  - [UAT-C4-04-network-worker-over-grpc.md](docs/uat/UAT-C4-04-network-worker-over-grpc.md)
  - [UAT-C9-04-grpc-orchestration-surface.md](docs/uat/UAT-C9-04-grpc-orchestration-surface.md)
  - [UAT-C9-05-streamevents-follows-committed-history.md](docs/uat/UAT-C9-05-streamevents-follows-committed-history.md)

---

## Recommended Implementation Order

1. Freeze `proto/grael.proto`
2. Add code generation wiring
3. Implement thin orchestration/read server mapping
4. Implement thin worker transport mapping
5. Add committed event subscription and `StreamEvents`
6. Add `grael serve`
7. Add end-to-end transport acceptance tests

This order keeps the public contract explicit before transport logic starts to sprawl.

---

## Main Risks

The main Sprint 7 risks are:

- putting orchestration logic into the gRPC layer
- building a live stream that can diverge from committed history
- accidentally changing timeout or lease behavior while adding transport
- letting "quick local no-auth transport" turn into premature platform work

---

## Review Questions

At the midpoint and end of the sprint, the team should ask:

1. Can every transport behavior still be traced back to `api.Service` and `engine.Engine`?
2. Does `StreamEvents` only emit committed events in committed order?
3. Would remote workers observe the same lease and stale-result semantics as local callers?
4. Are we still building a transport layer, not a second runtime or control plane?

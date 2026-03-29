# ADR Index

This file tracks Architecture Decision Records for Grael.

---

## Records

| ADR | Title | Status | Notes |
|---|---|---|---|
| `0001` | [V1 Source Of Truth Order](./0001-v1-source-of-truth-order.md) | `Accepted` | Defines planning and implementation document precedence |
| `0002` | [Scheduler Must Remain Pure](./0002-scheduler-must-remain-pure.md) | `Accepted` | Protects deterministic orchestration boundaries |
| `0003` | [Time Enters Only Through Persisted Events](./0003-time-enters-only-through-persisted-events.md) | `Accepted` | Makes time-driven behavior durable and restart-safe |
| `0004` | [No Admission Queue In V1](./0004-no-admission-queue-in-v1.md) | `Accepted` | Freezes the immediate-reject overload contract |
| `0005` | [Activity Type Strings Instead Of Version Routing In V1](./0005-activity-type-strings-instead-of-version-routing.md) | `Accepted` | Keeps worker routing small and honest in v1 |
| `0006` | [Timed Out Is Not A Separate V1 Node State](./0006-timed-out-is-not-a-separate-v1-node-state.md) | `Accepted` | Keeps the v1 state machine smaller while preserving timeout semantics |
| `0007` | [GetRun And ListEvents Are The Only V1 Read Surfaces](./0007-getrun-and-listevents-are-the-only-v1-read-surfaces.md) | `Accepted` | Freezes the minimal read contract for v1 |
| `0008` | [Graceful Cancel Is The Only V1 Cancellation Mode](./0008-graceful-cancel-is-the-only-v1-cancellation-mode.md) | `Accepted` | Keeps cancellation semantics honest in v1 |
| `0009` | [Checkpoints Must Not Block Unrelated Work](./0009-checkpoints-must-not-block-unrelated-work.md) | `Accepted` | Protects selective approval semantics |
| `0010` | [Spawn Validation Must Happen Before Graph Mutation](./0010-spawn-validation-must-happen-before-graph-mutation.md) | `Accepted` | Protects the graph mutation boundary |
| `0011` | [Compensation Applies Only To Completed Nodes](./0011-compensation-applies-only-to-completed-nodes.md) | `Accepted` | Keeps unwind semantics conservative and trustworthy |
| `0012` | [Visual Demo Must Be A Read-Only Event-Derived Surface](./0012-visual-demo-must-be-a-read-only-event-derived-surface.md) | `Accepted` | Keeps the post-v1 demo layer honest and non-authoritative |
| `0013` | [gRPC Transport Must Remain A Thin Layer Over `api.Service`](./0013-grpc-transport-must-remain-a-thin-layer-over-api-service.md) | `Accepted` | Freezes the network transport boundary for the v1 service surface |
| `0014` | [Node Input And SDK Fan-Out Must Remain Explicit Living-DAG Surfaces](./0014-node-input-and-sdk-fanout-must-remain-explicit-living-dag-surfaces.md) | `Accepted` | Closes authoring gaps without adding a second runtime model |
| `0015` | [Memory Layer Belongs To OmnethDB, Not Grael](./0015-memory-layer-belongs-to-omnethdb-not-grael.md) | `Accepted` | Freezes the product boundary between the Grael engine and OmnethDB |

---

## Usage

When adding a new ADR:

1. Copy `ADR_TEMPLATE.md`
2. Assign the next available number
3. Add the record to this index
4. Link any superseding relationship explicitly

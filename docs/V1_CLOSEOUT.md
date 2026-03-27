# Grael v1 Closeout

This document records the practical closeout state for Grael v1.

Grael v1 should now be treated as the completed baseline defined by the v1 planning documents and acceptance matrix, not as an aspirational target.

---

## Release Position

`Grael v1` is a single-binary durable workflow engine for long-running, dynamic AI-agent execution.

The v1 product promise is now implemented at the committed scope:

- append-only WAL-backed orchestration state
- deterministic replay-derived execution state
- worker-driven execution through the public worker surface
- persisted retries, deadlines, leases, and checkpoint timers
- dynamic runtime node spawn with validation
- approval waiting without global run blocking
- graceful cancellation
- sequential compensation
- minimal read surface through `GetRun` and `ListEvents`

---

## What v1 Includes

The implemented v1 baseline includes:

- durable WAL append and replay
- snapshot plus WAL-delta recovery
- worker registration, polling, completion, failure, and heartbeat
- lease grant, lease expiry, and stale-result rejection
- retry backoff timers with restart catch-up
- execution deadline and absolute deadline handling
- dynamic DAG growth through runtime spawn
- cycle rejection and invalid spawn rejection
- checkpoint approval flow
- graceful cancellation
- restart-safe sequential compensation
- hard-capacity rejection on `StartRun`
- definition hash capture at run start
- minimal workflow definition contract for node policies
- thin Go worker SDK seam
- flagship composed demo covering spawn, retry, approval, restart, and successful completion

---

## What v1 Does Not Include

The following remain intentionally outside the v1 contract:

- memory-layer work
- sub-workflows
- admission queueing
- version-range routing
- error-handler branches
- multi-tenant platform concerns
- RBAC or mTLS platform surface
- broader control-plane UX

These are not gaps in v1 completion. They are explicit non-goals for this release line.

---

## Operator Guarantees

Grael v1 is intended to be trusted for:

- deterministic orchestration derived from persisted events
- restart-safe recovery without in-memory correctness dependencies
- visible failure semantics through event history
- rejection of stale worker outcomes after lease invalidation
- persisted timing behavior for retry, deadline, and approval timeout paths
- honest capacity behavior with immediate rejection instead of hidden queueing

Grael v1 does not claim:

- exactly-once task execution
- process-kill semantics for workers
- hidden recovery behavior outside the event log

---

## Acceptance Status

The committed v1 sprint scope is complete.

At closeout, the main committed late-v1 acceptance paths are covered:

- `UAT-C9-03` hard-capacity reject on `StartRun`
- `UAT-C10-01` workflow definition metadata executes as declared
- `UAT-C10-02` thin Go worker SDK seam
- `UAT-C11-01` flagship composed end-to-end demo

The lower-level runtime UATs from Sprints 1 through 4 are also represented in the current automated suite across:

- storage and replay
- worker execution
- leases and stale-result rejection
- retries and timers
- dynamic spawn
- checkpoint approval
- cancellation
- compensation

---

## Recommended Tag Readiness Checklist

Before tagging `v1.0.0`, the recommended final pass is:

1. Confirm `go test ./...` is green.
2. Confirm `docs/V1_TASK_BACKLOG.md` and sprint plans still match the implemented state.
3. Confirm the flagship demo example still runs from the current `README.md` instructions.
4. Confirm there are no unreviewed local changes unrelated to the v1 release.

If those are true, Grael is in a reasonable `v1.0.0` release-ready state.

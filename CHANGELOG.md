# Changelog

## v1.0.0

Initial Grael v1 release.

### Added

- Durable event-sourced workflow execution backed by an append-only WAL.
- Snapshot plus WAL-delta recovery for restart-safe run rehydration.
- Public worker execution surface with registration, polling, completion, failure, and heartbeat.
- Lease grant, lease expiry, and stale-result rejection for active task attempts.
- Persisted retry backoff, execution deadline, absolute deadline, and checkpoint timeout handling.
- Dynamic runtime graph expansion through validated node spawn.
- Checkpoint approval flow with restart-safe waiting semantics.
- Graceful cancellation with terminal cancelled outcomes.
- Sequential compensation with restart continuation.
- Hard-capacity rejection on `StartRun` with no hidden admission queue.
- Workflow definition hash capture at run start.
- Minimal workflow definition contract for node policies including retry, deadline, checkpoint, and compensation metadata.
- Thin Go worker SDK seam for small reference workers.
- Flagship composed demo workflow covering spawn, retry, approval, restart, and successful completion.

### Notes

- Grael v1 focuses on durability, determinism, recoverability, and operator trust.
- Memory systems, sub-workflows, admission queues, and broader platform concerns remain intentionally out of scope for v1.

# Grael v1 Release Announcement

Grael v1 is ready.

This release delivers the first complete Grael baseline: a durable workflow engine for long-running, dynamic AI-agent execution with deterministic recovery, visible event history, and honest failure semantics.

What makes v1 real:

- dynamic workflow graphs can grow at runtime
- retries, deadlines, approvals, cancellation, and compensation are persisted and replay-safe
- workers execute through a small public contract instead of hidden runtime shortcuts
- restart recovery preserves committed progress without relying on lucky in-memory state

The flagship v1 demo now shows the full product story in one run:

- discovery spawns new work
- one node fails retryably and recovers automatically
- one node pauses for approval while unrelated work continues
- the process can restart mid-execution
- the workflow still finishes successfully

Grael v1 is intentionally opinionated. It does not try to be a broad workflow platform yet. It focuses on the parts that matter most for trust:

- durability
- determinism
- recoverability
- correctness under failure
- operator comprehension

This is the point where Grael becomes not just implementable, but showable, explainable, and usable as a real v1 system.

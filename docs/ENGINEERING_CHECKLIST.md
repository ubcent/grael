# Grael Engineering Checklist

This is the daily-use companion to [AGENTS.md](/Users/dmitrybondarchuk/Projects/my/grael/AGENTS.md).

Use it before implementation, before review, and before merge.

If this checklist and the code disagree, the checklist wins until the relevant contract is explicitly changed.

---

## Before You Start

Confirm all of the following:

- I know which capability this change advances
- I know which task(s) from [docs/V1_TASK_BACKLOG.md](/Users/dmitrybondarchuk/Projects/my/grael/docs/V1_TASK_BACKLOG.md) this work maps to
- I know which UAT(s) define success for this work
- I am still inside `v1` scope
- I understand the restart semantics for the area I am touching

If any of these are unclear, stop and clarify before coding.

---

## Design Checklist

Before writing or approving implementation code, ask:

- Is all authoritative state still derivable from persisted events?
- Does scheduler logic remain pure?
- Is wall-clock time entering only through persisted timer/lease paths?
- Can this behavior be reconstructed after restart?
- Can stale worker results affect state incorrectly?
- Can invalid graph state enter the system before validation?
- Is the API contract honest about what the engine actually guarantees?

If any answer is "maybe", the design is not ready.

---

## Scope Checklist

Confirm that the change does not accidentally introduce:

- OmnethDB or Grael-owned memory-layer behavior
- sub-workflows
- admission queues
- error-handler branches
- version-range worker routing
- mTLS/RBAC/multi-tenant platform work
- force-cancel semantics beyond v1 contract

If it does, stop unless there is an explicit scope decision.

---

## Implementation Checklist

While coding, confirm:

- no hidden correctness-critical state is stored outside the WAL-derived model
- no in-memory-only timer behavior is relied on for correctness
- no terminal state can silently re-enter dispatch
- no recovery path depends on "the worker probably still knows"
- no projection or API response is being treated as primary state
- no convenience shortcut weakens determinism or restart safety

If you are tempted to add a shortcut "just for now" in leases, timers, recovery, spawn validation, or checkpoint handling, stop.

---

## Testing Checklist

Before calling the work done, confirm:

- local pure logic has unit coverage if appropriate
- component interaction has integration coverage if appropriate
- restart/crash behavior was considered explicitly
- the linked UATs can pass, or the gap is documented explicitly

For any runtime-significant change, ask:

- what happens if the process dies here?
- what happens if the worker disappears here?
- what happens if a stale result arrives here?
- what happens if replay starts from persisted history here?

If these are unanswered, the work is not done.

---

## Review Checklist

As a reviewer, check:

- the change has a clear capability/task/UAT mapping
- the change does not widen v1 scope
- deterministic behavior is preserved
- restart semantics are believable
- failure paths are specified, not implied
- the public contract is still honest
- the code did not get "cleaner" by becoming less correct

Most important review question:

`Will this still be correct when the process dies at the worst moment?`

---

## Pre-Merge Checklist

Before merge, confirm:

- the change is explainable in one paragraph without hand-waving
- the linked UATs are still the right acceptance targets
- no forbidden move from [AGENTS.md](/Users/dmitrybondarchuk/Projects/my/grael/AGENTS.md) was introduced
- there is no hidden dependency on future work for basic correctness
- the code leaves the repository more trustworthy, not just more featureful

If the merge increases surface area more than it increases trust, it is not ready.

---

## Fast Heuristics

Good sign:

- fewer hidden assumptions
- sharper ownership boundaries
- clearer failure semantics
- stronger restart story
- better auditability from `ListEvents`

Bad sign:

- more magic
- more "special cases"
- more caveats
- more behavior that only works if nothing crashes
- more state that exists nowhere in persisted history

---

## When Unsure

Choose the option that is:

- more deterministic
- more recoverable
- easier to audit
- easier to reject safely
- smaller in scope
- more honest

Trust is the product.

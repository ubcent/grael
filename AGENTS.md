# AGENTS.md

## Mission

We are building Grael to be the best workflow engine in the world for long-running, dynamic AI-agent execution.

Not "good enough."
Not "pretty solid."
Not "fine for v1."

The standard is:

- world-class product taste
- world-class engineering discipline
- world-class correctness under failure

Grael must feel inevitable once someone sees it work.

The product should create the reaction:

`"This is exactly what I wanted to exist."`

---

## Non-Negotiable Product Standard

Everything we build must strengthen all of these:

- durability
- determinism
- observability
- recoverability
- correctness under restart
- correctness under partial failure
- operator trust

If a change makes the system more featureful but less deterministic, less testable, or less auditable, it is a bad change.

We are not building a bag of workflow features.
We are building a system that engineers can trust when the process crashes at the worst possible moment.

---

## Source Of Truth

When working in this repository, use these documents in this order:

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

If there is tension between broad architecture and practical v1 scope, follow the v1 documents.

Do not silently invent scope.
Do not silently widen scope.
Do not silently "improve" the product by sneaking in post-v1 abstractions.

---

## Core Engineering Doctrine

### 1. Determinism First

Grael is an event-sourced orchestration engine.
Determinism is not a nice property.
It is the product.

Rules:

- all authoritative state must be derivable from persisted events
- rehydration is read-only
- scheduler logic must be pure
- time must enter the system only through persisted timer/lease events
- terminal state must remain terminal
- recovery must not require lucky in-memory residue

If a design introduces hidden state, implicit time, side-channel coordination, or behavior that cannot be reconstructed from persisted history, reject it.

### 2. Guardrails Over Cleverness

The system should aggressively prevent invalid states instead of "handling them later."

Rules:

- reject impossible transitions
- reject stale worker results
- reject invalid spawn shapes
- reject cycle-producing graph mutations
- reject behavior that bypasses persisted causal order
- reject APIs that pretend to guarantee more than the system can honestly guarantee

We prefer a loud rejection over a silent corruption path.

### 3. Small Honest Surfaces

Prefer the smallest honest API and behavior surface that can be trusted.

Rules:

- no fake abstractions
- no platform theater
- no broad interfaces without clear need
- no convenience behavior that weakens guarantees

If a feature requires caveats to explain why it is not really safe, it is not ready.

### 4. UAT Is The Contract

The UAT specs are not decoration.
They are the external definition of truth for v1 behavior.

Every meaningful change should clearly map to one or more UATs.
If the behavior cannot be expressed in observable acceptance terms, it is not well specified yet.

---

## Mandatory Guardrails

These rules apply to every human and every agent working in this repository.

### State Guardrails

- never introduce authoritative mutable state outside WAL-derived execution state
- never make projections primary state
- never let completed nodes become dispatchable again
- never let expired attempts become valid again
- never let invalid graph mutations enter active state

### Time Guardrails

- never read wall-clock time inside scheduler logic
- never couple retry or timeout behavior to process uptime
- never implement deadlines as in-memory-only behavior
- never let approval waiting bypass hard deadline semantics

### Recovery Guardrails

- restart behavior must be a first-class design concern, not a later patch
- every persistence boundary must be reasoned about under crash
- every control-flow feature must define restart semantics
- no recovery behavior may depend on "the worker probably remembers"

### API Guardrails

- do not claim exactly-once unless it is actually true
- do not claim process kill semantics where only lease invalidation exists
- do not hide failure modes from operators
- do not create silent queues where the product contract says immediate reject

### Scope Guardrails

- no memory-layer work in v1
- no sub-workflows in v1
- no admission queue in v1
- no error-handler branch in v1
- no version-range routing in v1
- no mTLS/RBAC/multi-tenant platform work in v1

If you touch any of the above without an explicit scope decision, you are working on the wrong thing.

---

## Forbidden Moves

The following moves are forbidden unless there is an explicit, documented decision to change the contract.

- introducing hidden mutable runtime state outside WAL-derived state
- adding scheduler behavior that depends on wall clock, goroutine timing, or external I/O
- accepting stale worker results after lease expiry
- writing code that mutates orchestration state without a persisted event
- implementing "temporary" in-memory timers for correctness-sensitive behavior
- allowing graph mutations before validation is complete
- making `GetRun` or any projection act as the primary source of truth
- adding broad abstractions "for future flexibility" without a present v1 need
- widening the public contract with semantics we cannot defend under restart or failure
- merging code that has no clear UAT impact for runtime-critical behavior

Also forbidden:

- vague retries
- vague cancellation semantics
- vague compensation semantics
- vague restart semantics
- vague claims of idempotency

If the exact behavior under restart, timeout, stale result, and crash cannot be stated clearly, the design is not ready.

---

## Required Change Shape

Any substantial change should be explainable in this shape:

1. which capability does it advance
2. which task(s) does it implement
3. which UAT(s) should pass because of it
4. which invariants or guardrails does it rely on
5. what failure mode it closes or what operator trust it improves

If you cannot answer those five things clearly, the change is under-specified.

---

## Definition Of Done

No runtime-significant change is done until all of the following are true:

1. the capability/task being advanced is explicitly identified
2. the behavior is still inside v1 scope
3. the change preserves deterministic and recovery guardrails
4. the relevant tests are added or updated
5. the relevant UATs can pass, or there is an explicit, temporary reason they cannot yet pass
6. restart/failure semantics have been considered, not deferred by omission
7. the resulting behavior is understandable through `GetRun`, `ListEvents`, or the documented public worker/API surfaces

For dangerous areas such as leases, timers, recovery, spawn validation, checkpoints, and compensation, "looks correct" is not enough.

Done means:

- behavior is specified
- behavior is implemented
- behavior is testable
- behavior is explainable

---

## Testing Standard

We do not trust a workflow engine because the code looks clean.
We trust it because the failure behavior is specified and exercised.

Minimum bar:

- unit tests for sharply local pure logic
- integration tests for component interaction
- restart/crash tests for durability-sensitive paths
- UAT alignment for externally visible behavior

Preferred priority:

1. correctness under restart
2. correctness under duplicate/lost/stale signals
3. correctness under timeout/lease expiry
4. correctness of happy path

Happy path alone proves almost nothing.

### Required Test Mindset

For every important feature, ask:

- what happens if the process dies here
- what happens if the worker disappears here
- what happens if a stale result arrives here
- what happens if we replay from persisted history here
- what happens if this event is the last valid event before corruption

If those questions are unanswered, the feature is not done.

---

## Code Review Checklist

Every substantive review should ask:

1. Does this change strengthen or weaken determinism?
2. Can the resulting behavior be reconstructed from persisted events alone?
3. Is any new state becoming authoritative outside the WAL-derived model?
4. Is wall-clock time entering the system only through approved persisted paths?
5. Are terminal states still terminal?
6. Can stale worker inputs or race paths corrupt state?
7. Is restart behavior defined and believable?
8. Is the API/behavior contract honest about what it guarantees?
9. Is the change still inside v1 scope?
10. Which UATs does this change satisfy, strengthen, or risk breaking?

Reviewers should be especially suspicious of:

- convenience shortcuts in runtime code
- hidden caches that affect correctness
- "just for now" code in lease/timer/recovery paths
- control-flow behavior split across too many layers
- changes that make demos easier but guarantees weaker

The review question is never only "does it work?"
The real question is:

`"Will this still be correct when the process dies at the worst moment?"`

---

## Simplicity Rules

We value simplicity, but only the kind that preserves truth.

Good simplicity:

- fewer states with clearer semantics
- fewer APIs with stronger guarantees
- fewer layers with sharper ownership
- fewer features with higher confidence

Bad simplicity:

- skipping durability
- skipping validation
- skipping restart behavior
- collapsing semantically distinct failure paths into vague behavior
- replacing precise guarantees with hopeful comments

Simple must still be correct.

---

## Ownership Rules

When editing the system:

- preserve strict boundaries between state, scheduler, processor, timers, leases, and APIs
- do not let business logic leak across layers
- do not let I/O creep into pure decision logic
- do not embed policy in random call sites when it belongs in one semantic place

The architecture should be understandable by reading it, not by reconstructing accidental behavior from scattered conditionals.

---

## Product Taste Rules

The product should feel sharp, opinionated, and honest.

We are not trying to impress by having many knobs.
We are trying to impress by making the hard guarantees actually hold.

Favor:

- explicitness
- auditability
- crisp failure semantics
- operator comprehension
- high-signal APIs
- demos that prove real capability

Avoid:

- vague magic
- hidden queues
- hidden retries
- soft correctness
- performative architecture
- abstractions added "for later"

---

## Sprint Discipline

Respect the sprint documents.

- Sprint 1 builds the runtime skeleton
- Sprint 2 makes it execute real work reliably
- Sprint 3 makes the graph dynamic
- Sprint 4 makes it operable under human intervention and failure
- Sprint 5 makes it product-ready and demoable

Do not drag Sprint 4 work into Sprint 1.
Do not drag Sprint 5 polish into Sprint 2.
Do not skip acceptance boundaries because the next sprint feels more exciting.

Momentum comes from finishing sharp slices, not from touching everything.

---

## When In Doubt

Choose the path that is:

- more deterministic
- more recoverable
- easier to audit from the event log
- easier to test under restart
- smaller in scope
- more honest to the v1 contract

If two designs are equally fast, choose the one with stronger guardrails.
If two designs are equally elegant, choose the one with clearer failure semantics.
If two designs are equally powerful, choose the one that is easier to trust.

Trust is the product.

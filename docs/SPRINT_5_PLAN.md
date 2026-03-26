# Grael v1 Sprint 5 Plan

This document defines the fifth implementation sprint for Grael v1.

Sprint 5 turns the now-functional engine into a coherent v1 product surface by tightening the workflow contract, adding the thin Go SDK seam, and packaging the flagship end-to-end demo path.

---

## Sprint Goal

Build the first presentation-ready Grael v1 that can:

- expose a stable minimum workflow definition contract
- support a thin Go worker integration seam
- demonstrate the product through one composed end-to-end scenario
- validate that the previously built execution and control-flow capabilities work together as a single system

At the end of Sprint 5, Grael should feel not only implementable, but demoable and explainable as a product.

---

## In Scope

The sprint includes the following tasks from `docs/V1_TASK_BACKLOG.md`:

1. `T32` Hard-capacity rejection on `StartRun`
2. `T33` Minimal workflow definition contract and definition hash capture
3. `T34` Thin Go worker SDK seam
4. `T35` Composite demo workflow and end-to-end acceptance harness

---

## Why This Slice

Sprints 1 through 4 establish the engine core, reliability semantics, dynamic graph behavior, and operator control flow. Sprint 5 makes those capabilities legible and usable from the outside.

It gives:

- a clearer authoring surface
- a realistic worker integration path
- a final composed acceptance path
- a product story that can be shown to users and used as the basis for early onboarding

without dragging in:

- memory-layer work
- broader platform features
- advanced multi-tenant or security features
- non-v1 extensions such as sub-workflows or richer policy engines

This is the right final slice because the system should already work by this point; Sprint 5 is about tightening the contract and proving the whole story end to end.

---

## Out of Scope

Do not pull these into Sprint 5:

- memory system work
- sub-workflows
- advanced auth, RBAC, or multi-tenancy
- projection systems beyond `GetRun` and `ListEvents`
- non-v1 cancellation modes
- advanced SDK ergonomics beyond the thin seam

Also avoid overbuilding:

- a full platform UX
- migration systems
- marketplace-style packaging
- broad plugin infrastructure

Sprint 5 should finish the v1 surface, not begin v2 or platform work.

---

## Expected End State

By the end of Sprint 5:

- `StartRun` enforces the explicit no-admission-queue v1 contract
- workflow definitions have a stable minimum contract for nodes and node policies
- the definition hash is captured at run start
- a Go worker can register and process work through the thin SDK seam
- the flagship composed demo workflow can run through dynamic spawn, retry, approval, restart, and completion

This is the point where Grael should be ready for serious internal demos, early users, and practical implementation work beyond planning.

---

## Exit Criteria

Sprint 5 is complete when all of the following are true:

- the in-scope tasks are implemented to a usable baseline
- the workflow contract is clear enough to author intended v1 workflows without hidden engine knowledge
- the SDK seam is sufficient for a small reference worker
- the composed demo validates that the major v1 capabilities work together
- the following UATs can pass:
  - [UAT-C9-03-start-run-capacity-reject.md](docs/uat/UAT-C9-03-start-run-capacity-reject.md)
  - [UAT-C10-01-definition-metadata-executes-as-declared.md](docs/uat/UAT-C10-01-definition-metadata-executes-as-declared.md)
  - [UAT-C10-02-go-sdk-worker-seam.md](docs/uat/UAT-C10-02-go-sdk-worker-seam.md)
  - [UAT-C11-01-core-demo-e2e.md](docs/uat/UAT-C11-01-core-demo-e2e.md)

---

## Stretch Goal

If Sprint 5 finishes early, the best stretch target is:

- tighten docs and reference examples around the demo workflow and SDK seam

Why:

- it improves onboarding immediately
- it does not introduce new runtime semantics
- it creates leverage for early adopters without changing the v1 scope

Do not treat the stretch goal as part of the committed sprint scope.

---

## Risks

The main Sprint 5 risks are:

- over-designing the workflow definition contract instead of keeping it minimal
- building the SDK before confirming the worker protocol is truly stable
- turning the demo into a special-case implementation rather than a composed proof of real capabilities
- spending too much time polishing presentation instead of preserving product honesty

---

## Review Questions

At the midpoint and end of the sprint, the team should ask:

1. Can a new user understand how to start a run and write a worker without learning engine internals first?
2. Does the demo workflow prove real engine behavior, or is it held together by shortcuts?
3. Is the workflow contract minimal and honest, or are we sneaking in v2 ideas?
4. Would showing this version of Grael to an engineer create confidence rather than caveats?

If the answer to any of these is "no", the sprint likely needs scope correction before proceeding.

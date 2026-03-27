# Grael

A workflow engine built for AI agents.

Grael orchestrates complex, long-running processes where the structure of work isn't known upfront. Unlike traditional workflow engines that execute a fixed graph, Grael runs a **living graph** — one that grows and reshapes itself as agents discover what needs to be done. Steps can spawn new steps at runtime. Parallel fan-outs emerge from data, not just structure. Compensation unwinds automatically when things go wrong.

At its core, Grael is an append-only event log. Every state transition is an event. The current graph is always derived by replaying that log — which means the engine survives crashes, replays any run from any point in history, and gives you a full audit trail for free.

Built for the SDLC. Designed to be embedded. Ships as a single binary.

---

## Quick Start

Build the CLI:

```bash
go build ./cmd/grael
```

Start a built-in example run:

```bash
./grael start -workflow examples/workflows/linear-noop.json -demo-worker
```

Or try the living-DAG scaffold around runtime spawn:

```bash
./grael start -workflow examples/workflows/living-dag.json -demo-worker
```

Or exercise the Sprint 4 demo composition with spawn, approval, and compensation:

```bash
./grael start -workflow examples/workflows/living-dag-ops.json -demo-worker
```

Or run the flagship composed demo with real multi-worker execution, mid-run graph growth, retry recovery, approval, and final completion:

```bash
./grael start -workflow examples/workflows/core-demo.json -demo-worker
```

`core-demo` now defaults to a slower `showcase` pacing profile so the live UI has
time to animate and tell the story. For a quicker local/dev run, use:

```bash
./grael start -workflow examples/workflows/core-demo.json -demo-worker -demo-profile fast
```

And open the live visual demo in another terminal:

```bash
./bin/grael-demo
```

`-demo-worker` now starts an SDK-based local demo harness. For `core-demo`, it
launches several demo workers so independent ready nodes really overlap through
the public worker surface instead of a bespoke in-process task loop.

The flagship `core-demo` tells a concrete morning incident briefing story:

- static prep work collects customer escalations, checkout metrics, and the briefing shell
- a planning step decides which follow-up checks to open
- the graph fans out into concrete investigations, including one retryable failure
- the editor approval gate waits while unrelated investigation work keeps moving
- the investigation results flow back into the final briefing and publish path

The visual demo is intentionally read-only and derived from `GetRun` plus
`ListEvents`. Paste a run id into the UI and it will render the graph, status
panels, and event timeline with polling-based live refresh.

For the full Sprint 6 operator walkthrough, see
`docs/SPRINT_6_DEMO_RUNBOOK.md`.

Inspect current state:

```bash
./grael status -run-id <RUN_ID>
```

Inspect raw event history:

```bash
./grael events -run-id <RUN_ID>
```

You can also start from your own workflow file:

```bash
./grael start -workflow /path/to/workflow.json
```

Today the CLI accepts workflow files in JSON, but JSON is only an ingress
format. Grael normalizes input into its canonical internal workflow model
before runtime execution, which keeps the engine decoupled from authoring
format choices.

---

## The name

*Grael* is a variation of *grail* — the vessel that holds something rare and essential.

In the context of this engine, the grail is the event log: an immutable, ever-growing record of everything that happened. Every workflow run pours its history into it. The graph lives and dies, but the log remains.

There's a second reading too. In old French, *grael* referred to a shallow dish — something that holds things at the surface, visible, traceable. That's what this engine does: it makes the invisible work of autonomous agents legible.

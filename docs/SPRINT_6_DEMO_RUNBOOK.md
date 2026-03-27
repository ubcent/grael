# Grael Sprint 6 Demo Runbook

This runbook is the operator-facing guide for the Sprint 6 visual demo line.

It exists to make the flagship `core-demo` easy to run, narrate, and verify without relying on engine-internal knowledge.

---

## Purpose

Use this runbook when you want to demonstrate that Grael can show a trustworthy live execution story through the read-only visual demo surface.

The demo should make all of the following visible:

- real overlapping worker execution
- dynamic graph growth during a running workflow
- one retryable failure followed by recovery
- one approval gate that does not represent a frozen run
- a final business outcome that reads clearly from the UI

---

## Flagship Story

The Sprint 6 flagship scenario is `core-demo`, presented as a morning incident briefing workflow.

The narrative is:

1. The run starts with three static preparation steps:
   - collect customer escalations
   - pull checkout metrics
   - prepare the briefing outline
2. A planning step decides which follow-up checks must be opened.
3. Grael dynamically spawns concrete investigation work:
   - verify checkout latency
   - confirm payment auth drop
   - review support spike
4. One spawned investigation fails retryably once, then recovers.
5. Editor approval is requested while unrelated investigation work can still progress.
6. The investigation results are assembled back into a final incident brief.
7. The briefing is published.

This shape is intentional:

- the graph begins with several meaningful static nodes
- dynamic work appears mid-run
- spawned work reconnects into the remaining graph instead of ending in a detached fan-out
- the final state is understandable in business terms

---

## Build

Build the CLI and demo server:

```bash
go build -o bin/grael ./cmd/grael
go build -o bin/grael-demo ./cmd/grael-demo
```

---

## Start The Demo UI

In one terminal:

```bash
./bin/grael-demo
```

Then open the local demo URL shown by the server.

The UI is read-only by design. It derives its view from `GetRun` plus `ListEvents` and must not be treated as an authoritative runtime surface.

---

## Start The Flagship Run

In another terminal:

```bash
./bin/grael start -workflow examples/workflows/core-demo.json -demo-worker
```

For a shorter local iteration:

```bash
./bin/grael start -workflow examples/workflows/core-demo.json -demo-worker -demo-profile fast
```

Paste the emitted run id into the demo UI.

---

## What To Point Out Live

Walk the viewer through the run in this order:

1. Show that the initial graph does not begin as a single-node toy.
2. Point out that the three preparation steps begin in parallel on separate demo workers.
3. Wait for `Decide follow-up checks` to complete and show the graph growing mid-run.
4. Highlight the spawned investigations and the new downstream edges back into the remaining graph.
5. Call out the retryable failure on `Confirm payment auth drop`, then show the retry timer and later recovery.
6. Call out the approval state on `Editor approval` and show that the run still has a truthful story instead of looking like a dead hang.
7. Show `Assemble incident brief` only becoming ready after the investigations complete.
8. End on `Publish morning brief` and the terminal completed state.

The important product message is:

`Parallel work` in the UI is now literally true for the flagship demo, not just graph-theoretically possible.

---

## Expected UI Signals

During a healthy run, a viewer should be able to recognize all of the following from the UI alone:

- the graph grows after the planning step completes
- multiple node starts overlap before the first completion
- the retryable node shows failure and later recovery
- the approval gate is visibly distinct from a generic stall
- the final publish path is fed by earlier dynamic work
- the run reaches `COMPLETED`

---

## Manual Validation Checklist

Use this checklist before calling the Sprint 6 demo ready:

1. `go test ./...` is green.
2. The demo UI still renders from read surfaces only.
3. The initial static preparation nodes are visible by name and easy to explain.
4. Dynamic spawned nodes appear during the run, not only in the initial graph.
5. At least one retryable failure is visible in timeline and graph state.
6. Approval is visible and understandable as an approval gate.
7. The spawned investigation work flows back into the final assembly and publish path.
8. A new viewer can explain the story without opening raw JSON.

---

## Troubleshooting

If the run completes too quickly for narration:

- use the default `showcase` profile instead of `fast`

If the UI looks disconnected from the run:

- verify the pasted run id
- refresh the page and reconnect
- confirm the run still exists through:

```bash
./bin/grael status -run-id <RUN_ID>
./bin/grael events -run-id <RUN_ID>
```

If the graph story looks implausible:

- do not patch the UI first
- inspect the run through `GetRun` and `ListEvents`
- preserve the rule that the UI must stay event-derived and read-only

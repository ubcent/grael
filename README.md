# Grael

A workflow engine built for AI agents.

Grael orchestrates complex, long-running processes where the structure of work isn't known upfront. Unlike traditional workflow engines that execute a fixed graph, Grael runs a **living graph** — one that grows and reshapes itself as agents discover what needs to be done. Steps can spawn new steps at runtime. Parallel fan-outs emerge from data, not just structure. Compensation unwinds automatically when things go wrong.

At its core, Grael is an append-only event log. Every state transition is an event. The current graph is always derived by replaying that log — which means the engine survives crashes, replays any run from any point in history, and gives you a full audit trail for free.

Built for the SDLC. Designed to be embedded. Ships as a single binary.

---

## The name

*Grael* is a variation of *grail* — the vessel that holds something rare and essential.

In the context of this engine, the grail is the event log: an immutable, ever-growing record of everything that happened. Every workflow run pours its history into it. The graph lives and dies, but the log remains.

There's a second reading too. In old French, *grael* referred to a shallow dish — something that holds things at the surface, visible, traceable. That's what this engine does: it makes the invisible work of autonomous agents legible.

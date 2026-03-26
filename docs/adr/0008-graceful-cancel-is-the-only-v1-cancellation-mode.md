# ADR 0008: Graceful Cancel Is The Only V1 Cancellation Mode

- `Status`: `Accepted`
- `Date`: `2026-03-26`
- `Deciders`: `Project maintainers`
- `Related docs`: `docs/V1_CANONICAL_BASELINE.md`, `docs/V1_SCOPE.md`, `docs/SPRINT_4_PLAN.md`
- `Supersedes`:
- `Superseded by`:

---

## Context

Cancellation in distributed systems is easy to misrepresent.

One possible model is a forceful mode that implies:

- process interruption
- immediate stop of in-flight work
- hard revocation semantics that appear stronger than they really are

For Grael v1, we must decide what cancellation semantics we are willing to promise honestly.

---

## Decision

Grael v1 will support `GracefulCancel` only.

That means:

- cancellation requests propagate through the engine
- pending and ready work is prevented from continuing normally
- running work is handled through the graceful path the system can actually support

Grael v1 will not expose a stronger cancellation mode that implies guarantees it cannot honestly enforce.

---

## Alternatives Considered

### Add forceful cancel semantics in v1

Rejected because distributed systems cannot honestly guarantee process kill or side-effect rollback simply by exposing a stronger-sounding API.

### Expose both graceful and forceful modes, but document caveats

Rejected because caveat-heavy semantics are worse than a smaller honest contract.

### Avoid cancellation in v1 entirely

Rejected because the ability to stop workflows is table stakes for a usable engine.

---

## Consequences

Benefits:

- smaller and more honest cancellation contract
- less semantic confusion for operators
- lower risk of overpromising distributed interruption guarantees

Tradeoffs:

- some emergency-stop scenarios remain outside v1
- future versions may need a broader cancellation model

---

## Guardrails

The following rules now follow from this decision:

- do not expose stronger cancellation semantics than the engine can enforce honestly
- do not imply that v1 can kill worker processes or reverse in-flight side effects immediately
- do not widen cancellation APIs beyond graceful semantics without a superseding decision

---

## Validation

This decision is holding if:

- [UAT-C7-01-graceful-cancel.md](../uat/UAT-C7-01-graceful-cancel.md) passes
- the public cancellation contract remains clear and unsurprising
- no v1 API claims forceful interruption semantics it cannot actually deliver

---

## Notes

This ADR protects the honesty of the product as much as the simplicity of the implementation.

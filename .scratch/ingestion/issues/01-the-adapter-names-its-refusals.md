# 01: The adapter names its refusals

**What to build:** The [spec](../spec.md)'s decision, recorded before any code
depends on it: the GitHub adapter answers with an outcome — an event, or a refusal
carrying its reason — and `undefined` does not cross that seam. The ADR states the
two reasons a payload produces nothing (`not-critical`, a decision; `malformed`, a
defect), the rule that a malformed answer is terminal in composition so a
half-changed merge payload is never reinterpreted as an assignment, and the
rejected alternatives with their reasons: `undefined` (erases the difference),
`Option`/`Maybe` (informationally identical to `T | undefined`, and it defeats the
compiler's flow narrowing at the call site), and a thrown error (incompatible with
ADR-0002, which acknowledges and discards rather than rejects).

**Blocked by:** None (can start immediately).

**Status:** done

- [x] A new ADR in the project's register records the decision, in the house form,

  numbered next in sequence.
- [x] It defines both refusal reasons, and says which one is normal and which one is

  a defect.
- [x] It states the composition rule: `malformed` is terminal, so one payload never

  yields two readings.
- [x] It records the rejected alternatives and why each was rejected.
- [x] It cross-references the ADRs it touches (signature-then-parse, acknowledge-and-

  discard, the event shape) and contradicts none of them.
- [x] It states what the decision does **not** change: the `200` on every

  authenticated delivery, the stored rows, the served history.
- [x] No source code changes in this ticket.


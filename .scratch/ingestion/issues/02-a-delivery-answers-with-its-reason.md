# 02: A delivery answers with an outcome, or with its reason

**What to build:** One delivered webhook produces exactly one of two things: a
stored event, or a refusal that names why there is none. Today the adapter answers
`undefined` for "not critical" and for "critical but unreadable" alike, so the
relay cannot tell a quiet repository from a payload shape that drifted; and the
guards that decide it are type predicates promising fields they never verified, so
each mapper re-reads every field defensively and the compiler contributes nothing
at the one place it was hired for.

End to end: the adapter answers a discriminated outcome, each adapter parses the
payload in a single pass and can only claim what it checked, the HTTP layer names an
unreadable body apart from a payload with no critical event, and the controller
branches on the kind. Nothing crosses that seam as `undefined`.

**This is a naming and typing change, not a policy change.** Every filter decision,
every stored row, every field value and every response code must be identical to the
base commit. The evidence record under the feature's ticket directory is written once
the criteria are met and the work is merged, per the project's convention.

**Blocked by:** 01 — the decision must be on record first.

**Status:** ready-for-human

- [x] A delivered payload yields either a stored event or a named refusal
      (`not-critical` / `malformed`); the adapter's return type has no `undefined`
      variant, and the compiler rejects a caller that ignores the kind.
- [x] No type predicate and no type assertion remain in the GitHub adapter. Each
      adapter walks the payload once, and every field it reads is checked in the same
      pass that decides the outcome.
- [x] A payload that passes a filter but cannot be normalized is `malformed`; a
      payload that fails a filter is `not-critical`. The merge-then-assignment
      composition stops at `malformed`, so one payload never yields two readings.
- [x] An unreadable request body is distinguishable from a payload that carried no
      critical event, without changing what the sender sees.
- [x] The wire contract is untouched: `200 {"received":true}` for every authenticated
      delivery, `401` for a bad signature, nothing stored on a refusal.
- [x] Duplicate deliveries still store once and still never reach the stream twice.
- [x] The existing end-to-end suite passes with **no test edited**. Its cases are the
      regression gate: green before, green after, over the same payloads.
- [x] A differential check confirms the served history is byte-for-byte identical to
      the base commit for the same fitted payloads.
- [x] Build, lint and the relay suite are green.
- [x] No new dependency: the outcome type is TypeScript's own discriminated union.
- [x] The flow document's adapter steps and its file table describe the named
      outcomes, and the architecture document's repository tree matches the modules
      that exist afterwards.

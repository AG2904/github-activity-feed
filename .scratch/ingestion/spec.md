# Ingestion — Typed Refusals at the Adapter Seam

Status: ready-for-agent

## Problem Statement

When a webhook produces no event, the relay cannot say why. The GitHub adapter
answers `undefined` for two situations that are not the same thing: a payload
that is not critical — the filter working as designed — and a payload that *was*
critical but could not be normalized, which means the adapter and GitHub
disagree about a shape the filter already accepted.

From outside the relay both look identical, and that is deliberate for the wire:
both are acknowledged with `200` and neither is stored (ADR-0002). But inside
the relay the difference is the only thing that would let a maintainer answer
"why is my event missing?" without re-reading a 25 KB payload by hand. Today that
answer is erased, and nothing can log it, count it, or assert on it.

The same seam carries a second defect, closer to the type system: the adapter's
guards are type predicates that promise more than they verify. A guard proves
three fields of a payload and then declares the whole payload type — title, URL,
labels and all — so each mapper immediately re-reads every one of those fields
defensively, with `typeof` checks, as if the signature meant nothing. Validation
happens twice and the compiler contributes nothing at exactly the place it was
hired for. The symptoms are visible: optional chaining on values that are
declared non-nullable, and `asRecord()` on values the signature says are already
objects.

## Solution

The GitHub adapter answers with one type: an **ingestion outcome**. Either the
event, or an explicit refusal that carries its reason. No `undefined` crosses
that seam, so every caller that receives nothing also receives why.

Each adapter parses in a single pass — filter and normalization are the same walk
over the same payload — and therefore can only claim what it actually checked.
The type predicates disappear, and with them the lie they depended on.

The wire contract does not change: every authenticated delivery is acknowledged
with the same `200`, whether it produced an event, a refusal, or a duplicate.

## User Stories

1. As a relay maintainer, I want a payload that produced no event to tell me
   whether it was not critical or could not be normalized, so that I know whether
   I am looking at the filter working or at a payload shape that drifted.
2. As a relay maintainer, I want the refusal reason to be part of the adapter's
   return type, so that I cannot forget to handle it when I add a caller.
3. As a relay maintainer, I want the adapter to stop returning `undefined`, so
   that "no event" is never silently indistinguishable from "nobody checked".
4. As a relay maintainer, I want a single type per adapter answer, so that the
   controller, the tests and any future logging read the same vocabulary.
5. As a developer debugging a missing event, I want the difference between "not
   critical" and "malformed" to be nameable, so that I can find the cause without
   re-reading the raw payload.
6. As a developer debugging a missing event, I want a malformed merge not to be
   reinterpreted as an assignment, so that a payload GitHub half-changed produces
   one honest refusal instead of a wrong event.
7. As a developer debugging a missing event, I want an unreadable request body to
   be distinguishable from a payload that simply carried no critical event, so
   that a broken sender is not mistaken for a quiet repository.
8. As a reviewer, I want a guard that promises only the fields it verified, so
   that I can trust a type predicate without auditing its body.
9. As a reviewer, I want the filter and the normalization to read the payload
   once, so that I do not have to check that two passes agree about the same
   field.
10. As a reviewer, I want no type assertion standing in for validation at the
    adapter seam, so that a wrong assumption fails at compile time instead of at
    runtime in the history.
11. As a contributor adding a new critical event type, I want an exhaustive
    outcome type, so that the compiler tells me every place that has to learn
    about the new kind.
12. As a contributor adding a new refusal reason, I want the reason to be a
    closed union, so that every consumer is forced to consider it.
13. As a contributor, I want the adapter's exported surface to be one function
    per event type plus the entry point, so that the shape of the module tells me
    how ingestion works without reading it.
14. As a contributor, I want dead type scaffolding to be deleted rather than
    left behind, so that the module does not document fields nobody can rely on.
15. As the team relying on the relay, I want the wire contract untouched by this
    change, so that no client, GitHub delivery, or stored row behaves differently
    because of an internal refactor.
16. As the team relying on the relay, I want the stored history to be byte-for-
    byte identical after the change, so that the audit record keeps its meaning.
17. As the team relying on the relay, I want the same payloads to be stored and
    refused as before, so that the fix is about naming, not about policy.
18. As the maintainer of the documentation, I want the flow document to describe
    the named outcomes, so that the map matches the code.
19. As a future reader of the decision, I want the choice recorded in an ADR, so
    that "why not `undefined`" and "why not `Option`" are not re-litigated from
    memory.
20. As a future reader of the decision, I want the rejected alternative recorded
    with its reason, so that the rejection is auditable rather than folklore.

## Implementation Decisions

- **One outcome type.** Ingestion answers with a discriminated union of two
  variants: an event, carrying the normalized `ActivityEvent`; and a refusal,
  carrying a reason. The union is the adapter's only return shape.

  ```
  Ingestion =
    | { kind: 'event';   event: ActivityEvent }
    | { kind: 'ignored'; reason: 'not-critical' | 'malformed' }
  ```

- **The reasons are defined, not adjectival.** `not-critical` is a decision: the
  filter read the payload and passed on it, which is the normal case. `malformed`
  is a defect: the payload passed the filter and a field the event needs was
  missing or unusable. Nothing else will use these strings.

- **A malformed answer is terminal in composition.** Composition tries the merge
  adapter, and then the assignment adapter only when the first answered
  `not-critical`. A merge that is critical but unreadable is never reinterpreted
  as an assignment: it is the same payload, and a half-changed shape must produce
  one refusal, not a plausible-looking wrong event.

- **Predicates are removed, not narrowed.** Each adapter does its filter and its
  normalization in one pass and returns the outcome directly. No type predicate,
  no type assertion, and therefore no claim the code did not verify. The payload
  readers that answer `T | undefined` for a single field stay as they are: there,
  "was this field here?" is the question, and `undefined` is the honest answer to
  it.

- **The request body gets the same treatment.** Parsing the raw bytes today
  answers `undefined` for "no body" and for "not JSON", which is the same
  conflation one layer up. Body parsing answers a small tagged outcome instead,
  and the controller returns the acknowledged response for an unreadable body
  without consulting the adapter.

- **The controller branches on the kind.** An event is stored and, only when the
  store reports a fresh row, delivered to the stream. A refusal is acknowledged
  and dropped. The controller holds no `if (event)` truthiness check.

- **The duplicate-versus-fresh signal is untouched.** The service's boolean —
  meaning "the row was written, not a redelivery" — stays a boolean: it is a
  two-state question about a write, not about what the payload was.

- **The wire contract does not move.** `200 {"received":true}` for every
  authenticated delivery; `401` for a bad signature; nothing stored for a
  refusal; stored events and history identical to before.

- **No dependency.** The union is TypeScript's own tool. `Option`/`Maybe` was
  considered and rejected: it is informationally identical to `T | undefined`
  (it cannot say *why* there is nothing), it defeats the compiler's flow
  narrowing at the call site, and `ActivityEvent` is a wire contract that must
  not leak a wrapper into JSON.

- **The module's shape shrinks.** The two predicates, the two mappers, and the
  payload interfaces they declared disappear. What remains is one function per
  critical event type and the entry point that composes them, plus the outcome
  type in its own module so the three consumers can import it without a cycle.

- **An ADR records the decision**, superseding nothing and extending ADR-0003's
  territory: the adapter names its refusals, and `undefined` does not cross that
  seam. It will note the rejected alternatives (`undefined`, `Option`, and a
  thrown error) with their reasons.

- **The documentation follows the code**: the flow document's adapter steps and
  its file table, and the architecture document's repository tree.

- **Preservation is a hard requirement.** Every filter decision, every stored
  event, every field value, and every response code must be identical before and
  after. This is a naming and typing change, not a policy change.

## Testing Decisions

- **One seam, and it already exists**: the HTTP end-to-end suite. No new seam is
  proposed. Adding a module-level unit test seam for the adapter would double the
  testing vocabulary for one internal refinement, against the project's single
  preferred seam.
- **What makes a good test here** stays what it is today: drive a signed request
  through the real HTTP surface and assert the observable result — status code,
  body, and what `/history` serves afterwards. Implementation details of the
  adapter are not asserted.
- **The regression gate is the existing suite**, which is expected to stay green
  without a single edit. Because the change is behavior-preserving, green before
  and green after, over the same cases, is the evidence; the prior art is the
  whole of the existing end-to-end suite.
- **The refusal reason is not observable at this seam, and this spec accepts
  that.** It is guaranteed by the compiler — an exhaustive union, and no
  `undefined` in the adapter's return type — not by a runtime assertion. The
  moment the reason drives something observable (a log line, a metric, a
  response header), that feature introduces its own seam and its own tests; this
  spec does not premise them.
- **A differential check on stored output** performs the same fitted payloads
  against the base commit and against the change and compares the served history
  byte for byte.
- **Seam confirmed** by the maintainer on 2026-10-28: one seam, the existing HTTP
  end-to-end suite, no module-level unit seam for the adapter. The refusal reason
  stays a compile-time guarantee rather than a runtime assertion.

## Out of Scope

- The type assertions between a stored row and the domain shape
  (`row.type as ActivityEventType`, `row.when as IsoInstant`) and the untyped
  entity columns behind them. Same family of defect, different seam; it deserves
  its own ticket because it changes what a corrupt row does.
- The service's boolean return, and whether "duplicate" deserves a name.
- Schema-validation libraries (zod, valibot) and any single-pass schema parsing.
- A logging or metrics surface that consumes the refusal reasons.
- Any change to the filters, the event shape, the storage schema, the wire
  contract, or the CLI.
- The `cobra` analysis of the Go client, which is separate and already archived.

## Further Notes

- **Do not start the code yet.** A security audit is running against the
  repository from another machine; new code must not land until it finishes. The
  spec exists so the work is ready when the audit closes, and so the audit is
  reasoning about the code that is actually there.
- **Suggested order once it unblocks**: the ADR, then the outcome type, then the
  two adapters, then the entry point and the controller, then the documentation,
  then the suite run and the differential check.
- **Vocabulary**: "ingestion" and "refusal" are the words this spec uses for "the
  adapter's decision about one delivered payload" and "an answer carrying no
  event". They join the glossary's existing *relay*, *ingestion*, and *critical*.

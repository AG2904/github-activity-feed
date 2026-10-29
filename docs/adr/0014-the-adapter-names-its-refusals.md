# The adapter names its refusals, and no undefined crosses that seam

The GitHub adapter answers with an outcome, never with `undefined`: either the event, or a refusal that carries its reason — `not-critical`, the filter working as designed, or `malformed`, a payload that passed a filter and could not be normalized. Ingestion is the only place the difference survives, because the wire answers the same success code to both (ADR-0002) and stores neither.

The immediate reason is what `undefined` erased. Asked "why is my event missing?", the relay could not answer without a human re-reading a 25 KB payload: an assignment with a `chore` label and a merge whose `html_url` went missing were the same value. One is the product deciding not to interrupt anyone; the other is this adapter and GitHub disagreeing about a shape. They deserve different words, and a caller that only wants the event still gets one line.

The second reason is a type defect at the same seam. The guards were type predicates that promised more than they verified: a guard read `action`, `merged` and `base.ref`, then declared the whole payload shape — `title`, `html_url`, `merged_at`, `labels` and all — so each mapper immediately re-read every one of those fields with `typeof` checks, as if the signature meant nothing. Validation happened twice and the compiler contributed nothing where it had been hired. The symptoms were visible from outside: optional chaining on values declared non-nullable, and `asRecord()` on values the signature said were already objects. A predicate that cannot keep its promise is worse than no predicate, because it launders an assumption into the type system.

## Considered Options

- **Keep `undefined`, and document the two meanings in a comment**: rejected. That is what the code did, and the comment cannot be read by a caller, counted, or asserted on.
- **`Option`/`Maybe`**: rejected, and not narrowly. It is informationally identical to `T | undefined` — `None` cannot say *why* there is nothing, so it renames the problem instead of solving it — it defeats the compiler's flow narrowing at the call site by requiring an unwrap before the value can be used, and `ActivityEvent` is a wire contract that must not leak a wrapper into JSON. The case where a monad pays is a deep chain of lookups; this payload is wide, not deep, and a monad would cost a lambda per field.
- **Throw, and let the HTTP layer catch**: rejected. ADR-0002's rule is that an authenticated delivery is acknowledged and discarded, never rejected; a missing title is not a request error.
- **Narrow the predicates only, keeping two passes**: rejected as a half measure. It stops the lie, but the mapper still re-validates everything, and the filter and the normalization still read the same payload twice — the second time with less information than the first.
- **Validate the payload with a schema library**: rejected for now, not forever. It would collapse both passes into one and give the reasons for free, at the price of the relay's first runtime dependency, for a payload whose whole surface is five fields.

## Consequences

`malformed` is **terminal** in composition. The entry point tries the merge adapter and then the assignment adapter only when the first answered `not-critical`. A merge that is critical but unreadable is never reinterpreted as an assignment: it is the same payload, read twice, and a half-changed shape must produce one honest refusal instead of a plausible-looking wrong event.

The refusal reason is a **compile-time** guarantee, not an observable one. No response header, log line or metric carries it yet, so the existing end-to-end suite cannot assert it: what the suite pins is that the same payloads are stored and refused as before. The moment something consumes the reason, that feature brings its own seam and its own tests. This is the deliberate cost of keeping one testing seam in the relay.

The request body gets the same treatment, one layer up. Parsing the raw bytes answered `undefined` for "no body" and for "not JSON", which is the same conflation: a sender that broke its serialization was indistinguishable from a delivery that carried nothing worth reporting.

`T | undefined` survives **inside** the field readers — `asRecord`, `loginOf`, `isoInstant` — and that is not an inconsistency. There the question really is "was this field here?", asked of one value, and `undefined` is the honest answer. The distinction this ADR draws is between the absence of a field and the absence of an event: the first is a detail, the second is a decision that deserves a name.

The wire contract, the stored rows and the served history do not move. This is a naming and typing change: the same payloads are stored, the same payloads are refused, and the same `200` answers both.

One defect of the same family is left standing, deliberately and out of scope here: the mapping from a stored row to the domain still asserts its way through `row.type as ActivityEventType` and `row.when as IsoInstant`, so a corrupt row can still lie to the type system. It belongs to a different seam — storage, not ingestion — and it changes what a bad row does, which is a decision worth taking on its own.

# At-least-once webhook delivery: dedupe on the delivery identifier

> **Amended by [ADR-0013](0013-branch-as-a-field-of-the-event.md).** The shape now has
> nine fields, not eight: ticket 05 added `branch`. Everything else in this decision
> stands — the delivery identifier is still a row-level column, still outside the event
> shape, and the unique index on it is still the whole dedupe.

Delivery is at-least-once. GitHub does not automatically redeliver failed webhooks, but a delivery still arrives more than once when a timeout is retried or someone redelivers it by hand — and `X-GitHub-Delivery`, the per-delivery GUID, is **unchanged across redelivery**. So a duplicate is not evidence of an automatic retry; it is one delivery arriving again, carrying the same identifier.

We persist that identifier on each ActivityEvent **row** under a unique index, and drop any incoming delivery whose identifier already exists, acknowledging it exactly as if it had been stored.

## The identifier belongs to the row, not to the event

The delivery identifier is deliberately **not** part of the `ActivityEvent` shape. This is the part of the decision a future reader will otherwise try to "fix", so it is worth stating plainly:

- An ActivityEvent describes a *happening*: what happened, where, to whom, where to look, and when. It carries exactly eight fields, enumerated in the spec's normalization decision and in user story 10, and the same shape is served by both the stream and the history so the CLI parses one thing (user story 16).
- The identifier identifies a *delivery*, not the happening. Attaching it to the shape would put transport metadata into the audit record and hand every client a field it can never use — the CLI is a display client with nothing to dedupe.
- It does not even exist where an event is built: `X-GitHub-Delivery` is a request **header**, not a body field. An event assembled from the payload cannot carry it without the domain reaching into HTTP.

The storage id and the delivery id are therefore both row-level columns, and both excluded from the shape. The read path already enforces this for the storage id; see the ticket 02 record.

## Considered Options

- **A hash of the normalized event** (actor + type + subject + when): needs no identifier at all. Rejected — it conflates two genuinely distinct happenings that happen to look identical, so a legitimate second assignment of the same issue to the same person in the same second would be silently swallowed. Dropping a real event from an audit record is worse than storing a duplicate.
- **`X-GitHub-Delivery`** (chosen): the identifier GitHub itself uses to mean "this delivery", and stable across redelivery, so it collapses exactly the duplicates we want collapsed.

## Consequences

The store path needs the header value alongside the domain event, so storing takes the event plus the delivery identifier — one domain argument and one infrastructure argument. That asymmetry is the honest cost of keeping the shape clean, and it is the seam to watch if it ever starts to spread.

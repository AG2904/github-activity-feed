# The repository is derived from the event's URL, not added to the shape

The CLI groups and filters by repository, and `ActivityEvent` has no repository field. Rather than extend the shape, the CLI derives it from the event's `url` — its first two path segments — and keeps that derivation in one place.

The relay receives `repository.full_name` on every webhook and discards it, so a field would be the honest model. It is not added here for three reasons. The shape is a frozen, tested, and documented contract: "seven fields, not one more" is written into the architecture document, enumerated in user story 10, and asserted by deep equality across the whole suite, so changing it reopens a completed phase and touches the storage schema. Grouping is a *presentation* concern, and forcing a schema and wire change on a shipped backend for a view the client renders is engineering in the wrong direction. And the derivation is robust for the cases that exist, because a GitHub pull request or issue URL always carries `owner/repo` as its first two path segments — including on GitHub Enterprise, where only the host differs.

The cost is real and worth naming: a URL is a *link to* a thing, and this makes it the source of truth for *where* the thing is. If the relay ever serves a URL that is not a GitHub pull request or issue, the derivation lies.

## Considered Options

- **A `repository` field on `ActivityEvent`** — the clean fix, deferred rather than rejected. It is filed as a relay follow-up, and the CLI is built so it can adopt it: the derivation is one function, and replacing it would not touch the parser, the renderer, or the filters.
- **Deriving it in the renderer**, at the point of display: rejected, because the filter needs the same value. A repository the renderer knows and the filter does not is a second definition waiting to disagree with the first.

## Consequences

The CLI's tests assert grouping and filtering through the process seam — by feeding events whose URLs differ — rather than by testing the derivation directly. That is consistent with the single-seam testing decision and leaves the derivation free to be replaced by a field.

If the repository becomes a query the relay should answer, this is the decision to revisit. The trigger is concrete: the team watching more repositories than one and wanting the history narrowed before it crosses the network, rather than after.

# The repository is a field of the event, and the shape is eight fields

`ActivityEvent` carries `repository` as `owner/name`, read from the webhook's `repository.full_name` and stored in its own column, and `GET /history` accepts a `repository` filter. This supersedes [ADR-0006](0006-repository-derived-from-the-url.md), which derived the same value from the event's `url`.

ADR-0006's reasons were sound when it was written, and restating them is the point, because they are what changed. The shape was frozen: "seven fields, not one more" was written into the architecture document, enumerated in user story 10, and asserted by deep equality across the whole suite, so an eighth field meant reopening a completed phase and touching the storage schema. And grouping by repository looked like a *presentation* concern, which the client could serve without the relay ever knowing.

What changed is that the derivation turned out to be wrong in kind, not merely inconvenient. A URL is a *link to* a thing, and reading `owner/repo` out of its path makes a link the source of truth for *where* something happened — and only for URLs that happen to have the shape this relay produces. The relay, meanwhile, receives `repository.full_name` on every single webhook and throws it away. A client parsing a link to recover a value the server already held is the wrong shape of dependency, and "presentation concern" stops being an argument once the presentation needs the value exactly, for grouping and for filtering alike.

The cost was the one ADR-0006 predicted: the suite's deep-equality assertions moved, the fixtures stopped being trimmed payloads, and every document that said "seven fields" had to move with them. That is the work, and it is why the decision was deferred rather than taken cheaply — not why it was avoided.

## Considered Options

- **Deriving from the `url`** (ADR-0006, and what the CLI was first specified to do): rejected for the reason above. Its record is kept, because it is the argument that will be made again if anyone proposes removing the field.
- **A `repository` object instead of a string**: rejected. The client groups and filters on `owner/name` and needs nothing else from GitHub's repository object, so a string is the whole requirement and an object would invite fields nobody asked for.

## Consequences

The column is **nullable**, and that is forced rather than chosen. SQLite cannot add a `NOT NULL` column to a populated table, and TypeORM answers a schema change by rebuilding the table and copying the rows, so a `NOT NULL` column makes the copy fail and the relay refuse to boot — it retries forever and never starts. That was measured against a real populated database before the design was settled, not assumed. A row written before the column existed therefore answers with `repository: null`; recovering it from the `url` is precisely the derivation this decision removed, and it would only work for URLs of the shape we happen to produce, so the honest answer for those rows is that the repository is unknown.

`repository` is the shape's second nullable field, after `assignee`, and the two are nullable for different reasons worth keeping apart: `assignee` is null because a *kind* of event has none, while `repository` is null because a *row* predates the column. A client that treats the two the same will be wrong about one of them.

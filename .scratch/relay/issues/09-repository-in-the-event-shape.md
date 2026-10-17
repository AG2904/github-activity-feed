# 09: The repository becomes a field of the event

**What to build:** A `repository` field on `ActivityEvent`, carried from the payload through storage to the wire.

**Status:** ready-for-human

- [x] The shape carries the repository as `owner/name`, read from the payload's `repository.full_name`.
- [x] The stored row gains a column, and an existing audit history survives the change.
- [x] `GET /history` accepts a repository filter.
- [x] The seven-field invariant is retired deliberately: the spec, the architecture document, the glossary, and every deep-equality assertion in the suite move together rather than drifting apart.
- [x] The fixtures stop being trimmed payloads and carry the `repository` object GitHub actually sends.

## Comments

- 2026-10-17: Opened by the Phase 2 grilling, and deliberately not part of it. The CLI needs a repository to group and filter by, and derives it from the event's `url` instead (ADR-0006). That works and is tested, but it makes a link carry data, which is the wrong source of truth for *where* something happened — the relay receives `repository.full_name` on every webhook and throws it away. It is deferred because it changes a frozen, tested, documented contract and reopens a completed phase. The derivation is isolated in one function precisely so that this ticket can land without the CLI changing inside.
- 2026-10-17: Completed (54 e2e cases, 5 new). The shape is eight fields, the column is nullable, and `GET /history` narrows by `user`, by `repository`, or by both. ADR-0010 supersedes ADR-0006 and records why; ADR-0006 keeps its reasoning with a pointer, because it is the argument someone will make again.

  Two things are worth carrying forward. The column had to be **nullable**, and that was measured rather than assumed: a `NOT NULL` column makes TypeORM's table rebuild fail its row copy on SQLite, and the relay then retries forever and never boots. And the fourth criterion's "glossary moves together" turned out to need nothing — the glossary never enumerated the fields, and its **Repository** entry already said exactly what the field now carries. The invariant was retired in the spec, the architecture document, ADR-0005, ADR-0006 and the manual runbook, and in every deep-equality assertion in the suite.

  The suite gained a case that boots its own app against a database written with the old schema, which is the only way to prove the second criterion rather than assert it. The evidence record is at `docs/tickets/relay/09-repository-in-the-event-shape.md`.

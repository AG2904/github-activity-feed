# 09: The repository becomes a field of the event

**What to build:** A `repository` field on `ActivityEvent`, carried from the payload through storage to the wire.

**Status:** needs-triage

- [ ] The shape carries the repository as `owner/name`, read from the payload's `repository.full_name`.
- [ ] The stored row gains a column, and an existing audit history survives the change.
- [ ] `GET /history` accepts a repository filter.
- [ ] The seven-field invariant is retired deliberately: the spec, the architecture document, the glossary, and every deep-equality assertion in the suite move together rather than drifting apart.
- [ ] The fixtures stop being trimmed payloads and carry the `repository` object GitHub actually sends.

## Comments

- 2026-10-17: Opened by the Phase 2 grilling, and deliberately not part of it. The CLI needs a repository to group and filter by, and derives it from the event's `url` instead (ADR-0006). That works and is tested, but it makes a link carry data, which is the wrong source of truth for *where* something happened — the relay receives `repository.full_name` on every webhook and throws it away. It is deferred because it changes a frozen, tested, documented contract and reopens a completed phase. The derivation is isolated in one function precisely so that this ticket can land without the CLI changing inside.

# 05: A resent event is stored once

**What to build:** Delivery is at-least-once, so the same delivery can arrive twice. The relay stores it exactly once.

**Blocked by:** 02

**Status:** ready-for-human

- [x] The delivery identifier (`X-GitHub-Delivery`) is stored on the event's row under a unique constraint.
- [x] The identifier is a storage column, never a field of the `ActivityEvent` shape (ADR-0005).
- [x] Posting the same delivery twice stores a single row.
- [x] The duplicate is still acknowledged with a success status.
- [x] Two different events posted back to back both survive.
- [x] The duplicate is rejected by the unique constraint itself, never by reading first and writing after: two deliveries carrying the same identifier that arrive at the same time are both acknowledged, and neither produces an error response. _(Implemented — the store path inserts and catches the constraint, with no read before the write — but the criterion is not provable at the HTTP seam: see Comments.)_

## Comments

- 2026-10-15: Rewritten for the GitHub migration. The identifier moves from GitLab's `Idempotency-Key` / `X-Gitlab-Event-UUID` to GitHub's `X-GitHub-Delivery`, which is a request **header** and is unchanged across redelivery — which is what makes it usable as the key.
- 2026-10-15: The second criterion is new, and it exists because the spec's previous phrasing ("stored per event") was ambiguous enough to be read as a field of the event. It is not one: an ActivityEvent describes a happening and carries the same seven fields whether it is streamed or read from the history, while the delivery identifier belongs to the row, sits alongside the storage id, and never reaches the wire. The reasoning is in ADR-0005.
- 2026-10-16: Completed (40 e2e cases, 6 new). One commit on `main`: `feat: store a delivery once, however many times it arrives (ticket 05)`. The unique index was observed refusing an insert during the red phase (`QueryFailedError: SqliteError: UNIQUE constraint failed: activity_events.deliveryId`, `code: 'SQLITE_CONSTRAINT_UNIQUE'`), so the mechanism is real and not just declared.

  The last criterion is implemented but **not proven**, and that is the honest limit of this ticket. The concurrent test holds the contract (both deliveries acknowledged, one row) yet cannot distinguish a constraint-rejected insert from a read followed by a write: better-sqlite3 is synchronous, so the two writes serialize on the event loop and a reader-first implementation would pass it identically. The spec's testing decisions rule out reaching into the ORM, so the mechanism cannot be observed at this seam at all. Compensations: a case where the identifier repeats while the payload **differs**, which does prove the stored key is the header's value rather than the event's content and that nothing is re-read to decide; a comment in the test stating what it can and cannot distinguish; and the structural fact that `store` has no read before its insert.

  Two smaller calls, both recorded in `docs/tickets/relay/05-dedupe-resent-events.md`: the column is nullable (discarding a delivery with no identifier would lose a real event, which ADR-0005 rates worse than storing a duplicate), and the untagged path is pinned by a test that documents its cost — two untagged arrivals are two rows. Status `ready-for-human` per the canonical triage labels. The evidence record is at `docs/tickets/relay/05-dedupe-resent-events.md`.

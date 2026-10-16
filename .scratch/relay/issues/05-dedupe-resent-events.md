# 05: A resent event is stored once

**What to build:** Delivery is at-least-once, so the same delivery can arrive twice. The relay stores it exactly once.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] The delivery identifier (`X-GitHub-Delivery`) is stored on the event's row under a unique constraint.
- [ ] The identifier is a storage column, never a field of the `ActivityEvent` shape (ADR-0005).
- [ ] Posting the same delivery twice stores a single row.
- [ ] The duplicate is still acknowledged with a success status.
- [ ] Two different events posted back to back both survive.
- [ ] The duplicate is rejected by the unique constraint itself, never by reading first and writing after: two deliveries carrying the same identifier that arrive at the same time are both acknowledged, and neither produces an error response.

## Comments

- 2026-10-15: Rewritten for the GitHub migration. The identifier moves from GitLab's `Idempotency-Key` / `X-Gitlab-Event-UUID` to GitHub's `X-GitHub-Delivery`, which is a request **header** and is unchanged across redelivery — which is what makes it usable as the key.
- 2026-10-15: The second criterion is new, and it exists because the spec's previous phrasing ("stored per event") was ambiguous enough to be read as a field of the event. It is not one: an ActivityEvent describes a happening and carries the same seven fields whether it is streamed or read from the history, while the delivery identifier belongs to the row, sits alongside the storage id, and never reaches the wire. The reasoning is in ADR-0005.

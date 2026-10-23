# The branch is a field of the event, and the shape is nine fields

`ActivityEvent` carries `branch` — the source branch of a merge, read from the webhook's `pull_request.head.ref` — as a nullable string in its own column, and both `/history` and `/stream` serve it. It follows [ADR-0010](0010-repository-as-a-field-of-the-event.md), which added `repository` in the same shape and for the same kind of reason.

This decision amends the field-count claims of [ADR-0005](0005-at-least-once-dedupe-on-delivery-id.md) and [ADR-0010](0010-repository-as-a-field-of-the-event.md): both said the shape had eight fields, and it now has nine. Both hold in everything else — the delivery identifier still belongs to the row, and the repository is still a field rather than a derivation.

The card used to say `maria.perez merged into main` because that was all the shape allowed. `merge-to-main.ts` read `base.ref` to decide whether a merge was critical, and `head.ref` was in the payload and unread: the relay knew where the work landed and threw away which line of work it was. Deriving the phrase from the type was honest while the field did not exist — `pr.merged` *does* mean a merge into `main` — but it answers the destination and not the thing a developer actually wants to know. "Something landed in main" is a notification; "`fix/memory-leak` landed in main" is the event.

The information was never missing. GitHub sends it on every pull-request delivery, the same way it sends `repository.full_name`. Adding it is the same move ADR-0010 made: a value the server already holds belongs in the event, not in a guess the reader has to reconstruct from the type.

## Considered Options

- **Expose the branch only in `--json` and `history`, leaving the card alone**: rejected. It is cheaper — no phrase questions, no render change — but it answers the storage, not the reader. The feed is the surface that asked for the field.
- **Keep deriving `into main` from the type and add no field**: rejected for the reason above: the type says where, not what landed, and no amount of rephrasing recovers the branch from it.
- **Treat the branch as required and drop a merge payload without one**: rejected. The branch is supplementary, and the relay's rule is that an authenticated critical event is worth keeping even when a detail is missing (ADR-0005's preference for a duplicate over a lost event). A missing or unusable head ref normalizes to `branch: null`.

## Consequences

The column is **nullable**, and that is forced rather than chosen, exactly as it was for `repository`. SQLite cannot add a `NOT NULL` column to a populated table, and TypeORM answers a schema change by recreating the table and copying the rows, so a `NOT NULL` column makes the copy fail and the relay refuse to boot. A row written before the column existed therefore answers with `branch: null`. Recovering it from the `url` is not possible and not attempted: the URL names the pull request, not the branch it came from.

A null branch has two causes and the client treats them the same, deliberately. An assignment has no branch because a *kind* of event has none (`branch` is the shape's third nullable field, after `assignee` and `repository`); a legacy merge row has none because a *row* predates the column. Both render `actor merged into main`, the type-derived phrase that was the only phrase before this change. That fallback is load-bearing: it is what lets the shape grow without a migration of meaning for rows already stored.

`--json` carries `branch` for new events and `null` for legacy rows. The CLI still hands over the relay's bytes verbatim — the shape changed at the relay, not in the client — so the wire contract and the `--json` fixtures move together. The branch takes the palette's **location** role, blue: it is a place, shown the way a path is shown. No truncation is added: long branches wrap at the terminal edge with the rest of the card.

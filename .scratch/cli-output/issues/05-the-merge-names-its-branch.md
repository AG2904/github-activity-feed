# 05: The merge names its branch

**What to build:** The [spec](../spec.md)'s next amendment: a merged event carries the
source branch, and the card names it — `maria.perez merged fix/memory-leak into main`
— instead of deriving `into main` from the type alone. GitHub's webhook already
carries the data (`pull_request.head.ref`); the relay must read it and the wire shape
must carry it, the same way ADR-0010 added the repository: as a nullable field that
degrades honestly for rows stored before it existed.

**Blocked by:** none — 01 to 04 are done.

**Status:** ready-for-agent

- [ ] A merge-to-main event carries the source branch: the relay maps
      `pull_request.head.ref` to `branch`. A payload whose head ref is missing or is
      not a non-empty string still normalizes, with `branch: null` — the branch is
      supplementary and no event is lost over it.
- [ ] `ActivityEvent` gains `branch: string | null` as its ninth field, and the domain
      comment that says "Eight fields, no more" is updated. The entity column is
      nullable for the same SQLite reason as `repository` (ADR-0010): with
      `synchronize: true`, a `NOT NULL` column on a populated table makes TypeORM
      recreate it, the copy fails and the relay refuses to boot. Rows stored before
      the column read `null`.
- [ ] The client parses `branch` as a nullable field. The merged card reads
      `actor merged <branch> into main` when the branch is present, and keeps
      `actor merged into main` when it is null — no empty segment, no doubled
      `into main`.
- [ ] The same card appears in the live feed, the backfill, `history` and the grouped
      report (which still drops only the repository).
- [ ] `--json` carries `branch` for new events and `null` for legacy rows: the wire
      contract and the `--json` fixtures move together; the other fields keep their
      names, order and values. The CLI keeps handing over the relay bytes verbatim.
- [ ] The decision is recorded where the shape is documented: a new ADR following
      ADR-0010's precedent, and a spec amendment that lifts the "Any change to
      `--json`, to the wire format, or to the relay" out-of-scope clause and retires
      the "the event carries no branch" note under Implementation Decisions.
- [ ] `docs/cli-testing.md`, `docs/cli-flow.md` and `docs/end-to-end-testing.md` show
      the phrase with the branch and the legacy fallback; `GLOSSARY.md` names the
      field if the glossary lists the event's fields.
- [ ] Both suites are green, and the pinned fixtures cover a merged card with a
      branch, a merged card with `branch: null`, and the `--json` shape.

## Comments

- 2026-10-22: Asked by Andrés after seeing `AG2904 merged into main` in the feed: a
  developer should learn *which line of work* landed, not only where. The branch is
  the one piece of that sentence GitHub already sends and the relay throws away.
- 2026-10-22: Where the data is and what the change costs. `merge-to-main.ts` reads
  `base.ref` only to detect `main` through `isMainBranch`; `head.ref` is in the
  payload and unread. The change crosses the contract: payload type and
  normalization (relay), domain shape and the SQLite column (nullable — see the
  entity's comment on `repository`), SSE (mechanical, the frame serializes the
  event), client `Event`/`Parse`, renderer, fixtures and docs. `--json` changes by
  definition, so the spec's byte-identical promise is amended, not broken silently.
- 2026-10-22: Rejected alternative — expose `branch` only in `--json`/`history` and
  leave the card alone. It is cheaper (no phrase questions, no render change) but
  answers the storage, not the reader; the feed is the surface that asked for it.
- 2026-10-22: Honest limits, kept in scope but not solved here: branch names can be
  long and noisy (`dependabot/npm_and_yarn/…`) and there is still no wrapping by
  decision — the terminal wraps at the edge as it does today, and the phrase must
  not become a second title. No truncation is added by this ticket.

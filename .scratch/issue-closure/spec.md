# Issue closure — closed critical issues reach the feed

Objective: a closed issue carrying `bug` or `feedback` becomes an ActivityEvent,
split by the reason of the closure, and the CLI shows it. Branch:
`feat/issue-closure`. Delivery strategy: `single-pr`, because the relay and the
CLI share one wire contract and a chain would leave a window where the relay
emits a literal the shipped client cannot render. Runners: `npm test`,
`npm run build`, `npm run lint`, `npm run cli:test`, `npm run cli:lint`.

## Specs

S1. A delivery with `action: "closed"` whose issue carries `bug` or `feedback`
is stored as an event. The labels are read at the moment of the closure, from
`issue.labels`, exactly as the assignment rule reads them today.

S2. The closure is split by GitHub's `issue.state_reason`, because the user asked
to keep the two outcomes apart: `"completed"` (or absent, the plain reading of a
closure) normalizes to `issue.closed`; `"not_planned"` normalizes to
`issue.dismissed`. Any other reported value is read as `issue.closed`: the action
is a closure and the reason is unknown, which is not the same as discarded.

S3. A closed issue without a critical label produces nothing (`not-critical`).

S4. The event carries the nine contract fields and no more: `repository` from
`repository.full_name`, `branch: null`, `actor` from `sender.login`, `subject`
from `issue.title`, `assignee` from the issue's reported assignee (the first of
`issue.assignees`) or `null` when nobody is on it, `labels` flattened to names,
`url` from `issue.html_url`, `when` from `issue.updated_at` as a canonical
instant.

S5. A closure that passes the filter but cannot be normalized (no actor, no
title, no URL, no usable `updated_at`) is refused as `malformed`, and that answer
is terminal for the payload, exactly like the two existing adapters (ADR-0015).

S6. `ActivityEventType` grows from two literals to four. The shape is still nine
fields: no new field, no storage migration, no change to the stream, the history
or the dedupe.

S7. The CLI renders both: `issue.closed` reads `actor closed`, `issue.dismissed`
reads `actor dismissed as not planned`; the labels ride with the title, `--json`
hands over the relay's bytes, and `--user` matches the actor or the assignee as
it does for every other type.

S8. The session's stop line keeps its account honest with the new types:
`stopped · N events · N merged · N assigned · N closed · N dismissed`.

S9. Nothing already stored or already displayed changes: an existing merge or
assignment payload produces the same row and the same card as on the base commit.

S10. The living documentation follows the code: `GLOSSARY.md`, a new ADR,
`docs/relay-flow.md`, `docs/cli-flow.md`, `ARCHITECTURE.md`,
`docs/manual-testing.md`, `docs/end-to-end-testing.md`, and the relay and
cli-output specs. Historical records under `docs/tickets/` are not rewritten.

## Tasks

| ID | Specs | Route | Status | Commit |
| --- | --- | --- | --- | --- |
| T1 | S1–S6, S9, relay half of S10 | inline | done | `f0c3fec` |
| T2 | S7–S9, CLI half of S10 | inline | done | `0d26cfd` |
| T3 | S10, evidence record | inline | done | this commit |

## Log

L1 (2026-10-30, verbatim): "Quiero añaidr la funcionalidad de que los issues
cerrados con los lables que estamos manejados también sean eventos captados y
eventos mostrados en la CLI"

L2 (2026-10-30, verbatim, answer to the closure-scope question): "Manejemos los
dos por separado para dar infroamtcion veraz y contrasable a los usuarios"

L3 (2026-10-30): exploration and decision. The contract's nine fields cannot
carry an outcome without growing a field, and growing it would break ADR-0005's
shape asserted by deep equality; therefore the outcome lives in `type`, which is
already a closed set of literals. `assignee` is kept because the closure of an
issue assigned to someone is that person's event under `--user`, and the payload
reports it truthfully.

L4 (2026-10-30): relay first — the contract literal, the closure adapter, the
composition, the suite and the living relay docs. Then the CLI: the two literals,
the two sentences, the stopped line and its own suite. Relay suite 67/67,
`build`/`lint` clean, `go vet`/`gofmt` clean, client suite green, docs encoding
check clean.

L5 (2026-10-30): differential readback — the existing merge and assignment cases
still compare the whole nine-field row and exact stdout, and pass without an
edit, so the change is additive.

L6 (2026-10-30): evidence record written under
`docs/tickets/issue-closure/01-closed-issues-reach-the-feed.md`.

L7 (2026-10-30): independent verification. All S1–S10 passed; no blockers. The
verifier confirmed the mapping table, the field sources, `malformed` terminality
and the one-reading rule with its own probes, and observed both cards and the
full stop line from the compiled binary. Two advisories are documented, not
corrected: `issue.assignee` wins over `issue.assignees[0]` (the same precedence
the assignment adapter has used since the baseline, and GitHub mirrors the two),
and an empty title still stores (the same `typeof` rule as the other adapters).
Its client stop-line probe was committed as
`client/test/closure_stopline_test.go`. Its adapter-level unit probes were not
committed: the relay has one documented testing seam — the HTTP boundary — and
asserting refusal reasons is the separate seam ADR-0015 leaves for the feature
that first consumes them.

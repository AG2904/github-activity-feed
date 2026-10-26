## Code map

Two halves joined by a wire contract, never by a call path: a NestJS relay that
receives webhooks and a Go CLI that reads its stream. `ARCHITECTURE.md` is the
one-page view of both. The Go module is nested (ADR-0009), so `go` commands run
from `client/` and `npm` commands run from the root.

- **Reaching the relay** — webhook auth, which events are critical, SSE fan-out,
  history: `docs/relay-flow.md`; code in `src/relay/`.
- **The terminal client** — dispatch, `watch`, `history`, rendering: `docs/cli-flow.md`;
  code in `client/internal/{cli,activity,render,sse}/`.
- **Running the two together** — `docs/manual-testing.md` for the relay and a
  signed webhook, `docs/end-to-end-testing.md` for both halves at once, or
  `docs/docker.md` to run both in containers with no local toolchain.

### CodeGraph

Deliberately not indexed. The two halves share a wire contract, not a call graph,
so an index would show almost no edges and replace a `Grep` that already fits in
one read. For a structural question read `ARCHITECTURE.md`, then
`docs/relay-flow.md` or `docs/cli-flow.md` — those are the maps, kept current.
`codegraph init` stays off here.

## Agent skills

### Issue tracker

Issues and specs live as markdown under `.scratch/`. See `docs/agents/issue-tracker.md`.

### Triage labels

Five canonical roles, label string equal to its name. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### Ticket records

Completed tickets get an evidence record under `docs/tickets/<feature-slug>/`. See `docs/agents/ticket-records.md`.

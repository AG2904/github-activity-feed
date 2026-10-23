# 01: CI runs both suites on every push

**What to build:** A GitHub Actions workflow that runs the relay job (`npm ci`,
`npm run build`, `npm run lint`, `npm test`) and the client job (`go build ./...`,
`go vet ./...`, `gofmt -l .`, `go test -count=1 ./...`) on every push and pull
request targeting `main`, with pinned versions and caches.

**Blocked by:** none.

**Status:** ready-for-agent

- [ ] `.github/workflows/ci.yml` exists and triggers on `push` and `pull_request`
      targeting `main`, as two independent jobs.
- [ ] Relay job, on a clean checkout: `npm ci`, `npm run build`, `npm run lint`,
      `npm test` — every step green.
- [ ] Client job, working directory `client/`: `go build ./...`, `go vet ./...`, a
      `gofmt -l .` that FAILS if it prints anything, and `go test -count=1 ./...`.
- [ ] Versions pinned in the workflow: Node 24 and Go 1.27.x (the module declares
      `go 1.27.0`). npm and Go caches configured.
- [ ] A deliberately broken branch (one failing test) shows the job red; the same
      branch without the break shows it green. Both runs are recorded in the ticket.
- [ ] The workflow needs no repository secrets: neither suite talks to GitHub or to a
      real relay.

## Comments

- 2026-10-23: Two jobs because the two contexts share the contract and nothing else,
  and a red relay suite should not hide a red client suite. The commands are the ones
  already run by hand; the workflow exists to stop relying on memory.
- 2026-10-23: `-count=1` is load-bearing. The e2e package builds the binary in
  `TestMain`, so without it Go returns cached results when only `internal/` changed —
  a trap this repository already hit twice.

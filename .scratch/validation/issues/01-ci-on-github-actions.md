# 01: CI runs both suites on every push

**What to build:** A GitHub Actions workflow that runs the relay job (`npm ci`,
`npm run build`, `npm run lint`, `npm test`) and the client job (`go build ./...`,
`go vet ./...`, `gofmt -l .`, `go test -count=1 ./...`) on every push and pull
request targeting `main`, with pinned versions and caches.

**Blocked by:** none.

**Status:** done

- [x] `.github/workflows/ci.yml` exists and triggers on `push` and `pull_request`
      targeting `main`, as two independent jobs.
- [x] Relay job, on a clean checkout: `npm ci`, `npm run build`, `npm run lint`,
      `npm test` â€” every step green.
- [x] Client job, working directory `client/`: `go build ./...`, `go vet ./...`, a
      `gofmt -l .` that FAILS if it prints anything, and `go test -count=1 ./...`.
- [x] Versions pinned in the workflow: Node 24 and Go 1.27.x (the module declares
      `go 1.27.0`). npm and Go caches configured.
- [x] A deliberately broken branch (one failing test) shows the job red; the same
      branch without the break shows it green. Both runs are recorded in the ticket.
- [x] The workflow needs no repository secrets: neither suite talks to GitHub or to a
      real relay.

## Comments

- 2026-10-23: Two jobs because the two contexts share the contract and nothing else,
  and a red relay suite should not hide a red client suite. The commands are the ones
  already run by hand; the workflow exists to stop relying on memory.
- 2026-10-23: `-count=1` is load-bearing. The e2e package builds the binary in
  `TestMain`, so without it Go returns cached results when only `internal/` changed â€”
  a trap this repository already hit twice.
- 2026-10-23: ImplementaciÃ³n terminada y mergeada en `validation`, con el registro de
  evidencia en
  [`docs/tickets/validation/01-ci-on-github-actions.md`](../../../docs/tickets/validation/01-ci-on-github-actions.md).
  Los comandos del workflow corren verdes a mano y el YAML parsea, pero **la corrida
  real de Actions no se ejecutÃ³**: no hay runner local y el AC de rojo/verde sÃ³lo se
  observa al pushear `main`. Por eso el ticket queda `ready-for-human` y no `done`;
  se cierra cuando exista la corrida y su resultado quede en el registro. DesviaciÃ³n
  aceptada: Go se pinnea vÃ­a `go-version-file` (`1.27.0` exacto) en vez de `1.27.x`
  escrito en el workflow, para que el workflow no pueda declarar otra versiÃ³n que la
  del mÃ³dulo.
- 2026-10-24: Done. La primera corrida real existe: **verde** en `main` (run
  `40050526347`, push de `7108150`, 1 m 31 s, relay 22 s y cliente 87 s) y **roja** en
  un PR descartable (run `40051037419`, rama `ci-probe-red` / PR #8, cerrado sin
  merge) fallando exactamente en `Check formatting`, el gate de `gofmt`. Las dos
  corridas quedaron en el registro de evidencia.

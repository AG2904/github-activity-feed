# 01: The binary boots and reaches the relay

**What to build:** A `client/` Go module holding a `feed` binary that resolves where the relay is, knows its own version, and dispatches its two subcommands — plus the test harness every later ticket builds on.

**Status:** done

- [x] `client/` holds a Go module at `github.com/AG2904/github-activity-feed/client`, with `cmd/feed` as the entry point and `go.mod` carrying no `require` directive.
- [x] The relay's location resolves as flag over environment over default: `--relay <url>`, then `RELAY_URL`, then `http://localhost:3000`.
- [x] `feed --version` prints the version injected at build time through `-ldflags`, and nothing else.
- [x] `feed` with no subcommand prints usage and exits 2. `feed watch` and `feed history` are the two subcommands, and neither is implied.
- [x] `go vet ./...` is clean from inside `client/`.
- [x] The test harness compiles the binary, runs it against a fake relay, and asserts on stdout, stderr, and the exit code — and no test imports an internal package.

## Comments

- 2026-10-17: The walking skeleton for Phase 2, mirroring ticket 01 of the relay: the smallest thing that proves the seam. The harness is the deliverable as much as the binary is, because every later ticket is tested through it.
- 2026-10-18: Completed (7 process-seam cases, `go vet` clean, relay suite still 54 green). One commit on `main`: `0d09d6b feat: boot the feed binary and reach the relay (ticket 01)`.

  **What "reaches the relay" means in this ticket.** `watch` prints the resolved stream URL to stderr as a banner, opens `GET /stream`, and discards the frames; `history` opens `GET /history` and discards the body. Neither renders anything yet - ticket 02 owns the parser and the renderer, ticket 07 owns the history shape. The point of both fetches is to prove the configuration seam end to end and to give the harness its two routes.

  **Interim behaviour replaced later.** `watch` exits `1` when the relay is unreachable; ticket 04 replaces that with an indefinite retry. The banner is the observable signal that flag, environment, and default resolution all work: the default case is asserted by reading the banner rather than by claiming port 3000, so the test does not collide with a relay a developer may already have running.

  **Harness.** `TestMain` compiles the binary once with the version injected through `-ldflags`; every case runs the real process against `httptest` fake relays. No test imports `internal/cli`.

# 01: The binary boots and reaches the relay

**What to build:** A `client/` Go module holding a `feed` binary that resolves where the relay is, knows its own version, and dispatches its two subcommands — plus the test harness every later ticket builds on.

**Status:** ready-for-agent

- [ ] `client/` holds a Go module at `github.com/AG2904/github-activity-feed/client`, with `cmd/feed` as the entry point and `go.mod` carrying no `require` directive.
- [ ] The relay's location resolves as flag over environment over default: `--relay <url>`, then `RELAY_URL`, then `http://localhost:3000`.
- [ ] `feed --version` prints the version injected at build time through `-ldflags`, and nothing else.
- [ ] `feed` with no subcommand prints usage and exits 2. `feed watch` and `feed history` are the two subcommands, and neither is implied.
- [ ] `go vet ./...` is clean from inside `client/`.
- [ ] The test harness compiles the binary, runs it against a fake relay, and asserts on stdout, stderr, and the exit code — and no test imports an internal package.

## Comments

- 2026-10-17: The walking skeleton for Phase 2, mirroring ticket 01 of the relay: the smallest thing that proves the seam. The harness is the deliverable as much as the binary is, because every later ticket is tested through it.

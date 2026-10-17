# 08: The binary is distributed

**What to build:** The build entry points and the cross-platform binaries, plus the runbook that proves the CLI against the real relay.

**Blocked by:** 07

**Status:** ready-for-agent

- [ ] `npm run cli:build` builds the binary for the host platform, and `cli:test` and `cli:lint` run the Go suite and `go vet` — each changing directory into `client/` itself.
- [ ] `npm run cli:dist` produces static binaries for windows/amd64, linux/amd64, darwin/amd64, and darwin/arm64, with `CGO_ENABLED=0` and `-trimpath`.
- [ ] Each produced binary reports the version injected at build time.
- [ ] `docs/manual-testing.md` gains a CLI section: how to run the binary against a locally running relay and watch a real event arrive, including the piped case.
- [ ] The architecture document's repository tree and phase plan reflect the client's arrival.

## Comments

- 2026-10-17: npm scripts rather than a Makefile because the repository already speaks npm and `make` is not installed on the machine this is developed on. `goreleaser` is deliberately left to Phase 3: today it would be an extra dependency to build four binaries.

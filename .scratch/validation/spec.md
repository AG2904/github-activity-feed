# Validation and demo — the relay proves itself against real GitHub

Status: ready-for-agent

## Problem Statement

Fases 1 and 2 are complete and in `main`: the relay normalizes and serves the
contract, the CLI paints it. But nothing has *validated* the pair the way a
maintainer would: every suite runs against fixtures that were written by hand
(`test/app.e2e-spec.ts` builds them with `pullRequestPayload`/`issuePayload`), no
workflow runs on push, the repository has no README (`ARCHITECTURE.md` §9), and the
four static binaries only exist on the machine that builds them.

That leaves a known gap written down in the CLI's spec: a fake relay with a frozen
format cannot notice the wire drifting, and more tests do not close it — only running
the real relay with a real GitHub payload does. The published binaries are what the
three-dev team would hold in their hands, and the README is what they would read
first. Today the GitHub homepage points at a README that does not exist.

## Solution

Close the phase with four pieces, in this order, plus one optional deployment:

1. **CI on GitHub Actions** — two jobs on every push and pull request: the relay
   (build, lint, e2e suite) and the client (build, vet, `gofmt`, e2e suite with
   `-count=1`). The checks are the ones already run by hand, so the workflow records
   them instead of inventing them.
2. **A README** — what the system is, how to start the relay and build the CLI, where
   the contract lives, and the runbooks. Spanish, like the rest of the repository's
   documentation.
3. **A tagged release** — a tag builds the four binaries with the existing
   `npm run cli:dist`, injecting the tag as the version, and attaches them to a
   GitHub Release. No goreleaser: the script already does the work and a new tool is
   the dependency the ticket 08 record deliberately postponed.
4. **The demo on real payloads** — capture at least one real merged delivery and one
   real assigned delivery from GitHub, sanitize them, and replay them against the
   relay with the documented signature. Record the evidence. This is what closes the
   frozen-fixture gap.
5. *Optional:* deploy the relay to the internal server and run it with the team.

## User Stories

1. As a developer, I want every push and pull request to run both suites, so that a
   red branch is caught before review.
2. As a teammate, I want a README that tells me what this is and how to run it, so
   that I do not have to read the architecture document first.
3. As a developer, I want a tagged release with the four binaries attached, so that
   the demo does not depend on my machine.
4. As a developer, I want the relay and CLI validated with a real GitHub payload, so
   that the frozen fake relay stops being the only evidence.
5. As the maintainer, I want the demo's outputs recorded as evidence, so that the
   phase is closed with artifacts and not with a memory.
6. *Optional:* as the team, I want the relay reachable on the internal network, so
   that we can use the feed for a week.

## Implementation Decisions

- **CI is two independent jobs**, because the two contexts share a contract and
  nothing else: the relay job runs `npm ci`, `npm run build`, `npm run lint` and
  `npm test`; the client job runs `go build ./...`, `go vet ./...`, a `gofmt -l` that
  must print nothing, and `go test -count=1 ./...`. `-count=1` is not decoration: the
  e2e package builds the binary in `TestMain`, and without it Go returns cached
  results when only `internal/` changed — a trap this repository already hit.
- **Versions are pinned in the workflow**, not left to the runner's default: Node 24
  (the version the types target) and Go 1.27.x (the module's `go 1.27.0`). The
  workflow caches npm and the Go module/build caches, and runs on `push` and
  `pull_request` targeting `main`.
- **The release workflow is tag-driven and reuses `cli:dist`**: checkout, Node, `npm
  ci`, `npm run cli:dist` with the tag as the version, then attach the four files to
  the GitHub Release using the runner's `gh` with `GITHUB_TOKEN`. No goreleaser, no
  packaging, no installers.
- **The demo is a runbook, not a new test.** The captured payloads are evidence
  inputs: they are replayed with the real signature against the real relay, and the
  outputs (stdout, stderr, stored rows) go into the ticket record. Adding them to the
  suite would freeze them again and re-create the gap the demo exists to close.
- **The README is the public face, in Spanish**, consistent with `ARCHITECTURE.md`
  and the runbooks. It links to the contract instead of duplicating it.
- **The optional deployment is a separate ticket with `Status: ready-for-human`**: it
  needs a server and credentials a maintainer holds, and the phase closes without it.
- **The instance question is a prerequisite, not code**: `.scratch/relay/spec.md`
  says the instance that will send the webhooks must be confirmed before this phase.
  The demo ticket records the answer (corporate instance or `github.com`) before its
  evidence is written.

## Testing Decisions

- **CI proves itself by running**: the workflow is correct when a pull request shows
  both jobs green and a deliberately broken branch shows them red. No separate test
  suite for YAML.
- **The release is verified by downloading**: fetch one binary from the Release page,
  run `--version`, and compare with the tag. The binaries are also run once on their
  platform (the maintainer's machine covers `windows/amd64`; the demo covers the rest
  if the team uses another OS).
- **The demo's evidence is recorded, not asserted**: exact commands, payloads
  (sanitized), and outputs under `docs/tickets/validation/`. A transcript with invented
  output is not evidence.
- **The README is verified by following it**: a clean shell, from the README alone,
  must reach a running relay and a built client.

## Out of Scope

- Deployment automation, containers, reverse proxies, TLS termination, uptime.
- goreleaser, installers, package managers, Homebrew, Docker images.
- Changing the event contract, the filter, the render, or the wire format.
- Multi-user auth, permissions, or any feature the phase does not need.
- Monitoring, metrics, log aggregation, or alerting.
- The three-dev rollout itself; the optional deployment ticket ends at "reachable and
  usable", not at "adopted".

## Further Notes

- **Why CI now and not earlier.** Both suites exist and are green (relay 57/57, client
  60 cases), and every change so far was verified by hand. The workflow is the same
  list written down, so the cost is small and the drift it prevents is the kind this
  project already suffered in prose (the "eight fields" sweep).
- **Why the demo is the only honest close for the frozen-fixture gap.** The client's
  fake relay speaks a format captured once; a wire change would pass every test and
  fail against the real relay. Only a real delivery exercises the contract end to
  end.
- **Why the README is part of validation.** `package.json` points its homepage at
  `README.md`; the file never existed. A phase that ends with binaries and CI but no
  front door is still a private project.
- **What "done" means for the phase**: CI green on `main`, a release whose artifacts
  download and run, a README a stranger can follow, and a recorded demo with real
  payloads. The optional deployment does not gate it.

## Comments

- 2026-10-23: Spec opened after the maintainer asked whether the project was ready
  for Phase 3. It is not: `.github/` does not exist, `README.md` does not exist, the
  binaries are only built locally, and the relay's tests use constructed payloads.
  The phase is scoped to close exactly those gaps, with the deployment kept optional
  as `ARCHITECTURE.md` §7 already said.

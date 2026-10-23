# 03: A tag releases the four binaries

**What to build:** A tag-driven GitHub Actions workflow that builds the four static
binaries with the existing `npm run cli:dist`, injects the tag as the version, and
attaches them to a GitHub Release. No goreleaser, no installers.

**Blocked by:** none.

**Status:** ready-for-agent

- [ ] `.github/workflows/release.yml` triggers on tags `v*` (a release is cut by
      pushing a tag, never by hand-uploading files).
- [ ] The workflow checks out, pins Node 24, runs `npm ci`, builds with
      `npm run cli:dist` (or the equivalent invocation) and passes the tag as the
      version — `package.json`'s version is not hand-edited for a release.
- [ ] The four artifacts produced by the script (`windows/amd64`, `linux/amd64`,
      `darwin/amd64`, `darwin/arm64`) are attached to the GitHub Release created for
      the tag, using the runner's `gh` and `GITHUB_TOKEN`.
- [ ] A failed build fails the workflow; no partial release is published.
- [ ] Verification: pushing a test tag produces a Release with four downloadable
      assets; a downloaded binary answers `--version` with the tag (not `dev`), and
      runs on its platform. The transcript goes in the ticket record.

## Comments

- 2026-10-23: `npm run cli:dist` already builds the four targets with `CGO_ENABLED=0`,
  `-trimpath` and the version injected by `-ldflags`; the ticket 08 record
  deliberately left goreleaser to this phase. Reusing the script keeps one build path
  instead of two, and adds no dependency.
- 2026-10-23: The version comes from the tag, not from `package.json` — bumping the
  package for every release is a step someone forgets, and the binary would then lie
  about where it came from.

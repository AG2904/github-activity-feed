# 02: The README is the front door

**What to build:** A README at the repository root that explains what the system is,
how to start the relay, how to build and run the CLI, where the event contract lives,
and where the runbooks are. Spanish, like the rest of the repository's documentation.

**Blocked by:** none.

**Status:** ready-for-agent

- [ ] `README.md` exists at the root and opens with what the project is and who it is
      for: a relay that turns critical GitHub deliveries into an activity feed, and a
      CLI that shows them in the terminal.
- [ ] A quick start reaches a running relay from a clean machine: copy
      `.env.example` to `.env`, set `GITHUB_WEBHOOK_SECRET`, `npm ci`,
      `npm start` (relay on port 3000, or `$PORT`).
- [ ] A quick start builds and runs the CLI: `npm run cli:build` (or `go build` in
      `client/`), `./feed watch --relay <url>`, and what to expect on stdout and
      stderr.
- [ ] The contract is linked, not duplicated: `ARCHITECTURE.md` §6 and the runbooks in
      `docs/` (`cli-testing.md`, `end-to-end-testing.md`, `exposing-the-relay.md`).
- [ ] The phase status is honest: Fases 1 and 2 complete, Fase 3 in progress, and
      what is out of scope.
- [ ] Every command in the README was run as written on the documented platform; the
      outputs shown are real, not invented. The links resolve.

## Comments

- 2026-10-23: `package.json` points its `homepage` at `README.md` and the file never
  existed; `ARCHITECTURE.md` §9 records it as pending. This ticket closes that and
  nothing else — no badges, no marketing.
- 2026-10-23: The README is Spanish because every other document in the repository is
  (the architecture, the runbooks, the ticket records); code and code comments stay
  English.

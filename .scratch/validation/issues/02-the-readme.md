# 02: The README is the front door

**What to build:** A README at the repository root that explains what the system is,
how to start the relay, how to build and run the CLI, where the event contract lives,
and where the runbooks are. Spanish, like the rest of the repository's documentation.

**Blocked by:** none.

**Status:** done

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
- 2026-10-23: Done. El README está en `validation`, con el registro de evidencia en
  [`docs/tickets/validation/02-the-readme.md`](../../../docs/tickets/validation/02-the-readme.md).
  Se verificó siguiéndolo al pie de la letra: relay con `--env-file`, los dos caminos
  de build y ejecución, el banner con `/stream`, y `/history` → `[]`. Dos diferencias
  con el texto del ticket, documentadas en el propio README: `npm start` a secas no
  lee `.env` (el relay lee variables de entorno, no el archivo), y en Windows
  `go build -o feed` produce un archivo sin extensión que no se ejecuta; el README
  usa `--env-file .env` y `-o feed.exe`.

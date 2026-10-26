# 04: The demo runs on real GitHub payloads

**What to build:** Capture at least one real merged delivery and one real assigned
delivery from the confirmed GitHub instance, sanitize them, replay them against the
real relay with the documented signature, and record the evidence. This closes the
CLI spec's known gap: a frozen fake relay cannot notice the wire drifting.

**Blocked by:** 03 (the demo uses the released binaries). Also needs one human
decision first: which GitHub instance sends the webhooks (the corporate instance or
`github.com`) — `.scratch/relay/spec.md` requires it confirmed before this phase.

**Status:** done

- [x] The GitHub instance that will send webhooks is recorded in the ticket before
      any capture (a dated comment), and the captured payloads come from that
      instance.
- [x] At least two real deliveries are captured — one merge into `main`, one issue
      assignment with labels — sanitized of secrets, tokens, internal URLs and
      personal data the feed does not need, and stored with a short note saying what
      each is and where it came from.
- [x] The payloads are replayed against the real relay with the correct
      `X-Hub-Signature-256` (the documented manual flow), and the relay accepts,
      stores and serves them (`GET /history`, `GET /stream`).
- [x] The client renders the real payloads: the merged card says
      `actor merged <branch> into main` with its labels, and the assigned card says
      `actor assigned to assignee`; `--json` hands over the relay's bytes verbatim.
- [x] Optionally, a real webhook is registered through the tunnel flow in
      `docs/end-to-end-testing.md` (step 6) — if used, the delivery id and what arrived
      are recorded.
- [x] The evidence record under `docs/tickets/validation/` quotes exact commands and
      real outputs, names the CLI spec's known gap as closed, and says what was not
      verified.
- [x] No new test is added for the captured payloads: they are evidence inputs, and
      freezing them in the suite would re-create the gap this ticket closes.

## Comments

- 2026-10-23: The relay's e2e fixtures are constructed (`pullRequestPayload`,
  `issuePayload` in `test/app.e2e-spec.ts`), and the CLI's suite speaks to a fake relay
  with a frozen format. Both are good tests and neither can catch a wire change — the
  demo is the only validation that can.
- 2026-10-24: The instance is confirmed: **github.com**. The repository lives there,
  and the demo's captures will come from a real delivery of a github.com repository;
  no Enterprise Server is in the picture, so the relay can be exposed with the
  documented tunnel and no corporate-network allow-listing is needed.
- 2026-10-24: Done. El demo se ejecutó sobre payloads reales de `github.com` con un
  repo descartable y está en `main`, con el registro en
  [`docs/tickets/validation/04-the-demo-on-real-payloads.md`](../../../docs/tickets/validation/04-the-demo-on-real-payloads.md).
  El webhook apuntaba primero a `/webhook` y GitHub registró `404`; tras corregirlo a
  la raíz del túnel, las entregas devolvieron `200` y el relay almacenó el merge y la
  asignación. Los payloads quedaron en `docs/demo/` —sin secretos, tokens ni correos—,
  el replay firmado devolvió `200`, y `feed history --json`, el reporte agrupado y
  `watch` mostraron las tarjetas. El repo de demo y el webhook se borraron después. El
  "known gap" del spec del CLI quedó marcado como cerrado con nota fechada.

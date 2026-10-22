# 03: The session says what it is doing

**What to build:** The [spec](../spec.md)'s session chrome: `watch` opens with a banner
that names its filters, confirms the first connection in words, and closes with one
line of account. Three lines, all on stderr, all plain text.

**Blocked by:** none — 01 and 02 are done.

**Status:** done

- [ ] On start, the banner reads `watching <url> · backfill <n>`, with ` · user <login>`
      and ` · repo <owner/name>` appended — in that order — when the flags are set.
- [ ] After the first connection is accepted, stderr carries exactly one line:
      `connected · <n> events from history` when the backfill printed events,
      `connected · waiting for events` when it printed none. `<n>` counts what was
      actually printed, after the filter.
- [ ] A later reconnect keeps `watch: reconnected` and does not print a second
      `connected` line.
- [ ] On exit — every path out of `watch`, Ctrl-C included — stderr carries
      `stopped · <n> events · <k> merged · <l> assigned` when any event was shown, and
      `stopped · no events` otherwise. The counts are of shown events, filter included.
      Events merged and assigned are counted by type; other types appear in no
      category but do count in `<n>`.
- [ ] No chrome line ever reaches stdout, and none carries an escape sequence: the
      exact stdout fixtures keep passing unchanged, and the `--json` output stays
      byte-identical.
- [ ] There is no heartbeat, no spinner, and no repeating line: the only affirmative
      line is the first connection's, and silence afterwards stays silent.
- [ ] `docs/cli-testing.md`, `docs/cli-flow.md` and `docs/end-to-end-testing.md` show
      the three lines and say where they go.

## Comments

- 2026-10-22: The chrome is **not** TTY-gated, unlike `gh run watch`'s epilogue. The
  reasoning is family, not taste: today's `watching <url>` already prints
  unconditionally, and every chrome line is one sentence of state that a redirected
  log deserves as much as a terminal does. What must never happen is stdout leakage or
  an escape — messaging is stderr's job by clig.dev and by our own `Run` doc comment.
- 2026-10-22: The first-connection line is the one moment where "is it alive?" has no
  other reply. Every later silence is already answered: a dead relay speaks through the
  reconnect notice, and a healthy quiet one leaves the last card's time to say how long
  ago it was. A periodic heartbeat was refused because repetition on an append-only
  terminal is permanent ink — `gh` can afford its refresh line because it redraws an
  alternate screen where repetition is invisible.
- 2026-10-22: Counts are of *shown* events. A filter that discards an event also
  discards it from the account, because the summary answers "what did I see", not
  "what did the relay send".
- 2026-10-22: Done. El chrome de sesión está en `cli-output-03-04`, con el registro de
  evidencia en
  [`docs/tickets/cli-output/03-the-session-says-what-it-is-doing.md`](../../../docs/tickets/cli-output/03-the-session-says-what-it-is-doing.md).
  Cliente en verde (`go test -count=1 ./...`, 47.3 s) y relay 55/55. Dos cosas que
  aportó la revisión de dos ejes: el gating de la línea `connected` estaba sobre
  intentos fallidos y no sobre la primera conexión *aceptada* —un relay que arranca
  caído nunca recibía la línea afirmativa—, y la prueba de la cuenta de salida sólo
  corría en Windows; ahora `endSession` tiene rama por plataforma (pipe roto en
  Windows, `os.Interrupt` en Unix). La rama no-Windows queda **sin ejecutar en esta
  máquina** (no hay WSL): falta un `go test ./...` en Unix. El ejemplo del banner en
  el spec se corrigió para coincidir con su prosa y con este ticket.

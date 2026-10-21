# 01: The feed reads as a card

**What to build:** Replace the two-line log output with the card the
[spec](../spec.md) describes — a sentence, the title, the context — including labels,
human relative time and the blank line between events.

**Status:** done

- [ ] A merge renders three lines: `actor merged into main`, the title on its own line,
      and `repository · [labels] · time`.
- [ ] An assignment renders `actor assigned to assignee`, and `assigned` alone when the
      event carries no assignee.
- [ ] The context line omits a segment the event does not carry — no repository for a
      legacy row, no brackets when the label list is empty — instead of printing an
      empty one.
- [ ] Labels appear between brackets and comma-separated: `[bug]`, `[bug, feedback]`.
- [ ] Relative time says `yesterday` between 24 and 48 hours, and `Nd ago` beyond that.
- [ ] Without colour the URL is a fourth indented line; with colour the title is the
      OSC 8 link and no URL line is printed.
- [ ] Consecutive cards are separated by exactly one blank line, with none before the
      first and none after the last; `--json` gets no blank lines.
- [ ] The live feed, the backfill and `history` print the same card; the grouped report
      keeps its repository heading and drops the repository segment from each card.
- [ ] `--json` output is byte-identical to what it was before the change, and the relay
      is untouched.
- [ ] The pinned fixtures in `client/test/e2e_test.go`, the expected outputs in
      `docs/cli-testing.md` and the render description in `docs/cli-flow.md` describe
      the new output; any sample in `docs/end-to-end-testing.md` follows.

## Comments

- 2026-10-21: The complaint that opened this ticket was not aesthetic. The relay exists
  so a developer does not learn about a merge by email; an output that reads like a log
  line keeps the interruptiveness of a notification and gives up its legibility. The
  card answers what/who/where/why in the order a person reads, and the label — the
  reason an assignment interrupted you — stops being parsed and discarded.
- 2026-10-21: Two implementation consequences worth pinning before coding. The blank
  line belongs to the emitter, not the renderer: the renderer paints one card and stays
  stateless, which is what keeps the four call sites (live, backfill, history, grouped)
  in agreement, and it is also how the last card avoids leaving a gap under the prompt.
  And `into main` is derived from the type, because the event carries no branch; the
  spec records the condition under which that stops being honest.
- 2026-10-21: Done. La tarjeta, las etiquetas, `yesterday` y la separación están en
  `main`, con el registro de evidencia en
  [`docs/tickets/cli-output/01-the-feed-reads-as-a-card.md`](../../../docs/tickets/cli-output/01-the-feed-reads-as-a-card.md).
  La suite del cliente quedó en verde (`go test ./...`, 38.2 s) y el relay en 55/55. El
  ticket 02 toma desde acá: este cambio no trae la paleta por rol, sólo el peso del
  título y su hyperlink.

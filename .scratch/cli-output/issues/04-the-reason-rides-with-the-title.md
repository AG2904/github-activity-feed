# 04: The reason rides with the title

**What to build:** The [spec](../spec.md) amendment that moves the labels out of the
context line and onto the front of the title: `[bug] Fix memory leak in parser
component`. The *why* arrives before the *what*.

**Blocked by:** none — 01 and 02 are done.

**Status:** done

- [ ] Line two of the card is `[labels] title` — the labels the event carries, in
      brackets, separated from the title by exactly one space, in the order the event
      sends them (`[bug, feedback]`).
- [ ] A card whose event has no labels starts line two with the title: no empty `[]`,
      no leading space, no extra blank line.
- [ ] The context line loses its label segment and keeps `repository · time`, each
      still omitted when the event does not carry it; a legacy event with
      `repository: null` renders a context line of just the time.
- [ ] With colour on, the labels stay bold red and the title stays bold and remains
      the OSC 8 hyperlink; the raw URL line still appears only when hyperlinks are
      unavailable.
- [ ] The same change lands in the grouped report's cards, which drop the repository
      and keep `[labels] title` on their line two.
- [ ] The colourless fixtures move with the shape — `[bug]` now precedes the title in
      the pinned strings — and nothing else about the output changes; `--json` stays
      byte-identical.
- [ ] `docs/cli-testing.md`, `docs/cli-flow.md` and `docs/end-to-end-testing.md` show
      the card with the labels before the title.

## Comments

- 2026-10-22: The label is the only segment that answers *why*. The context line is
  where the eye finishes, not where it decides — so a reason parked there is a reason
  scanned past. cargo puts the qualifier next to the object (`Compiling serde
  v1.0.203`), compose puts the source prefix on the content line; both teach the same
  lesson: the qualifier rides with the content, not with the footer.
- 2026-10-22: User story 3 gets stronger, not weaker: "the label reads immediately
  before the thing it qualifies" is a stricter promise than "the label is visible
  somewhere on the card".
- 2026-10-22: The red-bold label sitting next to the bold title was checked against
  conhost: the reset between the two segments keeps them distinct, so neither drops
  its weight. This is the one place the palette table's rows are adjacent in the
  output rather than separated by default text, and it survives.
- 2026-10-22: Done. Las etiquetas viajan con el título en `cli-output-03-04`, con el
  registro de evidencia en
  [`docs/tickets/cli-output/04-the-reason-rides-with-the-title.md`](../../../docs/tickets/cli-output/04-the-reason-rides-with-the-title.md).
  Cliente en verde (`go test -count=1 ./...`, 47.3 s) y relay 55/55. La revisión de
  dos ejes agregó el pin que faltaba —la tarjeta *merged* con etiquetas, que el spec
  pinea y su ejemplo muestra— y movió los literales `pr.merged` / `issue.assigned` a
  constantes compartidas en `activity`, porque ahora se usan en `render` y en `watch`.
  Los colores reales quedan con la misma frontera que el ticket 02: verificación de
  bytes y paso manual en `docs/cli-testing.md`.

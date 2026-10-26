# 06: The palette settles — soft roles, one reason

**What to build:** Replace the loud, bold role palette with the one the maintainer
chose from the prototype (`prototype/palette-options`, `ea100ee`, variant
`soft reason`): regular-weight ANSI hues, the repository dropping to grey, the labels
moving from bold red to magenta, the branch keeping the blue location role, and the
title keeping its bold + OSC 8 weight. Still sixteen colours: the choice is which
slots, not new ones.

**Blocked by:** none — 01 to 05 are done.

**Status:** done

- [ ] The spec's palette table is replaced by the chosen slots — actor and assignee
      `36`, `merged` and `main` `32`, `assigned` `33`, branch `34`, repository and the
      `·`/time `90`, labels `35`, title `1` + OSC 8 — with a dated amendment comment
      naming the prototype branch and commit, and saying why the old table (four bold
      hues, blue repository, red labels) was retired.
- [ ] `render.go` paints the new slots and names the constants honestly; no `1;3x`
      colour code remains — bold survives only on the title — and `paint()` stays the
      only door a coloured segment goes through.
- [ ] With colour off the output is byte-identical to today: every pinned fixture
      passes without being touched.
- [ ] A byte check (throwaway program; a pipe has no TTY) shows exactly these codes:
      `36` actor and assignee, `32` `merged` and `main`, `33` `assigned`, `34` branch,
      `90` repository and the connector/time, `35` labels, `1` plus the OSC 8 pair for
      the title — and no `1;3x` anywhere.
- [ ] `docs/cli-testing.md` (role table and the manual step) and `docs/cli-flow.md`
      (palette description) describe the new mapping; any colour sample in the docs
      moves with it.
- [ ] The manual pass is written for Windows Terminal **and** a conhost with VT: the
      suite can only assert the absence of escapes, and the terminal theme paints the
      actual RGB.
- [ ] The client suite is green; the relay is untouched and is not part of this change.

## Comments

- 2026-10-23: Asked by Andrés after living with the feed: the palette felt too
  strong. Diagnosis: four of the six roles used bold codes (`1;3x`), which most
  terminals render as *bright*, and the bold-red labels were the loudest element in the
  card. The choice was made by eye from the prototype branch `prototype/palette-options`
  (commit `ea100ee`), variant `soft reason`: regular weights, grey repository,
  magenta reason, blue branch.
- 2026-10-23: What stays and why: 16 colours still (the binary ships to terminals
  that are not Windows Terminal, ADR-0008's spirit), colour is still never the only
  signal, and the title keeps bold + OSC 8. What changes is the mapping: the two
  states keep green and yellow, the people keep cyan, the branch keeps the location
  blue, the repository becomes context grey, and the reason moves red → magenta, so
  the reason stays warm without red's alarm.
- 2026-10-23: Rejected alternatives, all visible in the prototype: drop-bold-only
  (`soft`, barely moves), two hues (`quieter`, loses the people), one accent
  (`minimal`, loses the states), bright slots (`bright`, theme-dependent), and 256
  colours (exact hues, but it breaks the portability decision and would need its own
  amendment).
- 2026-10-23: Honest limit: the suite cannot assert colour — a pipe has no TTY. The
  bytes are checked with a throwaway program and the hue pass stays manual in two
  consoles, exactly as ticket 02 established.
- 2026-10-23: Done. La paleta `soft reason` está en `cli-output-06`, con el registro
  de evidencia en
  [`docs/tickets/cli-output/06-the-palette-settles.md`](../../../docs/tickets/cli-output/06-the-palette-settles.md).
  Cliente en verde (`go test -count=1 ./...`, 45.8 s) y el relay sin cambios. La
  revisión de dos ejes encontró prosa que no coincidía con los slots —el comentario de
  roles, `amber` contra `yellow`, las menciones de "la única nota cálida" y la frase
  del ticket 05 que ubicaba la rama "en el rol de lugar del repositorio"—; todo
  corregido en `6f0290e`. El tono real queda para el paso manual: Windows Terminal y
  conhost con VT. La rama del prototipo se conserva como registro
  (`prototype/palette-options`, `ea100ee`).

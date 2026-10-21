# 02: Colour tells the roles apart

**What to build:** The palette the [spec](../spec.md) maps, applied to the card: people
in cyan, the two states in green and amber, the place in blue, the reason in red, and
the connective tissue in dim — 16 colours, only on a terminal.

**Blocked by:** 01

**Status:** done

- [ ] Actor in bold cyan; `merged` in bold green; `assigned` in bold amber; `main` in
      bold green; the assignee in bold cyan.
- [ ] The connectors `into` and `to` are dim; the repository is blue; the labels are
      bold red; the separators and the relative time are dim; the title is bold.
- [ ] The title remains the OSC 8 hyperlink, and the raw URL line still disappears when
      colour is on.
- [ ] No escape sequence is emitted when stdout is not a terminal, when `--no-color` is
      passed, or when `NO_COLOR` is set — the colourless fixtures keep passing unchanged.
- [ ] Only 16-colour SGR codes are used: no 256-colour, no truecolour, no emoji, no box
      drawing, no reverse video.
- [ ] `docs/cli-testing.md` records how to check the palette by hand in Windows Terminal
      and in a legacy console with VT enabled, and states what the automated suite can
      and cannot assert about colour.

## Comments

- 2026-10-21: The palette encodes roles, not elements. Green means "something reached
  main", amber means "work landed on someone", cyan is a person, blue is a place, red is
  the reason. If a future element needs a colour it needs a role first; otherwise it
  inherits dim.
- 2026-10-21: 16 colours is a deliberate ceiling, not a limitation of the machine: the
  binary is distributed to terminals that are not Windows Terminal, and every code in
  this palette works wherever VT works. The suite can only prove the absence of escapes
  through a pipe; the hues themselves are a manual check, and the ticket says so instead
  of pretending the automation covers it.
- 2026-10-21: Colour is reinforcement, never the only signal. `merged` and `assigned`
  are different words, the repository contains a slash, the labels sit in brackets. A
  reader who loses colour loses emphasis and nothing else — which is also why the first
  ticket ships and stays readable without this one.
- 2026-10-21: Done. La paleta está en `main` con el registro de evidencia en
  [`docs/tickets/cli-output/02-colour-tells-the-roles-apart.md`](../../../docs/tickets/cli-output/02-colour-tells-the-roles-apart.md).
  Verificación de bytes (un programa temporal con `Color = true`), suite del cliente en
  verde (38.3 s) y relay 55/55. Dos ajustes que salieron de la revisión: el gris es
  `90` y no `faint` porque conhost con VT ignora el atributo 2, y un tipo de evento
  desconocido hereda gris en vez de tomar prestado el ámbar de la asignación.

# 03: `--json` hands the contract over untouched

**What to build:** A raw output mode that writes the relay's own bytes instead of a rendered form, so the CLI is scriptable without a second parser for the shape.

**Blocked by:** 02

**Status:** done

- [x] `feed watch --json` writes one line per event: the frame's `data:` payload, byte for byte, with no re-serialization.
- [x] The emitted line preserves the eight keys in the relay's order, `assignee: null` for an event that has none, and the `.000Z` timestamp form.
- [x] `feed history --json` writes the relay's response body verbatim.
- [x] `--json` combined with `--group-by-repo` exits 2 with a message on stderr: grouping is presentation, and raw output has none.
- [x] `--json` suppresses colour and hyperlinks regardless of whether stdout is a terminal.

## Comments

- 2026-10-17: Small ticket, sharp invariant. Re-marshalling through Go's `encoding/json` would reorder keys and could re-shape `null`, which is exactly the drift user story 18 exists to prevent. Passing the bytes through makes that impossible rather than unlikely.
- 2026-10-18: Completed (13 process-seam cases, `go vet`/`gofmt` clean, linux/darwin builds clean, relay suite still 54 green). Two commits on `main`: `06e802c feat: hand the contract over untouched with --json (ticket 03)` and `83c021b refactor: give the watch loop an emit seam (ticket 03)`, el segundo con las correcciones de la revisión.

  **Passthrough validado, no ciego.** `--json` sigue pasando cada payload por `activity.Parse` antes de escribirlo: el modo raw cambia la forma de la salida, no la regla de qué cuenta como evento. El payload se escribe tal cual más un `\n`; no se re-marshalea nada. El fixture de test incluye un `&`, que Go escaparía como `\u0026` si alguien re-serializara, y `history --json` se compara byte a byte contra el cuerpo del relay, sin agregar salto de línea.

  **Conflicto antes de la red.** `--json` + `--group-by-repo` sale con 2 y un mensaje en stderr sin llegar a pedir `/history`. `--group-by-repo` existe desde este ticket porque el criterio lo exige, pero todavía no agrupa: eso es el ticket 07.

  **Revisión.** Se extrajo `emit` para que el loop de `watch` no mezcle transporte, parseo y dos políticas de salida (el próximo ticket lo vuelve a tocar), se protegió el cuerpo de `/history` del fake relay con el mutex del harness, y el caso JSON ahora descarta también frames `id:`-only, comentario, `event:`-only, `retry:` y JSON truncado, no sólo un cuerpo que no es JSON.

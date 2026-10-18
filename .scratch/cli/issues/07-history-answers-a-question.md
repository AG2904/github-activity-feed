# 07: `feed history` answers a question

**What to build:** A one-shot query against the relay's audit history, so a developer can reconstruct what a teammate merged or was assigned after the fact.

**Blocked by:** 05

**Status:** done

- [x] `feed history` fetches the stored history once, prints it, and exits 0.
- [x] With no filter it lists every stored event, in the order the relay serves it — newest first.
- [x] `--user` and `--repo` filter it, with the same meaning they carry on the live stream.
- [x] `--group-by-repo` prints one header per repository, in alphabetical order, keeping the relay's order inside each group and dropping the repository from the per-event lines.
- [x] An unreachable relay exits 1 with a message on stderr.
- [x] An empty history prints nothing and exits 0.

## Comments

- 2026-10-17: This is the `feed --user` the glossary and the architecture document have promised since before the relay existed, and the reason the relay serves one shape for both the stream and the history (user story 18). Grouping lives here and only here: a live stream is unbounded and unordered, so grouping is only meaningful over a finite list.
- 2026-10-18: Completed (46 process-seam cases, `go vet`/`gofmt` clean, windows/linux/darwin builds clean, relay suite still 54 green). Dos commits en `main`: `576b78f feat: answer the history the relay stores (ticket 07)` y el de las correcciones de la revisión de dos ejes. El registro de evidencia está en `docs/tickets/cli/07-history-answers-a-question.md`.

  **Un solo filtro y una sola forma.** `history` arma su `emitter` con `activity.Filter` y usa `shown`, la misma regla de aceptación que pide el stream; no hay una segunda definición de Involvement ni un segundo renderer. El reporte plano imprime en el orden del relay (más nuevo primero), porque un reporte se lee de arriba hacia abajo — al revés que el backfill del ticket 06, que es la cabeza de un stream.

  **`--json` ahora filtra, y eso fue un defecto encontrado en la revisión.** `history --json --user me` descargaba el historial de todos: el `io.Copy` del cuerpo pasaba antes que cualquier filtro. El ticket 05 ya había fijado que en el stream el filtro corre antes del payload crudo; acá se hizo lo mismo. El contrato "bytes del relay" se mantiene evento por evento (orden de claves, `assignee: null`, `.000Z`, ampersand literal) y el array es el sobre que agrega la CLI: sin filtro coincide con el cuerpo servido, que es lo que sigue pinneando el test del ticket 03.

  **Agrupar es presentación, así que vive en `render`.** Orden alfabético, orden del relay adentro de cada grupo, repositorio afuera de las líneas por evento y una línea en blanco entre grupos. Una fila sin repositorio no se puede agrupar y el spec prohíbe inventarle un nombre ("groups under nothing"), así que va última, sin header; perderla sería peor, porque es un reporte de auditoría.

  **La revisión.** El eje de Spec encontró el bug de `--json` y que el test del orden alfabético era tautológico (el fixture ya venía ordenado; se reconstruyó al revés y se verificó quitando el `SortFunc`). El eje de Standards pidió `newEmitter` para no duplicar la política y `hasRepository` para el predicado repetido. La segunda vuelta cerró los tres que quedaban: el reporte agrupado pasó a `emitter.group` para que todo byte de stdout salga del `emitter`, el `bool` de `line` se volvió `lineShape` con `withRepository`/`withoutRepository`, y `render` se partió en dos archivos —la línea del feed y el reporte— para que cada uno cambie por una sola razón. Se mantuvieron, con motivo, la línea en blanco entre grupos y la negrita del header (forma del reporte), el bloque sin header de las filas sin repositorio (el AC1 pide listar **todos** los eventos y el AC4 un header **por repositorio**: una fila sin repositorio no es un repositorio y no se puede perder) y el test del cuerpo ilegible (la rama de error existe en las tres salidas).

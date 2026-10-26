# 06: The first connection does not start blank

**What to build:** A backfill that fills the terminal on the first connection, so a developer can tell the CLI works before the next critical event happens.

**Blocked by:** 05

**Status:** done

- [x] On the first connection the CLI queries the stored history, prints the most recent `--backfill` events, and then follows the stream.
- [x] `--backfill N` defaults to 10; `--backfill 0` disables it, and the CLI prints nothing until a live event arrives.
- [x] The backfilled events honour `--user` and `--repo` exactly as the live stream does.
- [x] The backfill is flat and ignores `--group-by-repo`.
- [x] A reconnection never re-runs the backfill.
- [x] An empty history is not an error: the CLI prints nothing and follows the stream.

## Comments

- 2026-10-17: The relay replays nothing on connect, so without this the terminal stays empty until the next merge into `main` — possibly days — and the developer cannot tell a working CLI from a broken one. Running it on the first connection only is the point: a backfill on every reconnect would reprint history and undo the dedupe the relay went to the trouble of guaranteeing.
- 2026-10-18: Completed (36 process-seam cases, `go vet`/`gofmt` clean, windows/linux/darwin builds clean, relay suite still 54 green). Cuatro commits en `main`: `c5daf20 feat: fill the feed from the stored history (ticket 06)`, `b9f4d6d refactor: share the relay read path (ticket 06)` y `refactor: read the history before the stream opens (ticket 06)`, los dos segundos con las correcciones de las dos rondas de revisión. El registro de evidencia está en `docs/tickets/cli/06-first-connection-is-not-blank.md`.

  **El backfill es un paso del watch, no un comando nuevo.** Pide `/history` antes de abrir el stream y usa la misma regla de aceptación y el mismo `paint` que el stream: `emitter.accept` decide `(event, valid, shown)` en un solo lugar, así que el historial y lo vivo no pueden divergir en `--user`, `--repo`, `--json` ni color. `--json` entrega los bytes del relay, un payload por línea, así que `jq` lee lo mismo antes y después del primer evento.

  **Dos decisiones que el spec no fijaba y quedaron pinneadas.** El orden es cronológico (el relay sirve al revés): el backfill es la cabeza de un stream, no un reporte, y así el evento más nuevo queda justo arriba de donde caen los vivos — `feed history` conserva el orden del relay por la razón inversa. El filtro corre antes del límite, igual que `/history?user=`, para que filtrar no deje la terminal vacía. Y `--json` no re-marshalla: el cuerpo del historial se lee como `[]json.RawMessage`, así el ampersand, el `assignee: null` y el `.000Z` sobreviven.

  **Leer antes de conectar es la corrección de la segunda ronda.** Con el stream ya suscrito, un evento que se guardaba y se emitía durante la lectura del historial salía dos veces; un reprint es justo lo que el dedupe del relay existe para evitar, y el spec prohíbe deduplicar del lado del cliente. El hueco de dos requests que queda en su lugar es de la misma clase que el spec ya acepta en cada reconexión y que `feed history` recupera. `filled` solo se pone en verdadero cuando un relay contesta con un historial, así que una CLI arrancada antes que su relay igual llena la terminal cuando el relay vuelve.

  **Revisión.** Dos rondas. La primera cerró dos huecos de cobertura —el default de 10 que ningún test pinneaba y la imposibilidad de ver si la CLI angostaba la request del relay— y pidió extraer el request compartido (`fetch`). La segunda cerró los cuatro hallazgos que quedaban abiertos: la duplicación parseo+filtro pasó a una sola regla de aceptación (`emitter.accept`, `emit` desapareció), la *feature envy* se resolvió moviendo la selección a un método del `emitter` (`head`), la repetición del conteo de requests se concentró en `wantHits`, y la ventana de duplicado de la primera conexión se corrigió de raíz con el orden de lectura. Lo único que la revisión marcó y se decidió mantener es el orden cronológico del backfill, con el motivo en el registro.


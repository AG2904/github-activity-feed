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
- 2026-10-18: Completed (35 process-seam cases, `go vet`/`gofmt` clean, windows/linux/darwin builds clean, relay suite still 54 green). Two commits on `main`: `c5daf20 feat: fill the feed from the stored history (ticket 06)` and `b9f4d6d refactor: share the relay read path (ticket 06)`, el segundo con las correcciones de la revisión. El registro de evidencia está en `docs/tickets/cli/06-first-connection-is-not-blank.md`.

  **El backfill es un paso del watch, no un comando nuevo.** Corre dentro del callback de conexión aceptada, con el stream ya suscrito, y usa el mismo `activity.Filter` y el mismo `paint` que el stream: `emit` se partió en dos para que la política de salida (filtro, `--json`, color, forma) viva en un solo lugar y no pueda divergir entre el historial y lo vivo. `--json` entrega los bytes del relay, un payload por línea, así que `jq` lee lo mismo antes y después del primer evento.

  **Tres decisiones que el spec no fijaba y quedaron pinneadas.** El orden es cronológico (el relay sirve al revés): el backfill es la cabeza de un stream, no un reporte, y así el evento más nuevo queda justo arriba de donde caen los vivos — `feed history` conserva el orden del relay por la razón inversa. El filtro corre antes del límite, igual que `/history?user=`, para que filtrar no deje la terminal vacía. Y `--json` no re-marshalla: el cuerpo del historial se lee como `[]json.RawMessage`, así el ampersand, el `assignee: null` y el `.000Z` sobreviven.

  **Un costo aceptado y nombrado:** como el stream se suscribe primero, un evento guardado *y* emitido durante la lectura del historial se imprime dos veces. La alternativa (leer antes de conectar) cambia el duplicado por un hueco, y el spec prohíbe deduplicar en el cliente. Está en el registro, no escondido.

  **Revisión.** Los dos ejes: el de Spec encontró que el default de 10 no estaba pinneado por ningún test y que el harness no podía ver si la CLI angostaba la request del relay; los dos se cerraron (doce eventos guardados para el default, y el registro de la request completa para exigir un `/history` sin query). El eje de Standards pidió extraer el request compartido (`fetch`) y renombrar la fila a `storedEvent`; ambos hechos. Dos olores del baseline quedaron sin tocar y con motivo escrito en el registro: la duplicación del par parseo+filtro (un predicado compartido tendría que devolver los tres resultados que `emit` distingue para el backoff) y la *feature envy* sobre el `emitter` (mover el HTTP adentro del tipo de salida le daría una razón más para cambiar, y el filtro no está menos expuesto por vivir en otro archivo del mismo paquete).

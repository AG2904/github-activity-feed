# Ticket 06 - La primera conexión no arranca en blanco

## Resumen

`feed watch` ya no arranca con la terminal vacía. En la primera conexión pide
`/history`, imprime los eventos guardados más recientes (10 por defecto, con
`--backfill N`; `0` lo deshabilita) y recién entonces sigue el stream. El relay
no reenvía nada al conectar, así que sin esto el desarrollador no podía
distinguir una CLI sana de una rota hasta el próximo merge a `main` — que puede
tardar días.

Dos commits en `main`: `c5daf20 feat: fill the feed from the stored history
(ticket 06)` y `b9f4d6d refactor: share the relay read path (ticket 06)`, el
segundo con las correcciones de la revisión de dos ejes.

## Cambios Realizados

### client/internal/cli/backfill.go (nuevo)

- **`backfill(ctx, url, out, limit)`**: pide el historial, elige las filas más
  recientes que pasan el filtro, las invierte y las pinta. `limit <= 0` no pide
  nada al relay.
- **`storedHistory`** lee el cuerpo como `[]json.RawMessage`. La elección no es
  cosmética: un `json.RawMessage` conserva los bytes exactos de cada elemento, así
  que `--json` sigue entregando el ampersand literal, el `assignee: null` y la
  forma `.000Z` sin re-marshalling. Parsear a `activity.Event` y volver a
  serializar habría roto las tres cosas en silencio.
- **`mostRecent`** es un predicado puro: parsea cada fila, descarta la que la CLI
  no puede leer (la misma postura que el stream ante un frame roto) y aplica el
  filtro. El filtro corre **antes** del límite, así que `--backfill 10 --user me`
  muestra los diez eventos más recientes **míos** y no los que de los últimos diez
  de todos resulten ser míos.
- La request es `GET /history` sin query: el filtrado es local, igual que en el
  stream (ADR-0001). El relay no se enteró de que existe un `--user`.

### client/internal/cli/watch.go

- **`--backfill N`** (default 10) se suma a los flags de `watch`.
- **`emitter.paint`**: se partió `emit` en dos. `emit` parsea, filtra y delega;
  `paint` escribe un evento ya aceptado. Es el único lugar donde vive la política
  de salida, así que el backfill y el stream no pueden discrepar sobre `--json`,
  color o forma renderizada. El filtro no se aplica dos veces: `mostRecent` usa la
  misma `activity.Filter` que `emit` lleva adentro.
- **El backfill corre dentro del callback de conexión aceptada**, no antes del
  loop: el stream ya está suscrito cuando se lee el historial, así que un evento
  que ocurra durante la lectura queda en el buffer del socket y se imprime
  después. Hacerlo antes de conectar dejaba un hueco.
- `filled` (no `attempts`) decide si el backfill corre: exactamente una vez, en el
  primer `stream` que llega a responder. El aviso `watch: reconnected` sigue
  colgando de `attempts > 0`, como en el ticket 04, para no cambiar en silencio
  cuándo aparece.
- `stream` ahora recibe `opened func() error`: un fallo al escribir el feed es
  fatal (stdout cerrado), un fallo al leerlo no.

### client/internal/cli/cli.go

- **`fetch(ctx, url)`** extraído: `history` y el backfill abren su request por el
  mismo lugar, y los dos coinciden en que un relay que contesta mal se ve igual
  (`%s answered %s`). Salió de la revisión: las dos copias del request ya existían.

### client/test/

- `historyOf(payloads...)` arma el cuerpo de `/history` con los mismos payloads
  que un test mandaría por el stream: una sola fuente de bytes para los dos
  caminos (historia 18).
- El harness ahora registra la **request completa**, no sólo el path, y puede
  hacer que `/history` conteste 500. Lo primero es lo que permite constatar que la
  CLI siga pidiendo un historial sin filtrar; lo segundo, que un relay que
  contesta mal no sea fatal.
- Diez casos nuevos: el backfill de la primera conexión, el límite explícito, el
  default de diez pinneado con doce eventos guardados, `--backfill 0`, el filtro
  antes del límite y la request sin filtrar, la reconexión que no repite, el
  historial vacío, el historial ilegible, el 500, y `--json` con un payload por
  línea.

## Comportamiento resultante

```text
$ feed watch --user maria.perez --backfill 2 --no-color
[pr.merged] Fix memory leak in parser component
AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/pull/42
[issue.assigned] lucia.gomez Ship the onboarding checklist
AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/issues/7
# a partir de acá el stream: cada evento nuevo aparece debajo

$ feed watch --backfill 0
# nada hasta que llegue un evento vivo

$ feed watch --json --backfill 2
{"type":"pr.merged","repository":"AG2904/github-activity-feed",...}
{"type":"issue.assigned",...}
# el payload del relay, uno por línea, la misma forma que continúa el stream
```

## Verificación

```text
cd client && gofmt -l .
(sin salida)

cd client && go vet ./...
(exit 0, sin salida)

cd client && go test -count=1 ./...
?   	github.com/AG2904/github-activity-feed/client/cmd/feed	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/activity	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/cli	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/render	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/sse	[no test files]
ok  	github.com/AG2904/github-activity-feed/client/test	21.330s

cd client && GOOS=linux go build ./...                    → exit 0
cd client && GOOS=darwin go build ./...                   → exit 0
cd client && GOOS=darwin GOARCH=arm64 go build ./...      → exit 0
cd client && go build ./...                               → exit 0

npm test
Test Files  1 passed (1)
Tests       54 passed (54)
```

La suite del cliente tiene 35 casos, todos por el límite del proceso.

## Acceptance Criteria

- ✅ AC1: en la primera conexión la CLI consulta el historial, imprime los
  `--backfill` eventos más recientes y después sigue el stream.
- ✅ AC2: `--backfill N` es 10 por defecto (`TestWatchBackfillsTenEventsByDefault`
  con doce eventos guardados exige veinte líneas y la ausencia de los dos más
  viejos); `--backfill 0` deshabilita: no imprime nada y no le pide nada al relay.
- ✅ AC3: los eventos del backfill honran `--user` y `--repo` exactamente como el
  stream, porque pasan por el mismo `activity.Filter` y el mismo `paint`.
- ✅ AC4: el backfill es plano. `watch` no acepta `--group-by-repo` — agrupar es
  del reporte (`feed history`, ticket 07), no de un stream.
- ✅ AC5: una reconexión nunca lo vuelve a correr: `filled` es de un solo uso y el
  caso del drop exige un único `GET /history` y ninguna línea repetida.
- ✅ AC6: un historial vacío no es un error: pide `/history`, no imprime nada y
  sigue el stream.

## Decisiones y límites conocidos

- **Orden: del más viejo al más nuevo.** El spec no lo fija, y el relay sirve el
  historial al revés. Acá se invierte a propósito: el backfill es la cabeza de un
  stream, y leerlo en orden cronológico deja el evento más nuevo justo arriba de
  donde van a caer los vivos. `feed history` (ticket 07) mantiene el orden del
  relay porque es un reporte y se lee de arriba hacia abajo. Si el equipo prefiere
  el orden del relay, es borrar el `slices.Reverse` y dar vuelta las expectativas.
- **El filtro corre antes del límite.** Es la misma semántica que `/history?user=`
  del relay (que devuelve todo lo de un usuario, no "lo último de todos, filtrado"),
  y evita el peor resultado posible: un backfill vacío justo cuando el desarrollador
  filtró por lo suyo. Pinneado con un caso donde el evento más nuevo es de otro
  repositorio.
- **La ventana de duplicado de la primera conexión está aceptada, no escondida.**
  Como el stream se suscribe primero y el historial se lee después, un evento que
  se guarde y se emita durante esa lectura sale en el historial y otra vez por el
  stream. La alternativa (leer el historial antes de conectar) cambia el duplicado
  por un hueco, y un hueco pierde el evento para siempre. No se deduplica en el
  cliente: el spec lo dice explícitamente ("the binary is a display client with
  nothing of its own to dedupe") y la ventana es de milisegundos. Es el precio de
  que la primera conexión sea la que menos agujeros deja.
- **Falla de historial = silencio.** Un `/history` que contesta 500, que no
  responde o que devuelve un cuerpo ilegible no mata el watch ni ensucia stderr: el
  stream es lo que se pidió, el historial es una comodidad, y el `--backfill` no se
  reintenta en la reconexión (AC5). De paso mantiene intacto el conteo exacto de
  líneas de stderr que el ticket 04 pinnea para un relay inalcanzable.
- **`--json` en el backfill: un payload por línea**, con los bytes del relay, no el
  array. Es la forma que el stream continúa, así que `feed watch --json | jq` lee
  una sola cosa (historia 18). El array verbatim es `feed history --json`, que ya
  existe desde el ticket 03.
- **Duplicación deliberada: parseo + filtro.** `mostRecent` repite el par
  `activity.Parse` + `filter.Match` de `emit`. La revisión propuso compartirlo y no
  se hizo: `emit` distingue **tres** resultados (no es un evento / es un evento que
  el filtro descarta / se pinta), porque los dos primeros son distintos para el
  backoff, y un predicado compartido tendría que devolver los tres para que ningún
  llamador quede peor. Queda como deuda con esta nota, no como descuido.
- **El `emitter` sigue siendo la política de salida.** La revisión marcó *feature
  envy* porque `backfill` lee `out.filter` y llama `out.paint`. No se movió: el
  backfill hace trabajo de entrada (HTTP) y meterlo dentro del emitter sería una
  razón más para que ese tipo cambie. En Go, además, `filter` no está "más
  expuesto" por estar en otro archivo del mismo paquete.
- **Repetición en los tests.** Cuatro casos repiten el assert de `/history`; se
  dejaron a propósito, porque cada uno lleva su propio mensaje ("una vez", "0
  cuando está deshabilitado", "sólo la primera conexión", "el vacío se pidió") y
  son los mensajes los que explican la intención cuando fallan.
- **Cero trabajo del ticket 07.** No hay render del historial, ni `--group-by-repo`
  en `history`, ni `--user`/`--repo` en `history`: el backfill reusa el renderer del
  stream, no el reporte.
- **Deuda ajena:** el runbook manual y los scripts npm son del ticket 08.

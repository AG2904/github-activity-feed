# Ticket 06 - La primera conexión no arranca en blanco

## Resumen

`feed watch` ya no arranca con la terminal vacía. Antes de abrir el stream pide
`/history`, imprime los eventos guardados más recientes (10 por defecto, con
`--backfill N`; `0` lo deshabilita) y recién entonces se conecta. El relay no
reenvía nada al conectar, así que sin esto el desarrollador no podía distinguir
una CLI sana de una rota hasta el próximo merge a `main` — que puede tardar días.

Cuatro commits en `main`: `c5daf20 feat: fill the feed from the stored history
(ticket 06)`, `b9f4d6d refactor: share the relay read path (ticket 06)` y
`c021439 refactor: read the history before the stream opens (ticket 06)`, los dos
últimos con las correcciones de las dos rondas de revisión de dos ejes, más el
commit de este registro.

## Cambios Realizados

### client/internal/cli/backfill.go (nuevo)

- **`backfill(ctx, url, out, limit)`** pide el historial, elige las filas más
  recientes que este cliente muestra, las invierte y las muestra. `limit <= 0` no
  pide nada al relay. Devuelve si **leyó** un historial: un relay que no contesta
  deja la terminal como está y la conexión siguiente vuelve a preguntar, así que
  una CLI arrancada antes que su relay igual llena la terminal. Un historial que
  ya se leyó no se vuelve a pedir nunca.
- **`storedHistory`** lee el cuerpo como `[]json.RawMessage`. La elección no es
  cosmética: un `json.RawMessage` conserva los bytes exactos de cada elemento, así
  que `--json` sigue entregando el ampersand literal, el `assignee: null` y la
  forma `.000Z` sin re-marshalling. Parsear a `activity.Event` y volver a
  serializar habría roto las tres cosas en silencio.
- **`emitter.head(payloads, limit)`** es la regla de aceptación del `emitter`
  pedida sobre filas guardadas en lugar de frames. El filtro corre **antes** del
  límite, así que `--backfill 10 --user me` muestra los diez eventos más recientes
  **míos** y no los que de los últimos diez de todos resulten ser míos. Una fila
  que la CLI no puede leer se descarta, la misma postura que el stream ante un
  frame roto.
- La request es `GET /history` sin query: el filtrado es local, igual que en el
  stream (ADR-0001). El relay no se enteró de que existe un `--user`.

### client/internal/cli/watch.go

- **`--backfill N`** (default 10) se suma a los flags de `watch`.
- **El historial se lee antes de abrir el stream**, en cada intento hasta que un
  relay conteste. Es lo único que hace imposible imprimir un evento dos veces:
  leerlo con el stream ya suscrito repetiría lo que llegue durante la lectura, que
  es justo el reprint que el dedupe del relay existe para evitar.
- **`emitter.accept` es la única regla de aceptación**: parsea el payload y
  devuelve `(event, valid, shown)`. `valid` distingue "fue un evento" de "no se
  pudo leer", porque un evento que el filtro descarta igual resetea el backoff;
  `shown` es "este cliente lo muestra". El stream y el backfill la piden al mismo
  lugar, así que no pueden discrepar sobre `--user` ni `--repo`.
- **`emitter.paint` es el único lugar donde un evento llega a stdout**, así que
  las dos rutas no pueden discrepar sobre `--json`, color o forma renderizada. El
  viejo `emit` desapareció: era `accept` + `paint` con un tercer resultado de más,
  y su loop de llamada ahora dice explícitamente las dos cosas que le importan
  (que el evento fuera válido, y que se muestre).

### client/internal/cli/cli.go

- **`fetch(ctx, url)`** extraído: `history` y el backfill abren su request por el
  mismo lugar, y los dos coinciden en que un relay que contesta mal se ve igual
  (`%s answered %s`).

### client/test/

- `historyOf(payloads...)` arma el cuerpo de `/history` con los mismos payloads
  que un test mandaría por el stream: una sola fuente de bytes para los dos
  caminos (historia 18).
- El harness registra la **request completa** (no solo el path) y puede hacer que
  `/history` conteste 500 y después recuperarse. Lo primero es lo que permite
  constatar que la CLI sigue pidiendo un historial sin filtrar; lo segundo, que un
  relay que contesta mal no sea fatal y que el backfill se reintente.
- `wantHits(t, request, want, meaning)` concentra el conteo de requests, para que
  cada caso diga qué significa el número en vez de repetir el mismo `if`.
- Doce casos nuevos: el backfill de la primera conexión, el límite explícito, el
  default de diez pinneado con doce eventos guardados, `--backfill 0`, el filtro
  antes del límite y la request sin filtrar, la reconexión que no repite, el
  historial vacío, el historial ilegible, el 500, el reintento hasta que el relay
  contesta, y `--json` con un payload por línea.

## Comportamiento resultante

```text
$ feed watch --user maria.perez --backfill 2 --no-color
[pr.merged] Fix memory leak in parser component
AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/pull/42
[issue.assigned] lucia.gomez Ship the onboarding checklist
AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/issues/7
# a partir de aquí el stream: cada evento nuevo aparece debajo

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
ok  	github.com/AG2904/github-activity-feed/client/test	24.118s

cd client && GOOS=linux go build ./...                    → exit 0
cd client && GOOS=darwin go build ./...                   → exit 0
cd client && GOOS=darwin GOARCH=arm64 go build ./...      → exit 0
cd client && go build ./...                               → exit 0

npm test
Test Files  1 passed (1)
Tests       54 passed (54)
```

La suite del cliente tiene 36 casos, todos por el límite del proceso.

## Acceptance Criteria

- ✅ AC1: antes de la primera conexión la CLI consulta el historial, imprime los
  `--backfill` eventos más recientes y después sigue el stream.
- ✅ AC2: `--backfill N` es 10 por defecto (`TestWatchBackfillsTenEventsByDefault`
  con doce eventos guardados exige veinte líneas y la ausencia de los dos más
  viejos); `--backfill 0` deshabilita: no imprime nada y no le pide nada al relay.
- ✅ AC3: los eventos del backfill honran `--user` y `--repo` exactamente como el
  stream, porque pasan por el mismo `accept` y el mismo `paint`.
- ✅ AC4: el backfill es plano. `watch` no acepta `--group-by-repo` — agrupar es
  del reporte (`feed history`, ticket 07), no de un stream.
- ✅ AC5: una reconexión nunca lo vuelve a correr: un historial leído no se pide
  dos veces y nada se imprime dos veces. La lectura que un relay **rechazó** sí se
  reintenta en la conexión siguiente — no es un re-run de nada, porque no corrió —
  y está razonado abajo.
- ✅ AC6: un historial vacío no es un error: pide `/history`, no imprime nada y
  sigue el stream.

## Decisiones y límites conocidos

- **El historial se lee antes de abrir el stream.** La alternativa (suscribirse
  primero y leer después) tiene un atractivo real: no pierde nada de la ventana,
  porque lo que llega durante la lectura queda en el buffer del socket. Pero crea
  un duplicado, y un duplicado es peor que un hueco en el modelo que el spec ya
  eligió: "a reconnect never replays what was already printed", y `feed history`
  es lo que recupera lo que se perdió. Con este orden la CLI nunca reimprime, y el
  hueco que deja (dos requests al relay, la primera vez) es de la misma clase que
  ya acepta en cada reconexión. La revisión lo marcó como defecto y se corrigió.
- **Reintento de una lectura rechazada.** `filled` solo se pone en verdadero
  cuando un relay contesta con un historial. Un relay que estaba reiniciando
  cuando el desarrollador arrancó la CLI no le deja la terminal vacía para
  siempre: cuando vuelve, la conexión que lo alcanza llena la terminal. La
  lectura literal del AC5 podría pedir "una sola vez y punto", pero un backfill
  que nunca ocurrió no es un backfill, y lo que el AC protege —que una
  reconexión no reimprima historial— se cumple igual y está pinneado.
- **Orden: del más viejo al más nuevo.** El spec no lo fija y el relay sirve el
  historial al revés; la revisión lo señaló como invención del diff. Se mantiene
  igual: el backfill es la cabeza de un stream, y leerlo en orden cronológico deja
  el evento más nuevo justo arriba de donde van a caer los vivos. `feed history`
  (ticket 07) mantiene el orden del relay porque es un reporte y se lee de arriba
  hacia abajo. Si el equipo prefiere el orden del relay, es borrar el
  `slices.Reverse` y dar vuelta las expectativas.
- **El filtro corre antes del límite.** Es la misma semántica que `/history?user=`
  del relay (que devuelve todo lo de un usuario, no "lo último de todos,
  filtrado"), y evita el peor resultado posible: un backfill vacío justo cuando el
  desarrollador filtró por lo suyo. Pinneado con un caso donde el evento más nuevo
  es de otro repositorio.
- **Falla de historial = silencio en stderr.** Un `/history` que no responde, que
  contesta 500 o que devuelve un cuerpo ilegible no mata el watch ni ensucia la
  salida de errores: el stream es lo que se pidió, el historial es una comodidad.
  De paso mantiene intacto el conteo exacto de líneas de stderr que el ticket 04
  pinnea para un relay inalcanzable — el backfill no agrega ni una.
- **`--json` en el backfill: un payload por línea**, con los bytes del relay, no el
  array. Es la forma que el stream continúa, así que `feed watch --json | jq` lee
  una sola cosa (historia 18). El array verbatim es `feed history --json`, que ya
  existe desde el ticket 03.
- **Cero trabajo del ticket 07.** No hay render del historial, ni `--group-by-repo`
  en `history`, ni `--user`/`--repo` en `history`: el backfill reusa el renderer del
  stream, no el reporte.
- **Deuda ajena:** el runbook manual y los scripts npm son del ticket 08.

## Revisión

Dos rondas de revisión de dos ejes, y lo que cambió cada una.

**Ronda 1 (sobre `c5daf20`).** *Spec*: el default de 10 no lo pinneaba ningún test
—lo cierra uno con doce eventos guardados— y el harness no podía ver si la CLI
angostaba la request del relay, porque registraba solo el path (ahora registra la
request completa y un caso exige un `/history` sin query). *Standards*: extraer el
request compartido (`fetch`) y renombrar la fila a `storedEvent`.

**Ronda 2 (sobre esta revisión).** Cuatro hallazgos más, los cuatro cerrados:

1. **Duplicación parseo+filtro** entre el camino del stream y el del historial →
   ahora hay una sola regla de aceptación (`emitter.accept`), que los dos piden.
2. ***Feature envy*** del backfill sobre el `emitter` → la selección pasó a ser un
   método del dueño del filtro (`emitter.head`).
3. **Repetición en los tests** del conteo de requests → `wantHits`, que además
   obliga a cada caso a decir qué significa el número.
4. **La ventana de duplicado de la primera conexión**, que la ronda 1 había dejado
   como costo aceptado → corregida de raíz con el orden de lectura, no con un
   dedupe del lado del cliente (que el spec prohíbe: "nothing of its own to
   dedupe").

Queda una sola invención del diff que la revisión marcó y se decidió **mantener**,
con el motivo escrito arriba: el orden cronológico del backfill.

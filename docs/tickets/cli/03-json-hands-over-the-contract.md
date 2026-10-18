# Ticket 03 - `--json` entrega el contrato intacto

## Resumen

El modo raw. `feed watch --json` escribe el payload de cada frame tal como lo
mandó el relay, una línea por evento y sin re-serializar; `feed history --json`
copia el cuerpo de la respuesta byte a byte. El objetivo es que un script pueda
consumir el contrato sin que el CLI lo reinterprete: orden de claves,
`assignee: null` y la forma `.000Z` sobreviven porque los bytes nunca pasan por
`encoding/json` de vuelta.

Dos commits en `main`: `06e802c feat: hand the contract over untouched with
--json (ticket 03)` y `83c021b refactor: give the watch loop an emit seam
(ticket 03)`, el segundo con las correcciones de la revisión de dos ejes.

## Cambios Realizados

### client/internal/cli/cli.go

- **`watch --json`.** El flag bypassa el renderer: cada payload válido se escribe
  con `io.WriteString(stdout, payload+"\n")`, sin marshal ni normalización.
  `--no-color` sigue siendo aceptado y es irrelevante en este modo; no hay
  ningún escape posible porque `render.Palette` no participa.
- **`history --json`.** Copia `resp.Body` a stdout con `io.Copy`, sin agregar un
  salto de línea. Lo que el relay respondió es lo que sale.
- **Validación conservada.** `emit` parsea cada payload antes de escribirlo, en
  ambos modos. `--json` cambia la forma de la salida, no la regla del ticket 02:
  un frame que no parsea se descarta y el proceso sigue leyendo.
- **Conflicto de flags.** `history --json --group-by-repo` imprime en stderr
  `history: --json and --group-by-repo cannot be combined: grouping is
  presentation, and raw output has none` y sale con `2` antes de tocar la red.
- **`--group-by-repo`** existe desde este ticket porque el criterio de
  aceptación lo exige para el conflicto; todavía no agrupa nada (ticket 07).
- La ayuda raíz y la de cada subcomando muestran `--json` y la exclusividad
  `[--json | --group-by-repo]`.
- **`emit`** separa transporte, parseo y política de salida dentro de `watch`,
  para que el próximo ticket no vuelva a mezclar las tres cosas en el loop.

### client/test/harness_test.go

- El cuerpo de `/history` del fake relay pasó a `historyBody`, con `setHistory`
  y `history()` bajo el mismo mutex que ya protegía `request`. Un test puede
  servir un array realista sin romper la disciplina de concurrencia del harness.

### client/test/e2e_test.go

- **Watch JSON:** primero un bloque de frames inútiles (`id:` solo, comentario,
  `event:` solo, `retry:`, JSON truncado, cuerpo que no es JSON) y después un
  evento válido. Se exige que stdout sea exactamente el payload válido más un
  `\n`, sin escapes.
- **History JSON:** se compara el stdout completo contra el cuerpo del relay,
  byte a byte, sin salto de línea agregado.
- **Conflicto:** exit `2`, mensaje que explica la exclusión, stdout vacío y cero
  requests a `/history`.
- El payload de los tests contiene un `&`: si el CLI re-serializara, Go lo
  escaparía como `\u0026` y la comparación exacta fallaría. Ésa es la evidencia
  de que los bytes pasan sin tocar.

## Comportamiento resultante

```text
$ feed watch --json
{"type":"pr.merged","repository":"AG2904/github-activity-feed","actor":"maria.perez","subject":"Fix parser & renderer","assignee":null,"labels":[],"url":"https://github.com/AG2904/github-activity-feed/pull/42","when":"2026-10-11T13:00:00.000Z"}

$ feed history --json
[{"type":"pr.merged",...},{"type":"issue.assigned",...}]

$ feed history --json --group-by-repo
history: --json and --group-by-repo cannot be combined: grouping is presentation, and raw output has none
(exit 2)
```

## Verificación

```text
cd client && go test -count=1 ./...
?   	github.com/AG2904/github-activity-feed/client/cmd/feed	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/activity	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/cli	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/render	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/sse	[no test files]
ok  	github.com/AG2904/github-activity-feed/client/test	8.403s

cd client && go vet ./...
(exit 0, sin salida)

cd client && gofmt -l .
(sin salida)

cd client && GOOS=linux go build ./...   → exit 0
cd client && GOOS=darwin go build ./...  → exit 0

npm test
Test Files  1 passed (1)
Tests       54 passed (54)
```

La suite del cliente tiene 13 casos, todos por el límite del proceso.

## Acceptance Criteria

- ✅ AC1: `feed watch --json` escribe una línea por evento con el payload `data:` del frame, byte a byte, sin re-serialización.
- ✅ AC2: La línea emitida preserva las ocho claves en el orden del relay, `assignee: null` cuando no hay assignee, y la forma `.000Z`.
- ✅ AC3: `feed history --json` escribe el cuerpo de la respuesta del relay verbatim.
- ✅ AC4: `--json` combinado con `--group-by-repo` sale con `2` y un mensaje en stderr: agrupar es presentación y la salida raw no tiene.
- ✅ AC5: `--json` suprime color y hyperlinks independientemente de si stdout es una terminal.

## Decisiones y límites conocidos

- **Passthrough validado.** El modo raw no re-marshalea, pero sí exige que el
  payload sea un ActivityEvent parseable. Un cuerpo roto se descarta, como en el
  ticket 02; el modo raw no convierte al CLI en un `cat` del socket.
- **`history --json` no agrega newline.** El cuerpo se copia tal cual; si el
  relay no termina en salto de línea, la salida tampoco.
- **`--group-by-repo` todavía no agrupa.** Su comportamiento llega con el ticket
  07; este ticket sólo define el conflicto con `--json`.
- **El color sigue sin test automatizado en terminal.** En este modo es
  estructuralmente imposible que aparezca un escape, porque el renderer no se
  invoca.
- **Deuda ajena:** el árbol de `ARCHITECTURE.md` y los scripts npm siguen en el
  ticket 08.

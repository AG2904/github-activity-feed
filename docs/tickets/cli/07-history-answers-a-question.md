# Ticket 07 - `feed history` responde una pregunta

## Resumen

`feed history` deja de ser un stub que descargaba el cuerpo y lo tiraba. Ahora es
el reporte que el spec prometió: lee el historial una vez, lo imprime con la misma
forma de dos líneas que el stream, y lo angosta con `--user` (Involvement: actor o
assignee) y `--repo`, o lo agrupa por repositorio con `--group-by-repo`.

Dos commits en `main`: `576b78f feat: answer the history the relay stores
(ticket 07)` y el de las correcciones de la revisión de dos ejes, más este
registro.

## Cambios Realizados

### client/internal/cli/history.go (nuevo)

- **`history`**: flags `--relay`, `--user`, `--repo`, `--no-color`, `--json`,
  `--group-by-repo`; lee el historial una sola vez y sale 0.
- **El filtro es el mismo que el del stream.** El comando construye su `emitter`
  con `activity.Filter{User, Repository}` y usa `shown`, la regla de aceptación
  que el stream también pide. No hay una segunda definición de Involvement.
- **`--json` entrega los bytes del relay de cada evento que pasa el filtro,
  enmarcados en el array que el relay sirvió.** Nada se re-marshalla: el orden de
  claves, el `assignee: null` y el `.000Z` llegan como vinieron, y el ampersand
  literal sobrevive (hay un test con `&`). El sobre (`[`, `,`, `]`) es lo único
  que agrega la CLI.
- **`writeArray`**: el enmarcado, en un solo lugar, sin newline agregado.
- **Errores**: relay inalcanzable → 1 con el motivo en stderr; cuerpo que no se
  puede leer → 1; Ctrl-C → 0.

### client/internal/cli/stored.go (nuevo)

- **`storedEvent`** (los bytes del relay + el evento parseado) se movió acá desde
  `backfill.go`: ahora dos comandos leen las mismas filas.
- **`storedRows`** decodifica el cuerpo como `[]json.RawMessage`; **`storedHistory`**
  es `fetch` + `storedRows`, y lo usan el backfill y el reporte por igual.
- **`emitter.shown`** (antes `head`) devuelve las filas que este cliente muestra,
  en el orden del relay, sin límite: el límite es del backfill, no de la lectura.
  Es la regla de aceptación pedida sobre filas guardadas.

### client/internal/cli/backfill.go

- Se queda sólo con `backfill`: pide el historial, se queda con las primeras
  `limit` filas (las más nuevas) y las imprime al revés.

### client/internal/cli/cli.go

- `history` y el comando homónimo salieron de acá: `cli.go` queda como dispatch,
  configuración del relay y el plumbing compartido (`endpoint`, `fetch`,
  `colorEnabled`, `newFlagSet`, `parseFlags`, `relayFallback`).

### client/internal/cli/watch.go

- **`newEmitter(stdout, noColor, filter)`**: el mismo stdout, la misma regla de
  color y el mismo filtro en los dos comandos, para que `--no-color` y `--user`
  no puedan significar dos cosas distintas. `watch` es el único que además pone
  `json`, porque es el único donde `--json` cambia cómo se escribe un evento
  suelto.
- **`emitter.group(events, now)`**: la salida del reporte agrupado. Delega en la
  paleta igual que `paint` delega en la línea del feed, así que todo byte que la
  CLI escribe en stdout sale del `emitter`.

### client/internal/render/render.go

- **`Event` se partió en `Event` + `line(..., lineShape)`** para que el mismo
  formateador sirva a las dos formas del reporte. `lineShape` es el nombre de la
  distinción: `withRepository` para el feed, `withoutRepository` para el reporte
  agrupado, donde el header ya lo nombró.
- **`hasRepository`** nombra una sola vez la regla de "fila anterior a la columna"
  (ADR-0010), y la usan la línea y el agrupado.

### client/internal/render/grouped.go (nuevo)

- **`Grouped`**: el reporte por repositorio. Un header por repositorio en orden
  alfabético, los eventos de cada grupo en el orden en que llegaron, una línea en
  blanco entre grupos y el repositorio afuera de las líneas por evento porque el
  header ya lo nombró. Los eventos sin repositorio van últimos, sin header.
- **`byRepository`** hace el corte en una pasada y ordena los grupos con
  `slices.SortFunc`, así que el orden no depende del orden en que el relay los
  mandó.
- **`heading`**: el header en negrita cuando hay escapes; es la única estructura
  del reporte, así que es la línea que vale la pena pesar.
- El archivo separado es deliberado: `render.go` cambia cuando cambia la línea del
  feed, `grouped.go` cuando cambia el reporte.

### client/test/

- Nueve casos nuevos: el listado en orden del relay, `--user` (Involvement),
  `--repo` (incluyendo una fila sin repositorio, que no matchea), la conjunción de
  los dos, el agrupado alfabético, la fila sin repositorio al final sin header, el
  historial vacío, el relay inalcanzable, y el cuerpo ilegible.
- Un caso más para `--json --user`: el filtro corre antes del enmarcado.
- Constantes `mergedGrouped`/`assignedGrouped` junto a `mergedOutput`/`assignedOutput`
  para no repetir los literales del reporte agrupado.

## Comportamiento resultante

```text
$ feed history --user maria.perez --no-color
[issue.assigned] lucia.gomez Ship the onboarding checklist
AG2904/github-activity-feed · maria.perez · 2m ago · https://github.com/AG2904/github-activity-feed/issues/7
# el merge de juan no entra: maria no es la actora ni la assignee de ese evento

$ feed history --group-by-repo --no-color
AG2904/github-activity-feed
[issue.assigned] lucia.gomez Ship the onboarding checklist
maria.perez · 2m ago · https://github.com/AG2904/github-activity-feed/issues/7
[pr.merged] Fix the memory leak in the parser component
juan.gomez · 25m ago · https://github.com/AG2904/github-activity-feed/pull/42

other/repo
[pr.merged] Merge elsewhere
someone.else · 3h ago · https://github.com/other/repo/pull/9

$ feed history --json --user lucia.gomez | jq '.[].subject'
"Ship the onboarding checklist"
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
ok  	github.com/AG2904/github-activity-feed/client/test	22.876s

cd client && GOOS=linux go build ./...                    → exit 0
cd client && GOOS=darwin go build ./...                   → exit 0
cd client && GOOS=darwin GOARCH=arm64 go build ./...      → exit 0
cd client && go build ./...                               → exit 0

npm test
Test Files  1 passed (1)
Tests       54 passed (54)
```

La suite del cliente tiene 46 casos, todos por el límite del proceso.

## Acceptance Criteria

- ✅ AC1: `feed history` pide el historial una vez, lo imprime y sale 0
  (`TestHistoryListsTheStoredEventsNewestFirst`, `wantHits` sobre `/history`).
- ✅ AC2: sin filtro lista todos los eventos guardados en el orden del relay, el
  más nuevo primero. El orden se pasa tal cual; la CLI no re-ordena el reporte.
- ✅ AC3: `--user` y `--repo` lo filtran con el mismo significado que en el stream,
  porque es literalmente el mismo `activity.Filter` y la misma `accept`/`shown`.
  Tres casos lo pinnean, incluida la fila con `repository: null`, que no matchea
  ningún repositorio.
- ✅ AC4: `--group-by-repo` imprime un header por repositorio, en orden
  alfabético, conserva el orden del relay dentro de cada grupo y saca el
  repositorio de las líneas por evento. El caso del orden está construido al revés
  que el alfabético a propósito: sin el sort, falla (verificado quitándolo).
- ✅ AC5: un relay inalcanzable sale 1 con el motivo en stderr.
- ✅ AC6: un historial vacío no imprime nada y sale 0.

## Decisiones y límites conocidos

- **`--json` filtra.** Es la corrección de la revisión de dos ejes, y la que más
  importaba: `history --json --user me` descargaba el historial de todos porque el
  `io.Copy` del cuerpo pasaba antes de cualquier filtro. En el stream, el ticket 05
  ya había fijado que el filtro corre antes del payload crudo; acá se hace lo
  mismo. El contrato de "bytes del relay" se mantiene evento por evento —orden de
  claves, `null`, milisegundos, ampersand— y el array es el sobre que agrega la
  CLI. Sin filtro, ese sobre coincide con el cuerpo que el relay sirvió, que es lo
  que sigue pinneando el test del ticket 03.
- **Los eventos sin repositorio van últimos y sin header.** El ticket no los
  menciona; el spec sí: "groups under nothing rather than under an invented name"
  (ADR-0010). Se imprimen, porque es un reporte de auditoría y perder una fila es
  peor que un bloque sin título; su segunda línea ya dice todo lo que hay que decir
  sobre dónde pasó. Si molesta, la alternativa es un header que nombre la ausencia,
  que es exactamente lo que el spec descarta.
- **Línea en blanco entre grupos y header en negrita.** No están en el ticket; son
  forma del reporte y hacen legible un bloque que, si no, es un muro de dos líneas.
  Pinneados por bytes en los tests.
- **`--no-color` en `history`.** El ticket no lo pide, pero el spec dice que el
  color se desactiva con `--no-color` o `NO_COLOR`; el reporte ahora pinta, así
  que el flag tenía que existir también acá (y `NO_COLOR` ya funcionaba solo).
- **El agrupado vive en `render/grouped.go`.** Agrupar es darle forma a la
  presentación, igual que las dos líneas, y no toca flags; el CLI le entrega una
  lista finita y ya. Es también la razón por la que el agrupado no existe en el
  stream: una lista infinita y desordenada no se agrupa. El archivo separado es
  la respuesta a la revisión: `render.go` cambia cuando cambia la línea del feed,
  `grouped.go` cuando cambia el reporte.
- **`lineShape`, no un booleano suelto.** La revisión marcó el `bool` de `line`
  como *flag argument*; ahora los llamadores dicen `withRepository` o
  `withoutRepository`, así el sitio de llamada se explica solo y la distinción
  sigue viviendo dentro del paquete.
- **El reporte agrupado pasa por el `emitter`.** La revisión señaló que
  `history --group-by-repo` llegaba hasta `out.palette` y rompía la convención de
  que todo byte de stdout sale del `emitter`. Ahora hay `emitter.group`, que
  delega en la paleta igual que `paint` delega en la línea del feed: el
  `emitter` es la frontera de salida, la paleta sabe cómo se ve una línea.
- **`storedHistory` sobrevive.** La revisión lo llamó *middle man* cuando tenía un
  solo llamador; con el `--json` filtrando, el reporte también lo usa, así que la
  objeción desapareció: los dos comandos leen las filas guardadas por el mismo
  lugar.
- **Deuda ajena:** builds, runbook y scripts npm son del ticket 08.

## Revisión

Dos ejes sobre `576b78f`, y lo que cambió.

**Lo que se arregló.** (1) `--json` ignoraba `--user`/`--repo`: defecto, corregido
con el filtro antes del enmarcado y un caso propio. (2) El test del orden
alfabético era tautológico —el fixture ya venía alfabético— y se reconstruyó al
revés, verificado quitando el `SortFunc`. (3) La construcción del `emitter` estaba
duplicada en los dos comandos: ahora `newEmitter`. (4) El predicado "sin
repositorio" estaba escrito dos veces en `render`: ahora `hasRepository`. (5) El
`emitter` se construía y quedaba muerto en la rama `--json`: con una sola lectura
para todas las ramas, ya no. (6) Los literales del reporte agrupado se repetían en
los tests: constantes. (7) El nombre `shown` se sombreaba con el local: los locales
pasaron a `rows`. (8) El reporte agrupado llegaba hasta `out.palette` y rompía la
convención de que todo byte de stdout sale del `emitter`: ahora `emitter.group`.
(9) El `bool` de `line` era un *flag argument*: ahora `lineShape` con
`withRepository`/`withoutRepository`. (10) `render` tenía dos razones para
cambiar: la línea del feed y el reporte, y ahora son dos archivos.

**Lo que se mantuvo, con motivo.** La línea en blanco entre grupos y el header en
negrita son forma del reporte, no reglas: sin ellas el agrupado es un muro de dos
líneas y el header no se distingue de un evento. El bloque sin header para las
filas sin repositorio no es invención: el AC1 pide listar **todos** los eventos
guardados y el AC4 pide un header **por repositorio**; una fila sin repositorio no
es un repositorio y no se puede perder, así que va última sin título, que es
exactamente lo que el spec llama "groups under nothing rather than under an
invented name". Y el caso del cuerpo ilegible se queda porque el branch existe
—`storedRows` puede fallar en las tres ramas— y una rama de error sin test es peor
que un test de más.

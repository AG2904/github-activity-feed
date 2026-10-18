# El flujo de la CLI

Cómo se mueve un evento por el cliente de Go: qué archivo hace qué, en qué orden, y
qué invariantes sostienen el conjunto. Está escrito para alguien que abre el repo por
primera vez y quiere entender el código, no para justificar decisiones —eso último
vive en [`docs/adr/`](adr/), en [`.scratch/cli/spec.md`](../.scratch/cli/spec.md) y en
[`ARCHITECTURE.md`](../ARCHITECTURE.md)—.

Si preferís ver antes de leer, hay dos diagramas versionados:

- `.archify/architecture-cli-20261018-161013/cli.html` — la arquitectura por
  componentes: el binario, sus dos caminos y las dos terminales que toca.
- `.archify/architecture-cli-file-calls-20261018-161013/cli-file-calls.html` — cada
  archivo como nodo y cada flecha con el nombre de la llamada que la cruza.

Del otro lado del cable está el relay:
[`docs/relay-flow.md`](relay-flow.md) explica ese proceso.

## 1. Los tres recorridos

La CLI tiene exactamente dos comandos y un arranque. Todo lo demás son hojas.

### 1.1 El arranque (una vez por corrida)

```
main.go ──signal.NotifyContext──▶ cli.Run()
                                     │ 1. --version, si lo pidieron
                                     │ 2. dispatch: watch | history | nada
                                     ▼
                                  el subcomando, con stdout y stderr ya separados
```

- **`cmd/feed/main.go`** traduce `Ctrl-C` a un contexto cancelable, llama a
  `cli.Run()` y sale con el código que vuelve. Ese código **es** el contrato de
  proceso: 0 normal, 1 un fallo que no se puede reintentar, 2 un flag mal usado.
- Restaura el manejador por defecto apenas se cancela el contexto, así un segundo
  `Ctrl-C` mata un proceso colgado.
- La versión no está en el código: la inyecta el build con
  `-ldflags -X main.version=...`, y sin eso el binario se presenta como `dev`.

### 1.2 El camino en vivo (`feed watch`)

```
                ┌── si nadie contestó todavía ──────────────┐
                ▼                                           │
GET /history ──▶ mostrar los N más recientes ──▶ GET /stream ──▶ leer frames
                                                                    │
                                          data: {...} ──▶ accept() ─┤
                                                                    ▼
                                                          paint() ──▶ dos líneas
```

Paso por paso:

1. **`internal/cli/cli.go`** resuelve el relay como `--relay`, después
   `RELAY_URL`, después `http://localhost:3000`, y despacha el subcomando.
2. **`internal/cli/backfill.go`** pide el historial **antes de abrir el stream**.
   Es lo único que hace imposible imprimir un evento dos veces: leerlo con el stream
   ya suscrito repetiría lo que llegue durante la lectura, y un reprint es justo lo
   que el dedupe del relay existe para evitar. El precio es un hueco del tamaño de
   dos requests, que es el mismo hueco que deja una reconexión y que
   `feed history` recupera.
3. El backfill imprime del **más viejo al más nuevo**, aunque el relay sirva al
   revés: es la cabeza de un stream, y así el evento vivo cae debajo de todo.
4. `backfill` devuelve si **leyó** un historial. Un relay que no contesta deja la
   terminal como está y la conexión siguiente vuelve a preguntar, así que una CLI
   arrancada antes que su relay igual llena la terminal. Un historial que se leyó no
   se pide nunca más.
5. **`internal/cli/watch.go`** abre el stream y se queda ahí. `http.Client` sin
   timeout: la liveness la decide el lector, no un reloj sobre la conexión.
6. **`internal/sse/sse.go`** convierte los bytes en frames. Sólo `data:` produce un
   evento; `id:`, `event:`, `retry:` y los comentarios se ignoran a propósito (el
   `id:` del relay es un contador por conexión que no identifica nada reutilizable).
7. **`emitter.accept`** parsea cada frame y decide: `valid` cuando era un evento,
   `shown` cuando este cliente lo muestra. Un evento que el filtro descarta **igual**
   resetea el backoff: llegó, aunque no se pinte.
8. **`emitter.paint`** lo escribe: el payload del relay tal cual en `--json`, o las
   dos líneas de **`internal/render/`**.
9. Si la conexión se cae o se queda muda **cinco minutos**, el loop vuelve a
   empezar: backoff de uno a treinta segundos con jitter, un aviso por intento en
   stderr, y `watch: reconnected` la primera vez que vuelve a haber conexión.

### 1.3 El camino retrospectivo (`feed history`)

```
GET /history ──▶ storedHistory() ──▶ shown() ──┬── flat:  paint() por evento
                                               ├── json:  writeArray()
                                               └── group: palette.Grouped()
```

- **`internal/cli/history.go`** lee el historial **una vez** y sale. No se conecta al
  stream ni se queda escuchando: es una pregunta, no una suscripción.
- **`--json` no es una excepción al filtro.** La CLI entrega los bytes del relay de
  cada evento que pasa, enmarcados en el array que el relay sirvió: nada se
  re-marshalla, y un filtro sigue angostando, porque crudo es una forma y no un
  permiso. Sin filtro, el sobre coincide con el cuerpo servido.
- **`--json` y `--group-by-repo` juntos salen 2** antes de tocar la red: agrupar es
  presentación, y la salida cruda no tiene ninguna.
- El reporte plano imprime en el orden del relay —lo más nuevo primero—, porque un
  reporte se lee de arriba hacia abajo. Es la razón inversa del backfill, y por eso
  el mismo archivo sirve a los dos comandos sin opinar sobre el orden.

## 2. El modelo

### 2.1 El `ActivityEvent` — el contrato

`internal/activity/activity.go`. **Ocho campos, ni uno más**, y el mismo shape que
sirve el relay para el stream y para el historial, así el cliente parsea una cosa:

| Campo | Tipo en Go | Notas |
| --- | --- | --- |
| `Type` | `string` | `pr.merged` \| `issue.assigned`, conjunto cerrado |
| `Repository` | `*string` | `owner/name`; `nil` sólo en una fila anterior a la columna |
| `Actor` | `string` | quien causó el evento |
| `Subject` | `string` | título del PR o del issue |
| `Assignee` | `*string` | `nil` cuando el evento no tiene uno |
| `Labels` | `[]string` | nombres planos |
| `URL` | `string` | el link al PR o al issue |
| `When` | `time.Time` | ya parseado desde el ISO-8601 con milisegundos |

`Parse` **rechaza** un payload al que le falte un campo que el contrato siempre
lleva, y no lo pinta a medias. Un frame que no parsea se descarta y el loop sigue: es
la misma postura que toma el relay con un webhook que no entiende.

Los dos nulos **no significan lo mismo**, y confundirlos es el error fácil:
`Assignee` es `nil` porque una *clase* de evento no tiene asignado; `Repository` es
`nil` porque una *fila* es anterior a que existiera la columna (ADR-0010). Por eso
`Match` no deja que un evento sin repositorio pase un filtro de repositorio.

### 2.2 El `emitter` — la política de salida

`internal/cli/watch.go`. Un valor con cuatro cosas (el writer, la paleta, el flag de
`--json` y el filtro) y tres verbos:

- **`accept(payload)`** → `(event, valid, shown)`. Es la **única** regla de
  aceptación del programa: el stream y el backfill le preguntan lo mismo, así no
  pueden discrepar sobre `--user` ni `--repo`.
- **`paint(payload, event)`** → escribe un evento ya aceptado: el payload crudo o
  las dos líneas. Es el único camino del feed a stdout.
- **`group(events, now)`** → el reporte agrupado. Delega en la paleta igual que
  `paint` delega en la línea del feed: el `emitter` es la frontera de salida.

`newEmitter(stdout, noColor, filter)` lo construye igual en los dos comandos, para
que `--no-color` y `--user` no puedan significar dos cosas distintas.

### 2.3 El filtro — Involvement

`activity.Filter{User, Repository}` con un `Match` puro sobre el evento:

- `--user` matchea si el login es el **actor o el assignee** —Involvement, la misma
  regla que el relay aplica en `/history?user=`— con match exacto y case-sensitive.
- `--repo` lee el campo `repository`; no se deriva nada del `url` (ADR-0010).
- Los dos se combinan con **AND**, y el filtrado es **local**: la CLI nunca angosta
  lo que el relay manda, ni siquiera en `--json` (ADR-0001).

## 3. La forma de dos líneas

`internal/render/render.go` y `grouped.go`. Dos líneas por evento:

```text
[pr.merged] Fix the memory leak in the parser component
AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/…/pull/42
```

- La primera lleva el tipo, el assignee cuando lo hay (en negrita) y el asunto.
- La segunda lleva el repositorio, el actor y el tiempo relativo. Con la salida por
  un pipe no hay color y la segunda línea suma la **URL cruda**, porque un asunto sin
  hipervínculo es un callejón sin salida.
- Con color, el asunto es un hipervínculo **OSC 8** al `url` del evento.
- `lineShape` decide si la línea repite el repositorio: el feed sí, el reporte
  agrupado no, porque el header ya lo nombró.
- **`Grouped`** arma el reporte por repositorio: un header por repositorio en orden
  alfabético, los eventos de cada uno en el orden en que llegaron, una línea en
  blanco entre grupos, y los eventos sin repositorio —una fila anterior a la
  columna— últimos y sin header: no hay nada bajo lo que agruparlos y no se inventa
  un nombre.

## 4. El cableado (declaraciones, no llamadas)

- **`cmd/feed/main.go`** — el proceso: señales, `cli.Run`, `os.Exit`.
- **`internal/cli/terminal_windows.go` / `terminal_other.go`** — `isTerminal` con dos
  implementaciones por build tag: en Windows mira el modo de consola (y habilita
  VT), en el resto pregunta si el archivo es una terminal. Es lo que decide si hay
  color, y nada más.
- **`package.json`** — las cuatro entradas `cli:*`. `cli:build` y `cli:dist` llaman a
  **`client/tools/build.mjs`**, que fija `CGO_ENABLED=0`, `-trimpath` y la versión
  por target; `cli:test` y `cli:lint` corren `go test` y `go vet`. Los binarios caen
  en `client/dist/`, que git ignora.
- **`test/main_test.go`** — `TestMain` compila **dos** binarios con `-ldflags`: el
  normal y uno con el timeout de inactividad en 300 ms, para poder probar la regla de
  los cinco minutos sin esperar cinco minutos.

## 5. Mapa de archivos

| Archivo | Responsabilidad | Quién lo llama |
| --- | --- | --- |
| `cmd/feed/main.go` | el proceso: señales, versión, `os.Exit` | — |
| `internal/cli/cli.go` | dispatch, configuración del relay, `fetch` compartido, color | `main.go` |
| `internal/cli/watch.go` | el loop en vivo, el `emitter` y su `accept`/`paint`/`group` | `cli.Run` |
| `internal/cli/backfill.go` | la primera conexión: el historial una vez, más viejo primero | `watch` |
| `internal/cli/history.go` | el reporte: una lectura, plana, cruda o agrupada | `cli.Run` |
| `internal/cli/stored.go` | `storedEvent`, `storedHistory()`, `storedRows()`, `emitter.shown()` | `backfill`, `history` |
| `internal/sse/sse.go` | el parser SSE a mano: sólo `data:` | `watch` |
| `internal/activity/activity.go` | el contrato de ocho campos y el filtro | `watch`, `history`, `render` |
| `internal/render/render.go` | las dos líneas del feed y la paleta | `watch`, `history` |
| `internal/render/grouped.go` | el reporte por repositorio | `history` |
| `internal/cli/terminal_*.go` | ¿stdout es una terminal? | `cli` |
| `tools/build.mjs` | binarios estáticos y versión | `npm run cli:*` |
| `test/e2e_test.go` | los 46 casos, sobre el límite del proceso | — |
| `test/harness_test.go` | el relay falso y los helpers de procesos | `e2e_test.go` |
| `test/main_test.go` | compila los binarios que la suite corre | `TestMain` |

## 6. Los invariantes que no se ven en el código

Son las cosas que un lector nuevo rompe sin querer. Cada una tiene su evidencia en
la suite o en un ADR.

- **Una sola regla de aceptación.** Si agregás un camino que decida por su cuenta qué
  se muestra, el filtro deja de tener una definición. `accept` existe para eso.
- **`--json` entrega los bytes del relay, sin re-marshalling.** Volver a serializar
  reordenaría las claves y cambiaría el `null` y los milisegundos. El historial se
  lee como `[]json.RawMessage` por la misma razón.
- **Un evento filtrado resetea el backoff.** El reset depende de que el evento
  llegó, no de que se vea.
- **El historial se lee antes de abrir el stream.** Es lo que impide que un evento
  salga dos veces; el hueco que deja es el que ya acepta una reconexión.
- **El backfill corre una vez, y sólo después de que un relay conteste.** Una
  lectura rechazada se reintenta; una leída no se vuelve a pedir nunca.
- **Nada propio que deduplicar.** No hay cursor ni estado entre corridas. Un evento
  que pasa mientras la CLI está desconectada no vuelve por el stream: `feed history`
  es la respuesta.
- **El orden es del relay, en los dos comandos.** El reporte lo respeta; el backfill
  lo da vuelta a propósito porque es la cabeza de un stream. Ninguno de los dos
  ordena por su cuenta.
- **stdout lleva eventos y nada más.** El banner, los avisos de reconexión y los
  errores van a stderr; por eso `feed watch | tee` y `feed watch | jq` son seguros.
- **El color es cosa de la terminal, no del flag.** Sin TTY no hay escapes, y
  entonces el asunto lleva la URL cruda. `--no-color` y `NO_COLOR` fuerzan lo mismo.
- **El contrato son ocho campos.** Si agregás uno, hay aserciones de igualdad
  profunda en la suite que fallan a propósito para obligarte a decidirlo.
- **La suite nunca importa un paquete interno.** Todo se prueba por el límite del
  proceso: compilar, correr, mirar stdout, stderr y el código de salida.

## 7. Qué no está en este mapa

- **El relay**: es el otro proceso. Su mapa es [`docs/relay-flow.md`](relay-flow.md).
- **El build y la distribución**: los scripts npm y `tools/build.mjs` están arriba
  porque son parte del repositorio, pero publicar binarios y el runbook completo son
  [Fase 3](../ARCHITECTURE.md).
- **CI**: todavía no existe.
- **Un cursor o un `Last-Event-ID`**: deliberadamente no hay. El `id:` del relay es
  un contador por conexión; un cliente genérico lo leería como una posición que el
  servidor no tiene (ADR-0007).

## 8. Por dónde empezar a leer

Si tenés veinte minutos y querés entender el cliente de punta a punta, este orden
funciona:

1. `internal/activity/activity.go` — el contrato, que es de lo que habla todo lo
   demás.
2. `internal/cli/cli.go` — el proceso: dispatch, configuración y el `fetch`
   compartido. Es el archivo más chico que explica el comando entero.
3. `internal/cli/watch.go` — el loop completo: backfill, stream, reconexión,
   inactividad y el `emitter`.
4. `internal/sse/sse.go` — el parser, en sesenta líneas.
5. `internal/cli/history.go` y `internal/render/grouped.go` — el reporte, que es el
   otro camino y el que agrupa.
6. `test/e2e_test.go` — la suite es la especificación ejecutable: si algo no está
   claro arriba, acá está el comportamiento esperado, escrito como una aserción
   sobre un proceso.

Para **ejecutarlo y verlo con tus ojos**,
[`docs/manual-testing.md`](manual-testing.md) tiene los pasos con los comandos y la
salida esperada de cada uno.

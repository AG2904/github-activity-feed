# El flujo del CLI

Cómo se mueve un dato por el CLI en Go: qué archivo hace qué, en qué orden, y qué
invariantes sostienen el conjunto. Está escrito para alguien que abre `client/` por
primera vez y quiere entender el código, no para justificar decisiones —eso último
vive en [`docs/adr/`](adr/) y en [`ARCHITECTURE.md`](../ARCHITECTURE.md)—.

El relay está del otro lado del cable y tiene su propio mapa:
[`docs/relay-flow.md`](relay-flow.md).

Si preferís ver antes de leer, hay dos diagramas versionados:

- `.archify/architecture-cli-20261019-141819/cli.html` — los roles del CLI y cómo se
  conectan con el relay.
- `.archify/architecture-file-calls-cli-20261019-141819/file-calls.html` — cada
  archivo de `client/` como nodo y cada flecha con el nombre de la llamada que la
  cruza.

## 1. Los dos recorridos

El CLI tiene dos caminos de lectura y un arranque. Todo lo demás son hojas.

### 1.1 Una conexión en vivo (`feed watch`)

```
terminal ──feed watch──▶ cmd/feed/main.go (signal.NotifyContext)
                              │ cli.Run(os.Args[1:], stdout, stderr, version)
                              ▼
                         internal/cli/cli.go · watch()
                              │ flags: --relay --user --repo --no-color --json
                              │ banner "watching <url>" en stderr
                              ▼
                         internal/cli/watch.go · watch()  ── loop ──▶ stream()
                              │                                        │ GET /stream
                              │                                        │ (sin Last-Event-ID)
                              │                                        ▼
                              │                                   idleReader(5 min)
                              │                                        │
                              │                                        ▼
                              │                                   internal/sse · Reader.Next()
                              │                                        │ data:
                              │                                        ▼
                              │                                   emitter.emit()
                              │                                     ├─ activity.Parse()
                              │                                     ├─ Filter.Match()
                              │                                     └─ render.Palette.Event()
                              │                                        │  o payload crudo (--json)
                              │                                        ▼
                              │                                   stdout: el evento
                              └── error ──▶ backoff 1 → 30 s ──▶ reconecta
```

Paso por paso:

1. **`client/cmd/feed/main.go`** es el borde del proceso: crea un contexto que se
   cancela con `Ctrl-C`, llama a `cli.Run()` y sale con el código que este devuelve.
   El interior nunca llama a `os.Exit`: si lo hiciera, ningún test podría ejercitarlo
   sin matar el proceso de test. Un segundo `Ctrl-C` vuelve a poner el manejador por
   defecto, así que un run colgado se puede matar igual.
2. **`client/internal/cli/cli.go`** es el despachador. El flag set raíz sólo conoce
   `--version`; los subcomandos parsean los suyos. `feed` a secas imprime el uso y
   sale con `2`, y un subcomando desconocido hace lo mismo, con el nombre entre
   comillas. El contrato de flujos es fijo: **stdout lleva eventos** (y la versión,
   con `--version`), **stderr lleva el banner, los avisos y los errores**.
3. **`watch()`** arma el destino (`--relay`, después `$RELAY_URL`, después
   `http://localhost:3000`), escribe `watching <url>` en stderr y construye el
   `emitter` con las tres decisiones que no cambian durante la vida del proceso:
   paleta con o sin color, `--json` sí o no, y el filtro local.
4. **`stream()`** hace el `GET /stream` y sostiene esa conexión hasta que se cae, el
   relay se calla, o el contexto se cancela. Envuelve el body en un `idleReader` y le
   pasa el lector al parser SSE. Cada `Next()` produce un payload y `emitter.emit()`
   decide qué hacer con él.
5. **`emitter.emit()`** es la política de salida en un solo lugar. Un payload que no
   parsea **no es un error**: devuelve `false` y se ignora, porque un frame roto no
   puede tumbar un watch. Un evento que el filtro descarta devuelve `true`: prueba
   que la conexión está viva, aunque no se escriba nada. Si pasa, se imprime el
   payload tal cual (`--json`) o se renderiza en dos líneas.
6. **El loop** decide qué hacer con el error. Un error `fatal` (URL inválida, stdout
   cerrado) sale con `1`. Todo lo demás se reintenta: primero se calcula la espera,
   después se duerme, y se vuelve a llamar a `stream()`. Si `ctx` se canceló en
   cualquier punto, el retorno es `0`, no un error.
7. **`backoff`** arranca en un segundo, duplica hasta un techo de treinta, y suma un
   jitter de hasta un cuarto del paso. Una conexión que entregó al menos un evento
   válido resetea la escalera, así que un corte aislado vuelve a esperar un segundo.

### 1.2 Una consulta al historial (`feed history`)

```
terminal ──feed history──▶ cli.Run ──▶ internal/cli/cli.go · history()
                                            │ flags: --relay --json --group-by-repo
                                            │ target = relay + "/history"
                                            ▼
                                       GET /history ──▶ relay
                                            │ 200
                                            ▼
                                       --json:        io.Copy(stdout, body)
                                       sin --json:    io.Copy(io.Discard, body)
```

- **`history()`** hace una sola request y no reintenta: es una pregunta puntual, no
  una suscripción. Un `200` es éxito; cualquier otro código es un error con el status
  en stderr y salida `1`.
- **`--json` escribe el body crudo**, byte por byte, sin tocar `encoding/json`. Es la
  forma de ver el contrato del relay sin que el CLI opine sobre él.
- **`--json` y `--group-by-repo` juntos son un error de uso** (`2`), con el
  razonamiento en el mensaje: agrupar es presentación, y la salida cruda no tiene.
- **Hoy, sin `--json`, `history` no imprime nada**: consume el body y lo descarta.
  Es el ticket 07 del cliente, todavía abierto; `--group-by-repo` se acepta y no
  tiene efecto todavía. Lo que sí funciona es `--json`, y es lo que usa la suite.
- **No existe `--user` en `history`** todavía. El filtro por usuario que el relay ya
  sirve (`/history?user=`) todavía no tiene bandera en el CLI.

### 1.3 Códigos de salida

| Código | Cuándo |
| --- | --- |
| `0` | éxito; también `Ctrl-C`, `--help` y `--version` |
| `1` | el relay respondió distinto de `200`, falló el transporte, o stdout dejó de aceptar bytes |
| `2` | error de uso: subcomando faltante o desconocido, flag inválida, combinación inválida |

Un watch que no puede conectarse **no** sale: reintenta indefinidamente. Morirse
porque el relay estaba reiniciando es peor que esperar.

## 2. El modelo

### 2.1 `activity.Event` — el contrato, del lado del cliente

`client/internal/activity/activity.go`. Los mismos ocho campos que emite el relay,
con dos diferencias de representación:

| Campo | Tipo en Go | Notas |
| --- | --- | --- |
| `Type` | `string` | `pr.merged` o `issue.assigned`; el CLI no valida el conjunto |
| `Repository` | `*string` | `nil` sólo para una fila anterior a la columna |
| `Actor` | `string` | |
| `Subject` | `string` | |
| `Assignee` | `*string` | `nil` cuando el evento no tiene uno |
| `Labels` | `[]string` | se parsea; hoy no se renderiza |
| `URL` | `string` | |
| `When` | `time.Time` | ya parseado con `time.RFC3339` |

`Parse()` es la única puerta: rechaza un payload que no sea JSON, uno al que le falte
`type`, `actor`, `subject`, `url` o `when`, y uno cuyo `when` no parsea. Un evento a
medio llenar no se renderiza: se descarta. `labels` ausente queda `nil`, que es
distinto de una lista vacía sólo en la cabeza de quien lo lea —hoy nadie lo lee—.

### 2.2 `activity.Filter` — el filtro local

El relay es un broadcast: manda todo a todos y cada cliente filtra para sí
(ADR-0001). `Filter{User, Repository}` es ese filtro, y `Match()` aplica la misma
regla que `/history?user=`:

- `User` compara contra `Actor` **o** `Assignee` — involucramiento, nunca un tercer
  rol. Es exacto y sensible a mayúsculas.
- `Repository` compara contra el campo del evento; un `Repository` nulo no matchea
  ningún filtro de repositorio (ADR-0010).
- Un filtro vacío deja pasar todo.

Que las dos reglas coincidan no es casualidad: si divergieran, `feed watch --user`
y `feed history --user` contestarían cosas distintas sobre la misma persona.

## 3. Las reglas de la conexión

`client/internal/sse/sse.go` es el único código de protocolo del CLI, y el único
lugar donde un cambio en el formato de los frames puede romperlo (ADR-0007).

- **Sólo importa `data:`.** El parser junta las líneas `data:` de un frame y las une
  con `\n`, como manda el formato. `id:`, `event:`, `retry:` y los comentarios se
  ignoran. Un frame que termina en EOF sin su línea en blanco igual cuenta.
- **El `id:` se ignora a propósito.** Nest lo escribe en cada mensaje con un contador
  que arranca en `0` en cada conexión: no identifica nada. Un cliente genérico lo
  leería como `Last-Event-ID` y lo ofrecería como posición de reanudación, que no
  existe. El CLI nunca manda ese header, y la suite lo verifica.
- **El heartbeat del relay es un comentario**, y el parser ya los salteaba: el relay
  ganó liveness sin que el cliente cambiara una línea (ADR-0007, ticket 08 del relay).
- **El silencio es muerte después de cinco minutos.** `idleReader` reinicia su timer
  con cada byte; si expira, cierra el body y el error se traduce a
  `no bytes for 5m0s`. Con un heartbeat cada quince segundos, un stream sano nunca
  se acerca al límite: el timeout cubre una conexión que muere dentro del intervalo,
  o un relay que deja de escribir sin cerrar el socket.
- **La reconexión es del cliente.** Backoff exponencial de 1 a 30 segundos con
  jitter, reset cuando llegó un evento válido. El relay no guarda historia por
  conexión, así que reconectar deja un hueco que sólo `/history` puede llenar.
- **No hay timeout de request.** La conexión es por definición larga; el único reloj
  es el del `idleReader`.

## 4. El cableado (procesos, no llamadas)

- **`client/cmd/feed/main.go`** — señales, `os.Exit`, y el único lugar que conoce
  `os.Args` y `os.Stdout`/`os.Stderr`. La versión entra por
  `-ldflags "-X main.version=<v>"` y por defecto es `dev`.
- **`client/go.mod`** — módulo anidado, `go 1.27.0`, **sin un solo `require`**: el
  cliente se construye con la biblioteca estándar y nada más (ADR-0008, ADR-0009).
  Por eso todos los comandos de Go se corren dentro de `client/`.
- **`client/internal/cli/terminal_windows.go` y `terminal_other.go`** — dos builds de
  la misma función con build tags. En Windows `isTerminal` pregunta por el console
  mode y, si puede, prende `ENABLE_VIRTUAL_TERMINAL_PROCESSING` con un
  `SetConsoleMode` cargado por `LazyDLL`; si el console lo rechaza, devuelve `false`
  y no se emite un solo escape. En el resto de las plataformas alcanza con
  `ModeCharDevice`.
- **`client/test/`** — la suite no importa paquetes internos: compila dos binarios y
  les habla por la costura del proceso.

## 5. Mapa de archivos

| Archivo | Responsabilidad | Quién lo llama |
| --- | --- | --- |
| `client/cmd/feed/main.go` | proceso: señales, `cli.Run`, código de salida | la shell |
| `client/internal/cli/cli.go` | `Run`, despacho, `history`, `--version`, `colorEnabled` | `main.go` |
| `client/internal/cli/watch.go` | `watch`, `stream`, `emitter`, `idleReader`, `backoff` | `cli.Run` |
| `client/internal/cli/terminal_windows.go` | `isTerminal` + VT en Windows | `colorEnabled` |
| `client/internal/cli/terminal_other.go` | `isTerminal` en el resto | `colorEnabled` |
| `client/internal/sse/sse.go` | `Reader.Next()`: frames SSE a payloads | `watch.go` |
| `client/internal/activity/activity.go` | `Event`, `Parse()`, `Filter.Match()` | `watch.go` |
| `client/internal/render/render.go` | `Palette.Event()`: dos líneas de terminal | `watch.go` |
| `client/test/main_test.go` | compila los dos binarios con `-ldflags` | `go test` |
| `client/test/harness_test.go` | `fakeRelay`: `httptest` con `/stream` y `/history` | toda la suite |
| `client/test/e2e_test.go` | los casos, siempre a través del binario | `go test` |

## 6. Los invariantes que no se ven en el código

Son las cosas que un lector nuevo rompe sin querer. Cada una tiene su ADR.

- **stdout es un contrato, no un log.** Lleva eventos y nada más (más la versión con
  `--version`); el banner, los avisos de reconexión y los errores van a stderr. Eso es
  lo que hace seguros `feed watch | jq` y `feed watch | tee`.
- **`--json` no re-serializa.** Escribe el payload tal como llegó. Pasar por
  `json.Marshal` reordenaría claves y podría cambiar la forma de un `null` o del
  timestamp en milisegundos.
- **El color es del terminal, no del programa.** `--no-color`, `NO_COLOR`, o stdout
  que no es un `*os.File` o no es una consola: cualquiera de las tres apaga los
  escapes. Un pipe recibe texto plano.
- **Un frame que no es un evento no es una falla.** Se descarta en silencio. Un error
  de escritura en stdout sí es fatal: no tiene sentido seguir leyendo para nadie.
- **El filtro local no inventa reglas.** Es actor-o-assignee, igual que el historial.
  Cambiar una de las dos lados exige cambiar la otra.
- **No hay reanudación.** El `id:` se ignora y no se persiste cursor; el hueco entre
  reconexiones lo llena `feed history`, no el stream. Fingir continuidad sería mentir
  sobre lo que el relay guarda.
- **El interior no conoce el proceso.** `cli.Run` recibe `args`, `stdout` y `stderr`
  como parámetros y devuelve el código; nadie adentro toca `os.Args` ni `os.Exit`.
- **Cero dependencias.** Si `go.mod` gana un `require`, alguien rompió ADR-0008 sin
  decirlo. Es barato de chequear y es una propiedad, no un detalle.
- **El módulo está anidado.** `go build ./...` no corre desde la raíz del repo: hay
  que entrar en `client/` (ADR-0009).

## 7. Qué no está en este mapa

- **El relay** — tiene su propio mapa en [`docs/relay-flow.md`](relay-flow.md).
- **Los tickets abiertos del CLI**: 06 (la primera conexión no es una pantalla en
  blanco), 07 (el historial renderizado y sus filtros) y 08 (distribuir el binario).
  Hoy `history` sin `--json` no imprime, y no hay binarios publicados.
- **CI y release**: no hay workflow que compile el cliente ni publique artefactos.
- **El detalle de los colores**: qué badge toma qué color vive en
  `render.Palette.badge()`; no cambia el flujo.
- **La elección del relay**: `--relay` o `$RELAY_URL`; el resto de la configuración
  es del lado del relay.

## 8. Por dónde empezar a leer

Si tenés veinte minutos y querés entender el cliente de punta a punta, este orden
funciona:

1. `client/internal/activity/activity.go` — el contrato, que es de lo que habla todo
   lo demás.
2. `client/internal/sse/sse.go` — sesenta y seis líneas de protocolo, y el lugar donde
   un cambio de frame rompe todo.
3. `client/internal/cli/watch.go` — el camino completo: request, lectura, emisión,
   reconexión y backoff.
4. `client/internal/render/render.go` — cómo un evento se vuelve dos líneas.
5. `client/internal/cli/cli.go` — el despacho, los códigos de salida y el contrato de
   stdout/stderr.
6. `client/test/e2e_test.go` y `harness_test.go` — la suite es la especificación
   ejecutable: un `fakeRelay` de `httptest` y binarios reales, nunca imports internos.

Para ejecutarlo con tus ojos, desde `client/`:

```bash
go build -o feed ./cmd/feed
./feed watch   --relay http://localhost:3000
./feed history --relay http://localhost:3000 --json
GOOS=windows GOARCH=amd64 go build -o feed.exe ./cmd/feed
```

El relay tiene que estar arriba; los pasos para levantarlo y alimentarlo con un
webhook firmado están en [`docs/manual-testing.md`](manual-testing.md). En Windows el
binario es `feed.exe`, y `go test ./...` compila los dos binarios de prueba y tarda
alrededor de dos minutos.

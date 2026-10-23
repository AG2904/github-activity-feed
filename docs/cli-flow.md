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
                         internal/cli/cli.go · cli.Run
                              │ despacho: watch | history
                              │ flags de watch: --relay --user --repo
                              │                 --backfill <n> --no-color --json
                              │ banner "watching <url> · backfill <n>" en stderr
                              ▼
                         internal/cli/watch.go · watch()
                              │
                              │ primera conexión, si --backfill > 0:
                              │   GET /history ──▶ backfill() ──▶ N filas
                              │   oldest first, el vivo sigue debajo
                              ▼
                              loop
                              │
                              ├──▶ stream() ──▶ GET /stream (sin Last-Event-ID)
                              │                      │ aceptada: "connected · …"
                              │                      │ en stderr, la primera vez
                              │                      ▼
                              │                 idleReader(5 min)
                              │                      │
                              │                      ▼
                              │                 internal/sse · Reader.Next()
                              │                      │ data:
                              │                      ▼
                              │                 emitter.emit()
                              │                   ├─ activity.Parse()
                              │                   ├─ Filter.Match()
                              │                   └─ render.Palette.Event()
                              │                      │  o payload crudo (--json)
                              │                      ▼
                              │                 stdout: el evento
                              │
                              └── error ──▶ backoff 1 → 30 s ──▶ vuelve al loop

     Toda salida de watch() —Ctrl-C incluido— pasa por el defer que imprime
     "stopped · …" en stderr: una línea de cuenta, no una despedida.
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
   con `--version`), **stderr lleva el banner, las líneas de sesión, los avisos y los
   errores**.
3. **`watch()`** arma el destino (`--relay`, después `$RELAY_URL`, después
   `http://localhost:3000`), escribe en stderr la primera línea de la sesión —el
   banner `watching <url> · backfill <n>`, al que se agregan ` · user <login>` y
   ` · repo <owner/name>` en ese orden cuando los flags están puestos— y construye el
   `emitter` con las tres decisiones que no cambian durante la vida del proceso:
   paleta con o sin color, `--json` sí o no, y el filtro local. El `emitter` también
   cuenta lo que se muestra; esa cuenta es lo que dice la línea `connected · …` tras
   la primera conexión y lo que cierra la sesión con `stopped · …`.
4. **`backfill()`** corre antes de la primera conexión y pide `/history` una sola vez,
   para que la terminal no arranque en blanco: imprime hasta `--backfill <n>` filas
   (diez por defecto, `0` lo apaga) y las pinta oldest first, así el vivo sigue abajo
   en orden. Es best-effort a propósito: un relay que todavía no contesta deja la
   terminal como está y la conexión siguiente vuelve a preguntar; un historial que sí
   se leyó no se pide dos veces.
5. **`stream()`** hace el `GET /stream` y sostiene esa conexión hasta que se cae, el
   relay se calla, o el contexto se cancela. Envuelve el body en un `idleReader` y le
   pasa el lector al parser SSE. Cada `Next()` produce un payload y `emitter.emit()`
   decide qué hacer con él.
6. **`emitter.emit()`** es la política de salida en un solo lugar. Un payload que no
   parsea **no es un error**: devuelve `false` y se ignora, porque un frame roto no
   puede tumbar un watch. Un evento que el filtro descarta devuelve `true`: prueba
   que la conexión está viva, aunque no se escriba nada. Si pasa, se imprime el
   payload tal cual (`--json`) o se renderiza como la tarjeta de tres líneas —la
   frase, el título con las etiquetas al frente, el contexto— más la URL cuando no
   hay hyperlink, y se suma a la cuenta de lo mostrado.
7. **El loop** decide qué hacer con el error. Un error `fatal` (URL inválida, stdout
   cerrado) sale con `1`. Todo lo demás se reintenta: primero se calcula la espera,
   después se duerme, y se vuelve a llamar a `stream()`. Si `ctx` se canceló en
   cualquier punto, el retorno es `0`, no un error. Cualquiera sea el camino de
   salida —`Ctrl-C` incluido—, el `defer out.summarize(stderr)` imprime la última
   línea de la sesión: `stopped · <n> events · <k> merged · <l> assigned` cuando se
   mostró algo, `stopped · no events` cuando no. Los conteos son de eventos
   **mostrados**, filtro incluido: la respuesta a "qué vi", no a "qué mandó el relay".
8. **`backoff`** arranca en un segundo, duplica hasta un techo de treinta, y suma un
   jitter de hasta un cuarto del paso. Una conexión que entregó al menos un evento
   válido resetea la escalera, así que un corte aislado vuelve a esperar un segundo.

### 1.2 Una consulta al historial (`feed history`)

```
terminal ──feed history──▶ cli.Run ──▶ internal/cli/history.go · history()
                                             │ flags: --relay --user --repo
                                             │        --no-color --json | --group-by-repo
                                             │ target = relay + "/history"
                                             ▼
                                        GET /history ──▶ relay
                                             │ 200
                                             ▼
                                        storedHistory() ──▶ emitter.shown()
                                             │  filtra --user y --repo, descarta
                                             │  las filas que no parsean
                                             ▼
                                        --json:           writeArray, bytes crudos
                                        --group-by-repo:  render.Palette.Grouped
                                        sin flag:         render.Palette.Event,
                                                          newest first
```

- **`history()`** hace una sola request y no reintenta: es una pregunta puntual, no
  una suscripción. Un `200` es éxito; cualquier otro código es un error con el status
  en stderr y salida `1`.
- **El filtro es local, igual que en el stream.** `--user` y `--repo` se aplican sobre
  las filas que el relay sirvió, con el mismo `Filter`, así que `feed watch --user` y
  `feed history --user` no pueden contestar cosas distintas sobre la misma persona.
- **`--json` escribe un array de payloads crudos**, uno por evento mostrado, en el
  orden en que el relay los sirvió. Cada elemento son los bytes que llegaron: nada se
  re-serializa, así que claves, `null` y el timestamp en milisegundos quedan intactos.
  El filtro igual se aplica, porque crudo es una forma y no una exención.
- **Sin `--json` el historial se pinta**, newest first: la misma tarjeta
  `Palette.Event()` que usa el vivo, porque un reporte se lee desde arriba.
- **`--group-by-repo`** reemplaza ese render por uno agrupado: un encabezado por
  repositorio en orden alfabético, y las filas sin repositorio al final, sin encabezado,
  porque no hay nombre que inventarles (ADR-0010).
- **`--json` y `--group-by-repo` juntos son un error de uso** (`2`), con el
  razonamiento en el mensaje: agrupar es presentación, y la salida cruda no tiene.

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

`client/internal/activity/activity.go`. Los mismos nueve campos que emite el relay,
con diferencias de representación:

| Campo | Tipo en Go | Notas |
| --- | --- | --- |
| `Type` | `string` | `pr.merged` o `issue.assigned`; el CLI no valida el conjunto |
| `Repository` | `*string` | `nil` sólo para una fila anterior a la columna |
| `Branch` | `*string` | `nil` en una asignación, en una fila anterior a la columna, o si el payload no traía `head.ref` usable (ADR-0013) |
| `Actor` | `string` | |
| `Subject` | `string` | |
| `Assignee` | `*string` | `nil` cuando el evento no tiene uno |
| `Labels` | `[]string` | va entre corchetes al frente del título |
| `URL` | `string` | |
| `When` | `time.Time` | ya parseado con `time.RFC3339` |

`Parse()` es la única puerta: rechaza un payload que no sea JSON, uno al que le falte
`type`, `actor`, `subject`, `url` o `when`, y uno cuyo `when` no parsea. Un evento a
medio llenar no se renderiza: se descarta. `branch` y `repository` son suplementarios:
ausentes o `null` no rechazan el evento, y el renderer cae en la frase derivada del
tipo. `labels` ausente queda `nil`, distinto de
una lista vacía sólo en el modelo: el renderizador lee la longitud, así que las dos
formas imprimen el título sin un `[]` vacío.

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
| `client/internal/cli/cli.go` | `Run`, despacho, `--version`, `colorEnabled` | `main.go` |
| `client/internal/cli/watch.go` | `watch`, `stream`, `emitter`, `idleReader`, `backoff` | `cli.Run` |
| `client/internal/cli/history.go` | `history`: una request, filtro local y las tres salidas | `cli.Run` |
| `client/internal/cli/backfill.go` | `backfill`: la primera conexión llena la pantalla | `watch` |
| `client/internal/cli/stored.go` | `storedHistory`, `storedRows`, `emitter.shown` | `history`, `backfill` |
| `client/internal/cli/terminal_windows.go` | `isTerminal` + VT en Windows | `colorEnabled` |
| `client/internal/cli/terminal_other.go` | `isTerminal` en el resto | `colorEnabled` |
| `client/internal/sse/sse.go` | `Reader.Next()`: frames SSE a payloads | `watch.go` |
| `client/internal/activity/activity.go` | `Event`, `Parse()`, `Filter.Match()` | `watch.go`, `history.go` |
| `client/internal/render/render.go` | `Palette.Event()`: la tarjeta de un evento | `watch.go`, `history.go` |
| `client/internal/render/grouped.go` | `Palette.Grouped()`: el reporte por repositorio | `history.go` |
| `client/test/main_test.go` | compila los dos binarios con `-ldflags` | `go test` |
| `client/test/harness_test.go` | `fakeRelay`: `httptest` con `/stream` y `/history` | toda la suite |
| `client/test/e2e_test.go` | los casos, siempre a través del binario | `go test` |
| `client/tools/build.mjs` | `cli:build` y `cli:dist`: los cuatro binarios con versión | `npm run` |

## 6. Los invariantes que no se ven en el código

Son las cosas que un lector nuevo rompe sin querer. Cada una tiene su ADR.

- **stdout es un contrato, no un log.** Lleva eventos y nada más (más la versión con
  `--version`); el banner, la línea `connected` de la primera conexión, los avisos de
  reconexión, la línea `stopped` de la salida y los errores van a stderr, en texto
  plano y sin TTY-gating —una sesión redirigida a un log merece la misma narración
  que una terminal—. Eso es lo que hace seguros `feed watch | jq` y `feed watch | tee`.
- **La sesión se narra tres veces y nada más.** Un banner al arrancar, una sola
  línea `connected` tras la primera conexión —una reconexión dice
  `watch: reconnected`, nunca un segundo `connected`— y una línea de cuenta al salir.
  No hay heartbeat ni spinner: repetir en una terminal de append-only es tinta
  permanente, y el silencio ya está contestado por los avisos de reconexión y por el
  tiempo de la última tarjeta.
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
  reconexiones lo llena `feed history`, no el stream. `--backfill` no cambia eso:
  cubre la primera conexión, no las siguientes, y nunca pide el historial dos veces.
  Fingir continuidad sería mentir sobre lo que el relay guarda.
- **El interior no conoce el proceso.** `cli.Run` recibe `args`, `stdout` y `stderr`
  como parámetros y devuelve el código; nadie adentro toca `os.Args` ni `os.Exit`.
- **Cero dependencias.** Si `go.mod` gana un `require`, alguien rompió ADR-0008 sin
  decirlo. Es barato de chequear y es una propiedad, no un detalle.
- **El módulo está anidado.** `go build ./...` no corre desde la raíz del repo: hay
  que entrar en `client/` (ADR-0009).

## 7. Qué no está en este mapa

- **El relay** — tiene su propio mapa en [`docs/relay-flow.md`](relay-flow.md).
- **Los tickets del CLI**: 06 (la primera conexión no es una pantalla en blanco),
  07 (el historial renderizado y sus filtros) y 08 (distribuir el binario) están
  cerrados, y este mapa describe el resultado. Lo que sigue abierto es CI.
- **CI y release**: no hay workflow que compile el cliente ni publique artefactos;
  los binarios salen de `npm run cli:dist`, a mano.
- **El detalle de los colores**: la paleta por rol vive en `render.go` —cian para las
  personas, verde para la integración, amarillo para la asignación, azul para la rama
  —la ubicación que importa—, magenta para las etiquetas —la única nota cálida— y gris
  para el repositorio, el conector y el tiempo— y no cambia el flujo.
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
4. `client/internal/cli/history.go` y `stored.go` — el otro camino: una request, el
   mismo filtro y las tres salidas.
5. `client/internal/render/render.go` — cómo un evento se vuelve una tarjeta.
6. `client/internal/cli/cli.go` — el despacho, los códigos de salida y el contrato de
   stdout/stderr.
7. `client/test/e2e_test.go` y `harness_test.go` — la suite es la especificación
   ejecutable: un `fakeRelay` de `httptest` y binarios reales, nunca imports internos.

Para ejecutarlo con tus ojos, desde `client/`:

```bash
go build -o feed ./cmd/feed          # o, desde la raíz: npm run cli:build
./feed watch   --relay http://localhost:3000
./feed watch   --relay http://localhost:3000 --backfill 20
./feed history --relay http://localhost:3000
./feed history --relay http://localhost:3000 --json
./feed history --relay http://localhost:3000 --group-by-repo
GOOS=windows GOARCH=amd64 go build -o feed.exe ./cmd/feed
```

El relay tiene que estar arriba; los pasos para levantarlo y alimentarlo con un
webhook firmado están en [`docs/manual-testing.md`](manual-testing.md). En Windows el
binario es `feed.exe`, y `go test ./...` compila los dos binarios de prueba y tarda
alrededor de dos minutos.

El recorrido a mano, caso por caso y con la salida esperada de cada uno, está en
[`docs/cli-testing.md`](cli-testing.md); el del sistema completo —relay y CLI
hablándose— en [`docs/end-to-end-testing.md`](end-to-end-testing.md).

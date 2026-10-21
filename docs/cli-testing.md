# Probar el CLI a mano

Guía para ejecutar el binario de `client/` y verificar con las manos lo que los
tickets 01 a 05 construyeron. La suite e2e prueba lo mismo de forma automatizada,
compilando el binario real y hablándole por HTTP con un relay falso; esto es para
cuando querés ver el cable, probar tu propio relay o mostrarle el sistema a alguien.

Si lo que querés es probar el sistema completo —relay incluido—, el recorrido está
en [`docs/end-to-end-testing.md`](end-to-end-testing.md).

## Antes de empezar: tres ideas

1. **stdout lleva eventos; stderr lleva todo lo demás.** El banner `watching <url>`,
   los avisos de reconexión y los errores van a stderr. Si mezclás los dos, no podés
   distinguir un evento de un aviso, y `feed watch | jq` deja de funcionar.
2. **El CLI no decide qué es crítico.** El relay ya filtró: si un evento llegó al
   stream, es crítico. Lo que el CLI filtra es *para quién*, porque el relay hace
   broadcast y manda todo a todos (ADR-0001).
3. **El stream es en vivo y no repite.** Si te conectás después de que el evento se
   publicó, no lo ves. Para eso está `feed history` —hoy sólo en su forma `--json`—.

## Paso 0 — Compilar

```bash
cd client
go build -o feed ./cmd/feed
```

En Windows el binario es `feed.exe` y se invoca `.\feed.exe`; los comandos de abajo
usan `./feed` y funcionan igual en bash, zsh y PowerShell.

Para que `--version` diga algo distinto de `dev`, compilá como se compila un binario
distribuido:

```bash
go build -ldflags "-X main.version=v0.1.0" -o feed ./cmd/feed
```

No hace falta Node ni el relay para los pasos 1 y 2.

## Paso 1 — La versión

```bash
./feed --version
```

```text
v0.1.0
```

Sale con `0` y no escribe nada en stderr. La versión entra por `-ldflags`, que es el
mismo mecanismo que va a usar el binario distribuido (ticket 08): si ves `dev`, el
`-X` no llegó y estás mirando un build de desarrollo.

## Paso 2 — Uso y códigos de salida

| Comando | Salida | stdout | stderr |
| --- | --- | --- | --- |
| `./feed` | `2` | vacío | el uso completo |
| `./feed bogus` | `2` | vacío | `feed: unknown command "bogus"` y el uso |
| `./feed watch --relay ://bad` | `1` | vacío | `watching ://bad/stream` y `parse "://bad/stream": missing protocol scheme` |
| `./feed history --relay http://127.0.0.1:1 --json` | `1` | vacío | `history: Get "…": … connection refused` |
| `./feed history --relay http://127.0.0.1:1 --json --group-by-repo` | `2` | vacío | `history: --json and --group-by-repo cannot be combined: grouping is presentation, and raw output has none` |

Dos cosas que esta tabla enseña y conviene mirar de cerca:

- **Una URL inusable es fatal, no reintentable.** `://bad` sale con `1` y sin una
  sola línea de `reconnecting`. Un relay apagado, en cambio, se reintenta para
  siempre: esa diferencia es deliberada.
- **El conflicto de flags se detecta sin tocar la red.** El último caso no hace
  ningún request: es un error de uso, no del relay.

Para ver el código de salida: en bash `echo $?` después del comando; en PowerShell
`$LASTEXITCODE`.

## Paso 3 — Levantar el relay

Los pasos 4 a 11 necesitan un relay vivo. El recorrido completo está en
[`docs/manual-testing.md`](manual-testing.md#paso-1--levantar-el-servidor); en corto:

```powershell
$env:GITHUB_WEBHOOK_SECRET = "un-secreto-largo-de-prueba"
npm.cmd start
```

```bash
GITHUB_WEBHOOK_SECRET=un-secreto-largo-de-prueba npm start
```

El relay escucha en `http://localhost:3000` por defecto. Si usás otro puerto, pasalo
también al CLI con `--relay`.

## Paso 4 — Ver un evento en vivo

En una terminal:

```bash
./feed watch --relay http://localhost:3000 --no-color
```

En otra, mandá el webhook de un merge a `main` (está completo, con la firma, en
[`manual-testing.md`](manual-testing.md#paso-3--mandar-un-webhook-firmado)):

```powershell
$secret = "un-secreto-largo-de-prueba"
$body = '{"action":"closed","sender":{"login":"maria.perez"},"repository":{"full_name":"AG2904/github-activity-feed"},"pull_request":{"title":"Fix memory leak in parser component","merged":true,"merged_at":"2026-10-16T12:00:00Z","html_url":"https://github.com/AG2904/github-activity-feed/pull/42","base":{"ref":"main"},"labels":[{"name":"bug"}]}}'

$h = New-Object System.Security.Cryptography.HMACSHA256
$h.Key = [Text.Encoding]::UTF8.GetBytes($secret)
$sig = 'sha256=' + (($h.ComputeHash([Text.Encoding]::UTF8.GetBytes($body)) | ForEach-Object { $_.ToString('x2') }) -join '')

Invoke-WebRequest -Uri http://localhost:3000/ -Method Post -UseBasicParsing `
  -Headers @{ 'X-Hub-Signature-256' = $sig; 'X-GitHub-Event' = 'pull_request'; 'X-GitHub-Delivery' = 'cli-1'; 'Content-Type' = 'application/json' } `
  -Body $body
```

La terminal del CLI muestra, en stdout:

```text
maria.perez merged into main
  Fix memory leak in parser component
  AG2904/github-activity-feed · [bug] · 2h ago
  https://github.com/AG2904/github-activity-feed/pull/42
```

Y en stderr, desde el arranque:

```text
watching http://localhost:3000/stream
```

Detalles que importan: una tarjeta son tres líneas —la frase, el título y el
contexto— más una cuarta con la URL cuando no hay hyperlink, que es el caso de un pipe
o de `--no-color`. `2h ago` es **relativo al momento de imprimir**, no al `merged_at`.
Las etiquetas entre corchetes son las del evento y son la razón por la que una
asignación te interrumpió. En una terminal con color, el título es el link y la línea
de la URL desaparece.

## Paso 5 — `--json`: el contrato crudo

```bash
./feed watch --relay http://localhost:3000 --json
```

Cada evento sale como **una línea JSON idéntica al payload del relay**, sin
reformatear. Para probar que no re-serializa, mandá un evento cuyo `subject` tenga un
`&`:

```text
{"type":"pr.merged","repository":"AG2904/github-activity-feed","actor":"maria.perez","subject":"Fix parser & renderer",…}
```

El `&` tiene que seguir siendo `&`. Si ves `\u0026`, alguien pasó el payload por
`encoding/json` y rompió el contrato.

## Paso 6 — Los filtros locales

Con `--user` y `--repo` el CLI descarta eventos **después** de recibirlos; el relay
sigue mandando todo.

| Caso | Comando | Qué esperar |
| --- | --- | --- |
| actor | `./feed watch --user maria.perez` | sólo los eventos donde maria es `actor` |
| assignee | `./feed watch --user tomas.ferreira` | sólo la asignación donde tomas es `assignee` |
| nadie | `./feed watch --user nadie` | nada, y el stream sigue abierto |
| mayúsculas | `./feed watch --user Maria.Perez` | **nada**: la comparación es exacta y sensible a mayúsculas |
| repositorio | `./feed watch --repo AG2904/github-activity-feed` | sólo los eventos de ese repo |
| otro repo | `./feed watch --repo other/repo` | nada |
| combinados | `./feed watch --user lucia.gomez --repo other/repo` | los dos filtros son un AND |

El caso del assignee es el que más se olvida: `--user` significa **involucramiento**,
actor *o* assignee, la misma regla que `/history?user=` del relay. Si las dos reglas
divergieran, `feed watch --user` y el historial contestarían cosas distintas sobre la
misma persona.

## Paso 7 — Lo que se descarta

Un frame que no es un evento —`id:`, `event:`, `retry:`, un comentario de heartbeat,
un JSON roto— se ignora y el CLI sigue leyendo. Contra el relay real no podés
provocar eso, porque el relay no manda basura: ese caso lo cubre la suite, en
`TestWatchDiscardsFramesItCannotUseAndKeepsReading`, con un relay falso que mezcla
frames válidos y basura y después exige que lleguen exactamente los dos válidos.

Lo que sí podés ver a mano es la otra mitad de la regla: un frame basura **no corta la
conexión**. Si el siguiente evento llega, se imprime.

## Paso 8 — El relay se cae y vuelve

Con el `watch` corriendo, matá el relay (`Ctrl-C` en su terminal). El CLI no muere:

```text
watch: reconnecting in 1.223085495s: read tcp …: An existing connection was forcibly closed by the remote host.
watch: reconnecting in 2.042726645s: Get "http://localhost:3000/stream": … connection refused
watch: reconnecting in 4.783724298s: …
watch: reconnecting in 8.827214592s: …
```

Los delays son la escalera de backoff: un segundo, dos, cuatro, ocho…, con un jitter
de hasta un cuarto del paso, y un techo de treinta segundos. Levantá el relay de nuevo
y en la siguiente ventana aparece:

```text
watch: reconnected
```

Mandá otro webhook: se imprime. Los eventos que pasaron mientras estaba desconectado
**no** se reproducen —el stream no guarda historia—, y el CLI tampoco manda
`Last-Event-ID`: el contador `id:` de Nest arranca en cero en cada conexión y no es
una posición de reanudación (ADR-0007). Los eventos perdidos se recuperan con
`feed history`.

**El reset del backoff** es el otro comportamiento: si el relay entregó al menos un
evento válido, la escalera vuelve a un segundo. Matá el relay justo después de un
evento y el próximo `reconnecting in` va a volver a ~1 s.

## Paso 9 — El silencio: no esperes cinco minutos

`idleReader` declara muerta una conexión que no entrega ni un byte en cinco minutos.
Esperar cinco minutos a mano es absurdo, así que el binario tiene una costura de
test: `idleTimeoutSetting` se inyecta por `-ldflags` y no es una flag pública.

Compilá una copia con dos segundos de timeout:

```bash
cd client
go build -trimpath \
  -ldflags "-X github.com/AG2904/github-activity-feed/client/internal/cli.idleTimeoutSetting=2s" \
  -o feed-idle ./cmd/feed
```

Levantá un relay que acepte la conexión y no escriba nada:

```js
// silent-relay.js — acepta /stream y nunca escribe un byte.
const http = require('http');
http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': 'text/event-stream' });
  res.flushHeaders();
}).listen(4100);
```

```bash
node silent-relay.js
./feed-idle watch --relay http://localhost:4100
```

```text
watch: reconnecting in 1.058510306s: no bytes for 2s
watch: reconnected
watch: reconnecting in 2.039285589s: no bytes for 2s
watch: reconnected
```

La razón se nombra: `no bytes for 2s`. Con el binario normal diría `no bytes for
5m0s`, y ese caso es justamente el que el heartbeat del relay evita: un evento o un
comentario cada quince segundos reinicia el timer. Para verlo, dejá un `watch` contra
el relay real veinte segundos sin mandar nada: no aparece ninguna línea nueva y el
stream sigue siendo el mismo.

## Paso 10 — El historial

```bash
./feed history --relay http://localhost:3000 --json > history.json
```

El archivo es **el body de `/history`, byte por byte**. Comparalo:

```powershell
(Invoke-WebRequest http://localhost:3000/history -UseBasicParsing).Content
```

El orden es `id DESC` —lo más nuevo primero— y no hay paginación.

El CLI hace el mismo pedido y filtra localmente. Lo que conviene saber al probarlo a
mano:

- **`./feed history` sin `--json` pinta el historial**, newest first, con la misma
  tarjeta que el vivo. Sale con `0`.
- **`--group-by-repo` agrupa** por repositorio en orden alfabético, y deja al final,
  sin encabezado, las filas sin repositorio.
- **`--user` y `--repo` filtran localmente**, con la misma regla que `watch`: un
  `--user` que no es actor ni assignee no aparece en el reporte.

## Paso 11 — El color

```bash
./feed watch --relay http://localhost:3000 --no-color
NO_COLOR=1 ./feed watch --relay http://localhost:3000
./feed watch --relay http://localhost:3000 | cat
```

Los tres casos tienen que imprimir texto plano, sin un solo `\x1b`. La regla es una
sola: hay color sólo si stdout es un terminal de verdad, no lo apagó `--no-color` y no
lo apagó `NO_COLOR`. Un pipe nunca tiene color, y ése es el motivo por el que
`feed watch | jq` es seguro.

En Windows, además, el color exige que la consola acepte secuencias ANSI: el binario
prende `ENABLE_VIRTUAL_TERMINAL_PROCESSING`; si la consola lo rechaza, no emite
escapes en vez de imprimir basura.

## Paso 12 — Ctrl-C

`Ctrl-C` cancela el contexto y el proceso sale con `0`: no es un error, es la forma
normal de terminar un watch. Un segundo `Ctrl-C` mata igual un run colgado, porque el
manejador se restaura apenas se cancela el contexto. La suite no automatiza un evento
de consola, así que este es el único paso que se verifica a mano.

## Matriz de casos por ticket

| Ticket | Caso | Paso | Automatizado |
| --- | --- | --- | --- |
| 01 — arrancar y llegar al relay | banner, precedencia `--relay` > `$RELAY_URL` > default | 3, 4 | sí |
| 01 — arrancar y llegar al relay | URL inusable es fatal | 2 | sí |
| 02 — pintar eventos en vivo | merge y asignación como tarjetas | 4 | sí |
| 02 — pintar eventos en vivo | sin color en un pipe | 11 | sí |
| 03 — el contrato crudo | `--json` byte por byte, sin `\u0026` | 5 | sí |
| 03 — el contrato crudo | basura descartada sin cortar el stream | 7 | sí |
| 04 — la conexión se sostiene | reintento indefinido y backoff creciente | 8 | sí |
| 04 — la conexión se sostiene | reset del backoff tras un evento | 8 | sí |
| 04 — la conexión se sostiene | `no bytes` y reapertura | 9 | sí |
| 04 — la conexión se sostiene | el heartbeat mantiene vivo el stream | 9 | sí |
| 05 — filtrar para sí | actor, assignee, repo, AND, mayúsculas | 6 | sí |
| 06 — la primera conexión no es una pantalla en blanco | — | — | **pendiente** |
| 07 — el historial responde | — | — | **pendiente** |
| 08 — distribuir el binario | `--version` inyectada | 1 | parcial (sólo la inyección) |

## Problemas frecuentes

| Síntoma | Causa |
| --- | --- |
| `feed: command not found` | el binario no está en `PATH`: usá `./feed` (o `.\feed.exe`) |
| `go run` devuelve siempre `1` | `go run` no propaga el código del binario y agrega `exit status N`; compilá y ejecutá el binario |
| `watch` no imprime el evento que acabás de mandar | te conectaste después: el stream no repite. Mandá otro, o mirá `/history` |
| nada llega y el relay está arriba | otro puerto: mirá el banner `watching <url>/stream` |
| stderr lleno de `At line:… CategoryInfo…` | PowerShell 5.1 decora el stderr de ejecutables nativos; el texto del CLI está ahí, con basura alrededor |
| `401` al mandar el webhook | la firma no corresponde al body exacto, o el secreto no es el del relay |
| el reloj dice `just now` para un evento viejo | `when` en el futuro o el evento recién guardado; el tiempo es relativo a `now` |

## Limpieza

`Ctrl-C` cierra el watch y el relay. El historial queda en `relay.db` (o en
`DATABASE_PATH`); para empezar de cero, borralo con el relay apagado. El binario
`feed` y `feed-idle` son artefactos locales: están en `.gitignore`.

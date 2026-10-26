# Probar el sistema completo

El recorrido de punta a punta: un webhook firmado entra al relay, se guarda en SQLite,
sale por el stream y aparece en el terminal del CLI. Esta guía prueba **las dos
mitades juntas**, que es lo único que ninguna de las dos suites automáticas puede
probar: cada una usa un doble del otro lado del cable.

- El relay solo: [`docs/manual-testing.md`](manual-testing.md).
- El CLI solo: [`docs/cli-testing.md`](cli-testing.md).
- El contrato entre los dos: [`docs/relay-flow.md`](relay-flow.md) y
  [`docs/cli-flow.md`](cli-flow.md).

## Camino corto

1. Compilá los dos lados: `npm run build` y `cd client && go build -o feed ./cmd/feed`.
2. Levantá el relay con un secreto y una base limpia.
3. En otra terminal: `./feed watch --relay http://localhost:3000 --json`.
4. Mandá los webhooks del [paso 4](#paso-4--el-recorrido-completo).
5. Esperá **tres** líneas en el CLI y **tres** filas en `/history`. Si coinciden, el
   sistema completo funciona.

## Antes de empezar

| Pieza | Verificado en esta máquina |
| --- | --- |
| Node + npm | v26.8.2 / npm 12.2.0 |
| Go | 1.27.1 (`go.mod` pide 1.27.0) |
| Relay | build limpio, 54 tests e2e en ~14 s |
| CLI | build limpio, `go test ./...` en ~2 min |
| Puertos | `3000` libre; el CLI asume `http://localhost:3000` |

Tres ideas que ordenan todo lo que sigue:

1. **La respuesta del relay no dice si guardó.** `POST /` contesta `200
   {"received":true}` para un evento guardado, uno ignorado y uno duplicado. El
   oráculo es siempre `/history`.
2. **El stream es en vivo.** Lo que se publicó antes de que el CLI se conectara no se
   reproduce.
3. **El relay no sabe quién sos.** Manda todo a todos; el filtro por usuario o repo
   es del CLI (ADR-0001).

## Paso 1 — Compilar los dos lados

```powershell
npm run build
```

```bash
cd client && go build -o feed ./cmd/feed && cd ..
```

En Windows el binario queda en `client\feed.exe`. Si `npm run build` cuelga en tu
máquina, son los shims locales de la raíz: en este repo ya no están, y el arreglo es
borrarlos, no rodearlos.

## Paso 2 — Relay con base limpia

```powershell
$env:GITHUB_WEBHOOK_SECRET = "un-secreto-largo-de-prueba"
$env:DATABASE_PATH = "./sistema.db"
Remove-Item ./sistema.db -ErrorAction SilentlyContinue
npm start
```

```bash
GITHUB_WEBHOOK_SECRET=un-secreto-largo-de-prueba DATABASE_PATH=./sistema.db npm start
```

`GITHUB_WEBHOOK_SECRET` es obligatoria: sin ella el relay se niega a arrancar.
`DATABASE_PATH` es opcional y por defecto es `relay.db`. El relay carga `.env` al
arrancar con el loader nativo de Node, pero las variables exportadas en la shell
ganan: por eso las de arriba se aplican aunque exista un `.env`.

Confirmá que está arriba antes de seguir:

```bash
curl http://localhost:3000/history
```

```json
[]
```

## Paso 3 — El script que firma y envía

Todo el recorrido usa el mismo helper. Pegalo en la terminal de PowerShell que va a
mandar los webhooks:

```powershell
$secret = "un-secreto-largo-de-prueba"

function Send-Webhook($body, $event, $delivery) {
  $h = New-Object System.Security.Cryptography.HMACSHA256
  $h.Key = [Text.Encoding]::UTF8.GetBytes($secret)
  $sig = 'sha256=' + (($h.ComputeHash([Text.Encoding]::UTF8.GetBytes($body)) |
    ForEach-Object { $_.ToString('x2') }) -join '')

  Invoke-WebRequest -Uri http://localhost:3000/ -Method Post -UseBasicParsing -Headers @{
    'X-Hub-Signature-256' = $sig
    'X-GitHub-Event'      = $event
    'X-GitHub-Delivery'   = $delivery
    'Content-Type'        = 'application/json'
  } -Body $body
}
```

Lo que importa del helper:

- **La firma es sobre los bytes exactos del body.** Si reformateás el JSON entre
  firmar y enviar, la firma deja de servir (ADR-0002).
- **`X-GitHub-Delivery` es la identidad de la entrega**, no del evento. Es lo que el
  relay deduplica: dos envíos con el mismo valor son una sola fila.
- **`X-GitHub-Event` decide cómo se lee el payload.** Para un merge es
  `pull_request`; para una asignación, `issues`.

En bash el equivalente para firmar es `printf '%s' "$body" | openssl dgst -sha256
-hmac "$secret" | sed 's/^.* //'` y después `curl` con el header `sha256=<hex>`. Usá
`printf`, nunca `echo`: el salto de línea final cambia el HMAC. *(Este bloque no se
probó en esta máquina, que no tiene bash; el de PowerShell sí.)*

## Paso 4 — El recorrido completo

Abrí una segunda terminal y dejá el CLI conectado **antes** de mandar nada:

```bash
cd client
./feed watch --relay http://localhost:3000 --json
```

`--json` primero: muestra el contrato sin color, sin tiempo relativo y sin hyperlinks,
así lo que ves se puede comparar byte a byte con `/history`. El renderizado en
tarjetas se prueba después, en el paso 6.

En stderr, la sesión se nombra a sí misma: el banner `watching <url> · backfill 10`
y, como la base arranca vacía, `connected · waiting for events`. Con una base que ya
tiene historia, la segunda línea cuenta lo que imprimió: `connected · <n> events from
history`.

Con los dos procesos vivos, en la terminal del helper:

```powershell
$when = "2026-10-16T12:00:00Z"

$merge = '{"action":"closed","sender":{"login":"maria.perez"},"repository":{"full_name":"AG2904/github-activity-feed"},"pull_request":{"title":"Fix memory leak in parser component","merged":true,"merged_at":"' + $when + '","html_url":"https://github.com/AG2904/github-activity-feed/pull/42","base":{"ref":"main"},"head":{"ref":"fix/memory-leak"},"labels":[{"name":"bug"}]}}'

$assignment = '{"action":"assigned","sender":{"login":"lucia.gomez"},"assignee":{"login":"tomas.ferreira"},"repository":{"full_name":"AG2904/github-activity-feed"},"issue":{"title":"Ship the onboarding checklist","html_url":"https://github.com/AG2904/github-activity-feed/issues/7","updated_at":"' + $when + '","labels":[{"name":"bug"}],"assignees":[{"login":"tomas.ferreira"}]}}'

$otherRepo = '{"action":"closed","sender":{"login":"someone.else"},"repository":{"full_name":"other/repo"},"pull_request":{"title":"Other repo merge","merged":true,"merged_at":"' + $when + '","html_url":"https://github.com/other/repo/pull/3","base":{"ref":"main"},"head":{"ref":"other-work"},"labels":[]}}'

$develop = '{"action":"closed","sender":{"login":"maria.perez"},"repository":{"full_name":"AG2904/github-activity-feed"},"pull_request":{"title":"Merge into develop","merged":true,"merged_at":"' + $when + '","html_url":"https://github.com/AG2904/github-activity-feed/pull/43","base":{"ref":"develop"},"head":{"ref":"feature-x"},"labels":[{"name":"bug"}]}}'

Send-Webhook $merge      'pull_request' 'e2e-1'   # 1. merge crítico
Send-Webhook $merge      'pull_request' 'e2e-1'   # 2. la misma entrega, otra vez
Send-Webhook $develop    'pull_request' 'e2e-2'   # 3. merge crítico a otra rama
Send-Webhook $assignment 'issues'       'e2e-3'   # 4. asignación con label crítica
Send-Webhook $otherRepo  'pull_request' 'e2e-4'   # 5. otro repositorio
```

### Qué mirar

En el CLI, exactamente una línea JSON por caso 1 y 4 —la duplicada no se imprime— y
una más por el merge de `other/repo`:

```text
{"type":"pr.merged","repository":"AG2904/github-activity-feed","branch":"fix/memory-leak","actor":"maria.perez","subject":"Fix memory leak in parser component","assignee":null,"labels":["bug"],…}
{"type":"issue.assigned","repository":"AG2904/github-activity-feed","branch":null,"actor":"lucia.gomez","subject":"Ship the onboarding checklist","assignee":"tomas.ferreira","labels":["bug"],…}
{"type":"pr.merged","repository":"other/repo","branch":"other-work","actor":"someone.else","subject":"Other repo merge","assignee":null,"labels":[],…}
```

Y en `/history`, **tres** filas, la más nueva primero (el merge de `other/repo`):

| Caso | Respuesta del relay | ¿Fila nueva? | ¿Línea en el CLI? |
| --- | --- | --- | --- |
| 1. merge a `main`, entrega nueva | `200 {"received":true}` | sí | sí |
| 2. la misma entrega otra vez | `200 {"received":true}` | **no** | **no** |
| 3. merge a otra rama | `200 {"received":true}` | no | no |
| 4. asignación con `bug` | `200 {"received":true}` | sí | sí |
| 5. merge en otro repo | `200 {"received":true}` | sí | sí |

La fila 2 es la que hace visible la idempotencia (ADR-0005), y la 5 la que hace
visible el broadcast: el relay no filtra por repositorio, el CLI sí.

## Paso 5 — El filtro es del cliente

Dejá el `watch --json` del paso 4 corriendo y abrí un segundo:

```bash
./feed watch --relay http://localhost:3000 --repo other/repo --json
```

Mandá un merge nuevo a `AG2904/github-activity-feed` con `X-GitHub-Delivery: e2e-5`:

- el primer cliente lo imprime,
- el segundo **no** imprime nada y sigue conectado,
- `/history` tiene la fila para los dos.

El relay entregó el evento a ambos. El filtro es local, y por eso `feed watch --user`
puede cambiar sin tocar el relay.

## Paso 6 — El renderizado en tarjetas

El mismo evento, con el CLI en modo terminal:

```bash
./feed watch --relay http://localhost:3000 --no-color
```

```text
maria.perez merged fix/memory-leak into main
  [bug] Fix memory leak in parser component
  AG2904/github-activity-feed · 2h ago
  https://github.com/AG2904/github-activity-feed/pull/42

lucia.gomez assigned to tomas.ferreira
  [bug] Ship the onboarding checklist
  AG2904/github-activity-feed · 2h ago
  https://github.com/AG2904/github-activity-feed/issues/7
```

`2h ago` es relativo al momento de imprimir: con el `when` del ejemplo vas a ver los
días que hayan pasado desde el 16 de octubre, no siempre `2h ago`. La frase del merge
nombra la rama de origen (`head.ref`); una fila guardada antes de que existiera el
campo, o un payload sin `head.ref` usable, dice `maria.perez merged into main`
(ADR-0013). Las etiquetas del evento van al frente del título, antes del texto que
califican, y el contexto se queda con el repositorio y el tiempo. La cuarta línea, la
URL, existe sólo sin hyperlink: en una terminal con color el título es el link OSC 8 y
la tarjeta se queda en tres líneas. En un pipe no hay un solo escape.

Las tarjetas van a stdout; la sesión entera va a stderr. Al cerrar con `Ctrl-C`, la
línea de cuenta suma lo que se mostró, filtro incluido —en este recorrido,
`stopped · 2 events · 1 merged · 1 assigned`—, y `stopped · no events` cuando no se
mostró nada.

## Paso 7 — Lo que no se repite

Cerrá el CLI (`Ctrl-C`), mandá un webhook nuevo y volvé a conectarte:

- el evento nuevo **no** aparece al reconectar: el stream no guarda historia y el CLI
  no manda `Last-Event-ID` (ADR-0007);
- `/history` sí lo tiene.

Ese hueco es el contrato, no un bug: el historial es la memoria, el stream es el
presente.

## Paso 8 — El relay se cae con el cliente conectado

Con un `watch` corriendo, `Ctrl-C` en la terminal del relay. El CLI no muere:

```text
watch: reconnecting in 1.223085495s: read tcp …: An existing connection was forcibly closed by the remote host.
watch: reconnecting in 2.042726645s: Get "http://localhost:3000/stream": … connection refused
watch: reconnecting in 4.783724298s: …
```

Volvé a levantarlo (mismo `DATABASE_PATH`) y en la siguiente ventana aparece
`watch: reconnected`. Mandá un webhook: llega. Los que se mandaron con el relay caído
no existen —nunca entraron—, y los que entraron mientras el CLI estaba desconectado
sólo están en `/history`.

## Paso 9 — La persistencia es del relay

Pará el relay y levantalo de nuevo con el mismo `DATABASE_PATH`:

```bash
curl http://localhost:3000/history
```

Las filas siguen ahí. Con otro `DATABASE_PATH`, la base es otra y `/history` arranca
vacío. El CLI no guarda nada: es un cliente.

## Paso 10 — El silencio no rompe nada

Dejá el `watch` conectado veinte o treinta segundos sin mandar webhooks.

- No aparece ninguna línea nueva (el heartbeat del relay es un comentario SSE, y el
  CLI los ignora).
- No aparece `reconnecting`: un comentario cada quince segundos reinicia el timer de
  cinco minutos del cliente.
- No aparece un heartbeat ni un spinner del CLI: la única línea afirmativa de la
  sesión fue `connected · …` al conectar, y el silencio que sigue se queda en
  silencio. La reconexión ya contesta a "¿sigue vivo?"; repetir una línea de
  "acá estoy" en una terminal de append-only es spam con fecha de vencimiento.
- El siguiente evento se imprime en la misma conexión.

## Paso 11 — Las dos suites

```powershell
npm test
```

```bash
cd client && go test ./...
```

| Suite | Qué cubre | Tiempo |
| --- | --- | --- |
| `npm test` — 54 casos | el relay sobre su frontera HTTP: firma, filtro, dedupe, stream, historial | ~14 s |
| `go test ./...` | el CLI sobre la costura del proceso: compila dos binarios y les habla con un relay falso | ~2 min |

Ninguna necesita la otra mitad: la suite del relay no arranca el CLI y la del CLI no
arranca el relay. Eso es deliberado y también es el límite: **el único lugar donde el
relay real y el CLI real se hablan es este documento**.

## Paso 12 — GitHub de verdad (opcional)

Hasta acá todo fue simulado, que es suficiente porque la firma es la única
autenticación. Si querés el circuito real, hace falta una URL pública.

1. Expuesto el relay: `cloudflared tunnel --url http://localhost:3000` (o `ngrok http
   3000`). Ninguna de las dos está instalada en esta máquina, así que este paso **no
   está verificado acá**. El concepto, las opciones y los costos están en
   [`docs/exposing-the-relay.md`](exposing-the-relay.md).
2. En el repo de prueba: **Settings → Webhooks → Add webhook**.
   - Payload URL: la URL del túnel, terminada en `/`.
   - Content type: `application/json`.
   - Secret: el mismo `GITHUB_WEBHOOK_SECRET`.
   - SSL verification: activa.
   - Eventos: *Let me select individual events* → **Pull requests** e **Issues**.
3. Con el CLI escuchando, hacé un merge real a `main` en ese repo. En **Recent
   Deliveries** la entrega tiene que mostrar `200`, y el CLI la imprime.
4. **Redeliver** desde la UI de GitHub manda la misma entrega con el mismo
   `X-GitHub-Delivery`: es la prueba de idempotencia con tráfico real, y no debe
   duplicar ni la fila ni la línea.

Si el relay es de una instancia corporativa, la URL del túnel tiene que ser alcanzable
desde esa instancia; si no, la entrega queda en rojo en GitHub y el relay nunca se
entera.

## Qué prueba cada paso

| Propiedad del sistema | Paso | ¿Automatizado? |
| --- | --- | --- |
| el webhook firmado entra y se guarda | 4 | sí, por separado |
| un evento no crítico se acusa y se descarta | 4 | sí, por separado |
| una entrega repetida no duplica | 4 | sí, por separado |
| el guardado llega al stream | 4 | sí, por separado |
| el filtro es del cliente, no del relay | 5 | sí, por separado |
| la forma de 9 campos es la misma en stream e historial | 4, 6 | sí, por separado |
| la reconexión no rellena el hueco | 7, 8 | sí, por separado |
| el heartbeat mantiene viva la conexión | 10 | sí (relay y CLI por su lado) |
| el historial sobrevive al reinicio | 9 | sí (relay) |
| **relay real + CLI real, juntos** | todos | **no: este documento** |

## Qué no prueba este documento

- **El historial renderizado** y sus filtros en el CLI: ticket 07, cerrado, pero esta
  guía no lo recorre; se queda en la salida cruda contra el relay real.
- **La distribución del binario**: ticket 08, cerrado; acá sólo se prueba la versión
  inyectada, no los cuatro binarios de `cli:dist`.
- **CI**: no hay workflow que corra las dos suites.
- **Retención y purgado**: los eventos se guardan indefinidamente.
- **Varios relays o alta disponibilidad**: hay un proceso y un archivo SQLite.
- **Autenticación de lectura**: `/history` y `/stream` son abiertos por diseño.

## Limpieza

`Ctrl-C` en las tres terminales. Después, con el relay apagado:

```powershell
Remove-Item ./sistema.db
```

```bash
rm -f sistema.db
```

El binario `client/feed` queda como artefacto local; está en `.gitignore`.

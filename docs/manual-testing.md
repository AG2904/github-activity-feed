# Probar el relay a mano

Guía para levantar el relay y verificar con las manos lo que los tickets 01 a 07
construyeron. La suite e2e prueba lo mismo de forma automatizada (48 casos sobre la
misma seam HTTP); esto es para cuando querés ver el cable, probar un payload propio o
mostrarle el sistema a alguien.

La segunda mitad de este documento hace lo mismo del otro lado: el binario `feed`
contra el relay, con los eventos apareciendo en la terminal.

## Antes de empezar: las dos ideas que hay que entender

1. **La firma es sobre los bytes exactos del body.** `X-Hub-Signature-256` lleva
   `sha256=` más el HMAC-SHA256 en hex, calculado con el secreto sobre el cuerpo tal
   como se envía. Si cambiás el body, la firma cambia: no se puede hardcodear el
   header (ADR-0002).
2. **`POST /` responde `200` aunque descarte el evento.** Ese `200 {"received":true}`
   significa "recibido", no "guardado". El único otro código es `401`, y es para la
   firma. **La verdad está siempre en `/history`** — es el oráculo de todas las
   pruebas de abajo.

## Paso 1 — Levantar el servidor

| Variable | ¿Obligatoria? | Si falta |
| --- | --- | --- |
| `GITHUB_WEBHOOK_SECRET` | **Sí** | El proceso no arranca |
| `DATABASE_PATH` | No | Usa `relay.db` en el directorio actual |
| `PORT` | No | Usa `3000` |

PowerShell:

```powershell
$env:GITHUB_WEBHOOK_SECRET = "un-secreto-largo-de-prueba"
npm start
```

Bash:

```bash
GITHUB_WEBHOOK_SECRET=un-secreto-largo-de-prueba npm start
```

El secreto es tuyo: cualquier texto largo sirve, siempre que sea el mismo que uses
para firmar. No se commitea.

**Aviso sobre `.env`:** el proyecto no tiene `dotenv`, así que `npm start` **no lee**
`.env` aunque `.env.example` diga lo contrario. O pasás las variables por la shell
(lo de arriba), o usás `npm start -- --env-file .env`, que sí lo carga porque el flag
es del CLI de Nest.

**Si `npm start` falla con `'npx' is not recognized`:** en el directorio raíz hay
shims locales (`.cmd`, ignorados por git) que tapan a los binarios reales. Se
esquivan llamando al CLI por ruta: `node ./node_modules/@nestjs/cli/bin/nest.js start`,
o compilando y arrancando el JavaScript: `npm run build && node dist/main.js`.

## Paso 2 — Confirmar que responde

```bash
curl http://localhost:3000/history
```

Con la base vacía:

```json
[]
```

Si ves eso, el relay está arriba y escuchando. Cualquier otro resultado (conexión
rechazada, `500`) es un problema de arranque, no de la prueba.

## Paso 3 — Mandar un webhook firmado

El body se manda **raw**, sin reformatear entre firmar y enviar. Este es el de un
merge a `main`, que es un evento crítico:

```json
{"action":"closed","sender":{"login":"maria.perez"},"repository":{"full_name":"AG2904/github-activity-feed"},"pull_request":{"title":"Prueba manual","merged":true,"merged_at":"2026-10-16T12:00:00Z","html_url":"https://github.com/AG2904/github-activity-feed/pull/998","base":{"ref":"main"},"labels":[{"name":"bug"}]}}
```

### Opción A — PowerShell (probado)

```powershell
$secret = "un-secreto-largo-de-prueba"
$body = '{"action":"closed","sender":{"login":"maria.perez"},"repository":{"full_name":"AG2904/github-activity-feed"},"pull_request":{"title":"Prueba manual","merged":true,"merged_at":"2026-10-16T12:00:00Z","html_url":"https://github.com/AG2904/github-activity-feed/pull/998","base":{"ref":"main"},"labels":[{"name":"bug"}]}}'

$h = New-Object System.Security.Cryptography.HMACSHA256
$h.Key = [Text.Encoding]::UTF8.GetBytes($secret)
$sig = 'sha256=' + (($h.ComputeHash([Text.Encoding]::UTF8.GetBytes($body)) | ForEach-Object { $_.ToString('x2') }) -join '')

Invoke-WebRequest -Uri http://localhost:3000/ -Method Post -UseBasicParsing `
  -Headers @{ 'X-Hub-Signature-256' = $sig; 'X-GitHub-Event' = 'pull_request'; 'X-GitHub-Delivery' = 'prueba-1'; 'Content-Type' = 'application/json' } `
  -Body $body
```

Cuidado con las comillas: en PowerShell las simples `'...'` no expanden variables, y el
body tiene comillas dobles adentro, así que tiene que ir entre simples.

### Opción B — Postman

1. Un Environment con `baseUrl` = `http://localhost:3000` y `secret` = tu secreto.
2. `POST {{baseUrl}}/`, Body → **raw → JSON** con el body de arriba.
3. Headers: `X-Hub-Signature-256` (lo completa el script), `X-GitHub-Event:
   pull_request`, `X-GitHub-Delivery: {{$guid}}`, `Content-Type: application/json`.
4. En **Pre-request Script**:

```js
const body = pm.request.body.raw;
pm.request.headers.upsert({
  key: 'X-Hub-Signature-256',
  value: 'sha256=' + CryptoJS.HmacSHA256(body, pm.environment.get('secret')).toString(),
});
```

El script recalcula la firma en cada envío: si editás el body, la firma sigue válida.
`{{$guid}}` genera un identificador de entrega nuevo por request, así que cada envío
guarda un evento; para probar el duplicado, poné un valor fijo.

### Cualquier otro cliente

Sirve cualquiera (curl, HTTPie, un script) mientras la firma sea el HMAC-SHA256 en hex
del body exacto, con el prefijo `sha256=` en `X-Hub-Signature-256`. El `Content-Type`
no importa: el relay lee bytes crudos y parsea él.

## Paso 4 — Ver el evento guardado

Respuesta del POST:

```text
200 {"received":true}
```

Y ahora sí, el oráculo:

```bash
curl http://localhost:3000/history
```

```json
[{"type":"pr.merged","repository":"AG2904/github-activity-feed","actor":"maria.perez","subject":"Prueba manual","assignee":null,"labels":["bug"],"url":"https://github.com/AG2904/github-activity-feed/pull/998","when":"2026-10-16T12:00:00.000Z"}]
```

Ocho campos, ni uno más: el identificador de entrega quedó en la fila y no se sirve
(ADR-0005). El `repository` sale del `repository.full_name` del payload, no del `url`
(ADR-0010). El `when` es el `merged_at` que mandaste, normalizado a UTC con
milisegundos.

## Paso 5 — El canal en vivo

```bash
curl -N http://localhost:3000/stream
```

`-N` desactiva el buffering de curl: sin él, curl junta la respuesta y no ves nada
hasta que la conexión se cierre, y esta conexión no se cierra. Ese es el detalle
importante del paso, y vale para cualquier cliente: **usá uno que muestre la respuesta
a medida que llega**. `Invoke-WebRequest` de PowerShell, por ejemplo, se cuelga
esperando que el body termine.

Con el stream abierto, mandá otro webhook desde otra terminal (paso 3) y vas a ver el
frame:

```text
id: 1
data: {"type":"pr.merged","repository":"AG2904/github-activity-feed","actor":"maria.perez","subject":"Prueba manual","assignee":null,"labels":["bug"],"url":"https://github.com/AG2904/github-activity-feed/pull/998","when":"2026-10-16T12:00:00.000Z"}
```

Es el mismo evento que sirve `/history`, con el mismo shape, para que el cliente
parsee una sola cosa (user story 16). El canal es en vivo: lo que se guardó antes de
que te conectaras no se reproduce.

## Paso 6 — Los casos que demuestran cada comportamiento

Cada fila corresponde a un caso que la suite e2e ya prueba; repetirlos a mano es para
ver el cable. Después de cada uno, mirá `/history`.

| Qué probás | Cómo | Qué esperar |
| --- | --- | --- |
| Firma inválida (ticket 01) | el mismo POST con otro `$secret` | `401` y `/history` sin cambios |
| Merge a `main` (02) | el body del paso 3 | `200 {"received":true}` y una fila |
| Merge a otra rama (02) | `"base": {"ref":"develop"}` | `200` y ninguna fila nueva |
| PR abierto o cerrado sin merge (04) | `"action":"opened"`, o `"merged":false` | `200` y ninguna fila nueva |
| Push a `main` (04) | `{"ref":"refs/heads/main","sender":{"login":"maria.perez"}}` | `200` y ninguna fila nueva |
| Entrega duplicada (05) | el mismo body dos veces con el mismo `X-GitHub-Delivery` | dos `200` y **una sola** fila |
| Asignación con label crítica (03) | el body de abajo | `200` y una fila con `type":"issue.assigned"` |
| Asignación sin label crítica (04) | el mismo body con `"labels":[{"name":"chore"}]` | `200` y ninguna fila nueva |
| Filtro por usuario (07) | `curl "http://localhost:3000/history?user=maria.perez"` | sólo los eventos donde es actor o assignee |
| Usuario sin eventos (07) | `curl "http://localhost:3000/history?user=nadie"` | `[]` con `200`, nunca un error |
| Filtro vacío (07) | `curl "http://localhost:3000/history?user="` | el registro completo |
| Persistencia (21) | pará el servidor y levantalo de nuevo | las filas siguen ahí |

El body de asignación, para el caso de arriba:

```json
{"action":"assigned","sender":{"login":"lucia.gomez"},"assignee":{"login":"tomas.ferreira"},"issue":{"title":"Webhook drops events under load","html_url":"https://github.com/AG2904/github-activity-feed/issues/7","updated_at":"2026-10-16T11:00:00Z","labels":[{"name":"bug"}],"assignees":[{"login":"tomas.ferreira"}]}}
```

## Paso 7 — Empezar de cero

`Ctrl-C` apaga el servidor. El historial queda en el archivo SQLite
(`relay.db` por defecto, o donde apunte `DATABASE_PATH`), así que si querés una base
limpia:

```powershell
Remove-Item relay.db        # PowerShell
rm relay.db                 # bash
```

Y si querés probar los pasos con otro puerto, alcanza con `$env:PORT = 3001` antes de
arrancar.

---

# Probar la CLI a mano

El otro lado del cable: el binario `feed` contra el relay que acabás de levantar. La
preparación sigue siendo la de arriba —el relay corriendo (paso 1) y un webhook
firmado a mano (paso 3)—; lo de acá es la versión manual de lo que la suite del
cliente prueba en 46 casos sobre el límite del proceso.

## Paso 1 — Construir el binario

```bash
npm run cli:build     # tu plataforma, en client/dist/
npm run cli:dist      # las cuatro: windows/amd64, linux/amd64, darwin/amd64, darwin/arm64
```

```text
feed: dist/feed-windows-amd64.exe  windows/amd64  1.0.0+88bb654
feed: dist/feed-linux-amd64  linux/amd64  1.0.0+88bb654
feed: dist/feed-darwin-amd64  darwin/amd64  1.0.0+88bb654
feed: dist/feed-darwin-arm64  darwin/arm64  1.0.0+88bb654
```

La versión sale del `package.json` más el commit del que se construyó, con `-dirty`
si el árbol tenía cambios sin commitear. El binario la contesta:

```bash
client/dist/feed-windows-amd64.exe --version     # 1.0.0+88bb654
```

Los cuatro salen con `CGO_ENABLED=0` y `-trimpath`: son estáticos, no dependen de
nada instalado en la máquina del que los recibe, y no llevan rutas de tu disco
adentro. El build pasa por `client/tools/build.mjs` porque hay que fijar variables de
entorno por target, y `CGO_ENABLED=0 GOOS=linux go build` no es un comando que
entienda ninguna shell de Windows.

Las otras dos entradas son la suite y el vet, cada una entrando ella misma en
`client/`:

```bash
npm run cli:test      # go test -count=1 ./...
npm run cli:lint      # go vet ./...
```

## Paso 2 — Apuntarlo al relay

```bash
client/dist/feed-windows-amd64.exe watch --relay http://localhost:3000
```

El relay se resuelve como `--relay`, después `RELAY_URL`, después
`http://localhost:3000`. Como el relay de esta guía ya corre en 3000, alcanza con
`feed watch` a secas. La primera línea que ves es el banner, y va a **stderr**:

```text
watching http://localhost:3000/stream
```

Que stdout y stderr estén separados es lo que hace que `feed watch | tee` y
`feed watch | jq` sean seguros: por stdout sólo salen eventos.

## Paso 3 — Ver un evento real llegar

En una terminal, el `watch` del paso 2. En otra, el webhook firmado del paso 3 de
arriba. En el momento en que el relay guarda el evento, la terminal del `watch`
muestra las dos líneas:

```text
[pr.merged] Prueba manual
AG2904/github-activity-feed · maria.perez · just now · https://github.com/AG2904/github-activity-feed/pull/998
```

Dos líneas por evento: el badge del tipo (con el assignee adelante cuando lo hay, que
es el caso de `issue.assigned`) y el asunto; abajo, repositorio, actor y tiempo
relativo. En una terminal de verdad el badge va con color y el asunto es un
hipervínculo clickeable al PR o al issue.

**Si el relay es nuevo, la primera conexión no muestra nada**: no hay historial que
reponer. El `watch` no hace polling: se queda escuchando hasta que pasa algo.

## Paso 4 — El arranque en frío y el backfill

Con dos o tres eventos ya guardados (mandá el webhook dos veces con distinto
`X-GitHub-Delivery`), cortá el `watch` con `Ctrl-C` y volvé a arrancarlo. Ahora la
primera conexión repone los últimos diez eventos guardados, del más viejo al más
nuevo, y recién después sigue el stream. Es la diferencia entre una terminal que
arranca vacía y una que te dice que la CLI funciona.

```bash
feed watch --backfill 3     # sólo los tres más recientes
feed watch --backfill 0     # ninguno: la terminal arranca en blanco a propósito
```

El backfill corre **una sola vez**, en la primera conexión: si el relay se cae y
vuelve, no reimprime nada. Lo que pasó mientras estabas desconectado no se recupera
del stream — eso es `feed history`, y es el paso 6.

## Paso 5 — Filtrar y pipear

El relay le manda todo a todos y la CLI filtra para sí misma:

```bash
feed watch --user maria.perez                       # sus eventos: actor o assignee
feed watch --user juan.gomez --repo AG2904/github-activity-feed
```

El caso de la tubería, que es la razón por la que stdout está limpio:

```bash
feed watch | tee feed.log                            # queda el registro, sin color
feed watch --json | jq -c '.subject'                 # un JSON por línea, sin re-serializar
feed watch --json --user lucia.gomez | jq '.[0].assignee'
```

Con la salida por un pipe no hay color y el asunto lleva la URL cruda en vez del
hipervínculo; `--no-color` o `NO_COLOR` hacen lo mismo en una terminal. `--json` no
reformatea: entrega el payload del relay tal como llegó, así que el `&`, el
`assignee: null` y los milisegundos del `when` sobreviven.

## Paso 6 — La pregunta retrospectiva

```bash
feed history --no-color                              # todo el registro, más nuevo primero
feed history --user maria.perez --no-color           # lo de maria, actor o assignee
feed history --group-by-repo --no-color              # un bloque por repositorio
feed history --json --user lucia.gomez | jq '.[].subject'
```

```text
AG2904/github-activity-feed
[pr.merged] Prueba manual
maria.perez · just now · https://github.com/AG2904/github-activity-feed/pull/998
```

Agrupado: un header por repositorio en orden alfabético, los eventos de cada uno en
el orden del relay, sin repetir el repositorio en las líneas (el header ya lo
nombró). Es lo que el stream no puede hacer: una lista infinita y desordenada no se
agrupa.

## Paso 7 — Los casos que demuestran cada comportamiento

| Qué probás | Cómo | Qué esperar |
| --- | --- | --- |
| Binario suelto (08) | copiar `client/dist/feed-*` a otra máquina sin Go | corre igual: es estático |
| Versión (08) | `feed --version` | el `package.json` más el commit del build |
| Config del relay (01) | `RELAY_URL=http://localhost:3001 feed watch` | el banner apunta a 3001 |
| Evento en vivo (02) | el webhook del paso 3 con el `watch` abierto | las dos líneas, con `just now` |
| Frame roto (02) | `curl -N` y pegarle cualquier cosa al stream | el `watch` no se cae, descarta el frame |
| Backfill (06) | `Ctrl-C` y volver a arrancar | repone los últimos diez, más viejo primero |
| Backfill apagado (06) | `feed watch --backfill 0` | terminal en blanco hasta el evento vivo |
| Sin reimpresión (06) | cortar el relay y levantarlo de nuevo | aviso de reconexión y nada repetido |
| Filtro por actor (05) | `--user maria.perez` con un merge suyo | pasa |
| Filtro por assignee (05) | `--user tomas.ferreira` con la asignación | pasa |
| Filtro que no matchea (05) | `--user nadie` | nada, y el stream sigue vivo |
| Filtro por repositorio (05) | `--repo otro/repo` | nada de este repo |
| Pipe limpio (10) | `feed watch \| tee feed.log` | sin color ni escapes en el archivo |
| JSON crudo (17) | `feed watch --json \| jq -c .type` | `"pr.merged"`, payload sin reformatear |
| Historial (07) | `feed history --user maria.perez` | lo de maria, más nuevo primero |
| Historial agrupado (16) | `feed history --group-by-repo` | un bloque por repo, alfabético |
| Historial vacío (07) | `feed history` contra una base recién creada | nada, salida 0 |
| Relay caído (07) | `feed history` con el relay apagado | `history: ...` en stderr y salida 1 |
| Ctrl-C (21) | `Ctrl-C` con el `watch` abierto | sale limpio, la shell sigue usable |

## Paso 8 — Empezar de cero

Los binarios construidos quedan en `client/dist/`, que está ignorado por git, así que
borrarlos no ensucia nada:

```powershell
Remove-Item -Recurse client/dist     # PowerShell
rm -rf client/dist                   # bash
```

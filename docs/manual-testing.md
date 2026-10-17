# Probar el relay a mano

Guía para levantar el relay y verificar con las manos lo que los tickets 01 a 07
construyeron. La suite e2e prueba lo mismo de forma automatizada (48 casos sobre la
misma seam HTTP); esto es para cuando querés ver el cable, probar un payload propio o
mostrarle el sistema a alguien.

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

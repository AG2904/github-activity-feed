# GitHub Activity Feed

Un sistema que convierte las entregas críticas de GitHub en una fuente de actividad
para un equipo de desarrollo, y un CLI que la muestra en la terminal.

El proyecto tiene dos mitades que hablan el mismo contrato:

- **El relay** (`src/`, NestJS + SQLite) recibe los webhooks firmados de GitHub,
  decide qué eventos son críticos, los guarda y los emite en vivo por SSE.
- **El CLI** (`client/`, Go) se conecta al relay y pinta los eventos en la terminal,
  o consulta el historial almacenado.

Un evento es crítico en dos casos: un pull request fusionado a `main`, y un issue
asignado que lleva la etiqueta `bug` o `feedback`. Todo lo demás se acusa y se
descarta. El relay es la memoria compartida del equipo; el CLI es la vista de cada
desarrollador.

La justificación del diseño, el contrato y el estado del proyecto están en
[`ARCHITECTURE.md`](ARCHITECTURE.md).

## Requisitos

| Herramienta | Versión | Para qué |
| --- | --- | --- |
| Node.js | 24 (la versión que apuntan los tipos de las dependencias) | el relay |
| Go | 1.27 (`client/go.mod` declara `go 1.27.0`) | el CLI |

## Relay: arranque rápido

1. Copiar la plantilla de variables de entorno y completar el secreto:

   ```powershell
   Copy-Item .env.example .env
   ```

   En `.env`, reemplazar `change-me` por un secreto largo. `GITHUB_WEBHOOK_SECRET` es
   obligatoria: sin ella el relay se niega a arrancar. `.env` está en `.gitignore` y
   nunca se commitea.

2. Instalar las dependencias:

   ```powershell
   npm ci
   ```

3. Arrancar el relay:

   ```powershell
   npm start
   ```

   El relay carga `.env` al arrancar. Las variables exportadas en la shell tienen
   prioridad sobre el archivo, así que podés sobrescribir cualquiera sin editarlo.
   El proceso escucha en el puerto `3000`, o en el que indique la variable `PORT`.

4. Confirmar que responde:

   ```powershell
   Invoke-WebRequest http://localhost:3000/history -UseBasicParsing
   ```

   Con la base vacía, el cuerpo es:

   ```json
   []
   ```

   El historial vive en `relay.db` (o en la ruta de `DATABASE_PATH`). Las rutas
   `POST /` (webhooks), `GET /history` y `GET /stream` (SSE) son las únicas del relay.

## CLI: arranque rápido

1. Compilar el binario. El bloque siguiente se ejecuta desde la raíz del repositorio y
   la opción integrada en npm deja el binario de la plataforma anfitriona en
   `client/dist/`:

   ```powershell
   npm run cli:build
   ```

   La salida nombra el archivo producido, por ejemplo
   `feed: dist/feed-windows-amd64.exe  windows/amd64  1.0.0+<commit>`.

   Como alternativa se puede compilar directamente con Go. Este segundo camino se
   queda en `client\` para compilar y para ejecutar. En Windows el nombre del
   ejecutable debe incluir la extensión:

   ```powershell
   cd client
   go build -o feed.exe ./cmd/feed
   ```

2. Con el relay en marcha, escuchar los eventos en vivo. El comando depende del camino
   de compilación y cada bloque indica desde dónde se ejecuta.

   Opción npm, desde la raíz del repositorio:

   ```powershell
   .\client\dist\feed-windows-amd64.exe watch --relay http://localhost:3000
   ```

   Opción Go, desde `client\`, el directorio en el que quedó la shell tras el `cd client`
   del paso 1:

   ```powershell
   .\feed.exe watch --relay http://localhost:3000
   ```

3. Qué se ve. Los eventos van a **stdout**; la narración de la sesión va a **stderr**:

   - stdout: una tarjeta por evento, por ejemplo
     `maria.perez merged fix/memory-leak into main`, con el título, las etiquetas, el
     repositorio, el tiempo relativo y la URL.
   - stderr: el banner `watching <url>/stream · backfill <n>`, la línea `connected · …`,
     los avisos de reconexión y la línea `stopped · …` al salir con `Ctrl-C`.
     `<url>` es la base del relay: el CLI le agrega `/stream` para abrir el stream. Con
     los filtros locales, el banner suma ` · user <login>` y ` · repo <owner/name>`
     (véase [`docs/cli-testing.md`](docs/cli-testing.md)).

   Esa separación es deliberada: `feed watch | jq` recibe sólo eventos.

4. Para ver el contrato sin formato, una línea JSON por evento, tal como lo emite el
   relay. Con el binario de npm, desde la raíz del repositorio:

   ```powershell
   .\client\dist\feed-windows-amd64.exe watch --relay http://localhost:3000 --json
   ```

   Con el binario de Go, el mismo comando se escribe `.\feed.exe …` dentro de `client\`.

5. Para consultar lo ya almacenado:

   ```powershell
   .\client\dist\feed-windows-amd64.exe history --relay http://localhost:3000 --json
   ```

   Desde `client\` con el binario de Go, el equivalente es
   `.\feed.exe history --relay http://localhost:3000 --json`.

   Contra un relay recién arrancado, la salida es `[]`. Sin `--json`, el historial se
   pinta con las mismas tarjetas del modo en vivo.

El flujo completo —firma del webhook incluida— está en
[`docs/end-to-end-testing.md`](docs/end-to-end-testing.md).

## Contrato y runbooks

El contrato es único y no se duplica en este documento. La forma canónica de nueve
campos, con el significado de cada uno, está en
[`ARCHITECTURE.md` §6](ARCHITECTURE.md#6-contrato-de-datos-común-activityevent).

Los recorridos operativos:

- [`docs/manual-testing.md`](docs/manual-testing.md) — el relay solo, paso a paso,
  incluido el envío de un webhook firmado.
- [`docs/cli-testing.md`](docs/cli-testing.md) — el CLI solo: comandos, códigos de
  salida, color, filtros locales y reconexión.
- [`docs/end-to-end-testing.md`](docs/end-to-end-testing.md) — relay y CLI juntos,
  desde el webhook hasta la terminal.
- [`docs/exposing-the-relay.md`](docs/exposing-the-relay.md) — cómo exponer el relay
  con un túnel para recibir webhooks reales de GitHub, con sus límites medidos.

## Estado de las fases

El plan por fases está en [`ARCHITECTURE.md` §7](ARCHITECTURE.md#7-plan-de-implementación-por-fases).

- **Fase 1 — el relay:** completa. Ingesta firmada, filtro de eventos críticos,
  idempotencia por entrega, stream SSE e historial.
- **Fase 2 — el CLI:** completa. `watch` y `history`, filtros locales, renderizado en
  tarjetas y binarios estáticos para cuatro plataformas.
- **Fase 3 — validación y demo:** en progreso. Incluye CI en GitHub Actions, la
  publicación de binarios y una demostración con payloads reales de GitHub.

Fuera de alcance en esta fase: automatización de despliegue, contenedores, proxies
inversos y TLS; empaquetado con goreleaser, instaladores o imágenes Docker; cambios al
contrato de eventos, al filtro o al formato de cable; autenticación multiusuario,
permisos, monitoreo, métricas y alertas. El despliegue del relay en un servidor interno
es opcional y no bloquea el cierre de la fase.

# Ticket 02 - The README is the front door

## Resumen

El repositorio gana su primera puerta de entrada: un `README.md` en español neutro
(por pedido explícito; el resto de la documentación usa voseo) que explica qué es el
sistema, cómo levantar el relay, cómo compilar y correr el CLI, dónde vive el
contrato y dónde están los runbooks. Cada comando se ejecutó tal como quedó escrito;
las dos diferencias entre el ticket y la realidad que aparecieron al probarlo están
documentadas en el propio README en vez de disimuladas.

## Cambios Realizados

### `README.md`

- **Qué es y para quién**: el relay que convierte entregas críticas de GitHub en un
  feed y el CLI que lo muestra en la terminal; reglas de qué es crítico y puntero a
  `ARCHITECTURE.md`.
- **Requisitos**: Node 24 y Go 1.27, con la nota de `npm.cmd` en PowerShell.
- **Relay, arranque rápido**: copiar la plantilla de `.env`, definir
  `GITHUB_WEBHOOK_SECRET`, `npm.cmd ci`, `npm.cmd start -- --env-file .env`,
  comprobación con `GET /history` → `[]`, y las variables `PORT` y `DATABASE_PATH`.
- **CLI, arranque rápido**: los dos caminos (script `npm.cmd run cli:build` desde la
  raíz, o `go build` dentro de `client\`), cada bloque autocontenido con el directorio
  desde el que se ejecuta, y qué sale por stdout (eventos) y por stderr (banner,
  `connected`, reconexiones, `stopped`), más `--json` e `history`.
- **Contrato y runbooks**: enlaces a `ARCHITECTURE.md` §6 y a `docs/manual-testing.md`,
  `docs/cli-testing.md`, `docs/end-to-end-testing.md` y `docs/exposing-the-relay.md`.
- **Estado de las fases**: 1 y 2 completas, 3 en curso, y qué queda fuera de alcance.

### `ARCHITECTURE.md`

- La línea de estado pasa de "Fase 3 queda pendiente" a "está en curso", y §9 marca
  el README como escrito (ticket 02) dejando el ítem de CI abierto hasta la primera
  corrida real.

## Comportamiento resultante

Comandos reales ejecutados en el worktree, siguiendo el README tal como quedó:

```text
node --version        v26.8.2
npm.cmd --version     12.2.0
go version            go1.27.1 windows/386

npm.cmd ci            exit 0, 327 paquetes
npm.cmd start -- --env-file .env
                      Nest application successfully started
GET /history          []
npm.cmd run cli:build dist/feed-windows-amd64.exe  windows/amd64  1.0.0+<commit>
.\client\dist\feed-windows-amd64.exe --version  1.0.0+<commit>
.\feed.exe history --relay http://localhost:3000 --json   []
```

Y el banner observado, con y sin filtros:

```text
watching http://localhost:3000/stream · backfill 10
connected · waiting for events
watching http://localhost:3000/stream · backfill 10 · user maria.perez · repo AG2904/github-activity-feed
```

## Verificación

- Cada bloque del README se ejecutó desde el directorio que el propio bloque declara:
  el camino npm desde la raíz (`.\client\dist\feed-windows-amd64.exe`), el camino Go
  dentro de `client\` (`.\feed.exe`). El fallo anterior —`cd client` seguido de
  `.\feed.exe` desde la raíz— se reprodujo en rojo antes de corregirlo.
- Relay levantado con `--env-file .env`; `/history` respondió `[]`; el proceso se
  detuvo verificando el PID y el path del worktree antes de matarlo; `PORT` quedó
  libre.
- Enlaces y anclas: `ARCHITECTURE.md` §6 y §7 resuelven con el slug real; los cuatro
  runbooks y `.env.example` existen.
- Español neutro: barrido de voseo sobre las formas de segunda persona (`-és`, `-á`,
  `-í`) sin coincidencias; las construcciones son impersonales o con infinitivo.
- Revisión de dos ejes: se corrigieron el orden de los bloques del CLI (ya no se
  invoca `.\feed.exe` fuera de `client\`), el banner (faltaba `/stream`), la
  consistencia `npm`/`npm.cmd` y el estado de `ARCHITECTURE.md`.

**No verificado**: el camino con `$PORT` distinto de 3000 (el puerto estaba libre y no
se ejercitó), el render de un evento con webhook firmado real (solo se observaron
banner y `connected`), y las rutas de build/ejecución que no son Windows — están
documentadas en los runbooks, fuera del alcance de este ticket.

## Acceptance Criteria

- ✅ AC1: `README.md` en la raíz, con qué es y para quién.
- ✅ AC2: arranque del relay desde cero: plantilla de `.env`, secreto, `npm ci`,
  arranque en el puerto 3000 (o el que indique `PORT`), con la salvedad verificada de
  `--env-file .env` (el relay no lee el archivo por sí solo).
- ✅ AC3: arranque del CLI: build por script o por Go, `watch`, y qué esperar en
  stdout y stderr.
- ✅ AC4: el contrato se enlaza, no se duplica (§6) y los runbooks están enlazados.
- ✅ AC5: estado de las fases honesto (1 y 2 completas, 3 en curso) y fuera de alcance
  explícito.
- ✅ AC6: cada comando se ejecutó como está escrito; las salidas mostradas son reales
  y los enlaces resuelven. Las dos diferencias con el ticket (`npm start -- --env-file
  .env` y `go build -o feed.exe` en Windows) quedaron documentadas con su motivo.

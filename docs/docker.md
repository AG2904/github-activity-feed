# Todo en Docker

Dos imágenes, un contrato: el relay NestJS y el CLI en Go, cada uno construido
desde su propio toolchain y ejecutado sin él. El `Dockerfile` de la raíz tiene dos
targets y `docker-compose.yml` los orquesta.

Para qué sirve: probar el sistema sin instalar Node, Go ni SQLite, y tener el relay
corriendo siempre con el historial sobre un volumen en vez de un `relay.db` en el
directorio de trabajo.

La decisión y lo que se descartó están en
[ADR-0014](adr/0014-containers-as-the-distribution-path.md).

## Camino corto

```bash
cp .env.example .env      # y reemplazar `change-me` por un secreto real
docker compose up -d --build relay
docker compose run --rm feed watch
```

`docker compose run --rm feed history --json` hace la consulta puntual, y
`docker compose down` apaga todo sin tocar el volumen.

El relay queda publicado en `127.0.0.1:3000`, que es lo que el CLI de la máquina
espera por defecto (`http://localhost:3000`).

## Variables

| Variable | Dónde | Default | Para qué |
| --- | --- | --- | --- |
| `GITHUB_WEBHOOK_SECRET` | `.env` o entorno | **obligatoria** | HMAC del webhook; el relay no arranca sin ella |
| `RELAY_PORT` | `.env` | `3000` | puerto publicado en el host, siempre en `127.0.0.1` |
| `RELAY_URL` | entorno del `feed` | `http://relay:3000` | a qué relay apunta el CLI dentro de la red de compose |
| `STREAM_HEARTBEAT_MS` | `.env` | `15000` | cada cuánto escribe el comentario SSE que mantiene viva la conexión |
| `FEED_VERSION` | entorno | `docker` | lo que responde `feed --version`; el release pasa el tag |
| `DATABASE_PATH` | compose | `/data/relay.db` | el SQLite, sobre el volumen |

`docker compose` lee `.env` solo para interpolar estas variables. El relay **no**
lee `.env` dentro del contenedor: su configuración entra por `environment`, así que
`.env` nunca se copia a la imagen (`.dockerignore`).

## El historial sobrevive

El SQLite vive en el volumen `relay-data`, montado en `/data`. Recrear o actualizar
el contenedor no lo toca; `docker compose down -v` sí, y es la única forma de
borrarlo.

```bash
docker compose cp relay:/data/relay.db ./relay-backup.db   # copia en caliente
```

## El CLI como herramienta, no como servicio

`feed` está detrás del perfil `tools`, así que `docker compose up` arranca solo el
relay. La razón es que un `watch` es un proceso en primer plano pegado a la
terminal: como servicio en segundo plano no tiene dónde mostrar su salida.

```bash
docker compose run --rm feed watch --repo owner/name
docker compose run --rm feed history --group-by-repo
```

`run` construye la imagen si falta, y el CLI resuelve `relay` por DNS interno, así
que no hace falta publicar el puerto para que el contenedor lo alcance.

## Webhook y túnel

GitHub tiene que llegar al relay, y para eso sigue haciendo falta un túnel
([`exposing-the-relay.md`](exposing-the-relay.md)): el contenedor publica el puerto
en `127.0.0.1`, no en internet. `cloudflared tunnel --url http://localhost:3000`
apunta al puerto publicado y funciona igual que sin Docker.

El límite conocido de los Quick Tunnels —`/stream` entrega cero bytes por la URL
pública— no afecta al CLI en Docker: el contenedor habla con el relay por la red de
compose, no por el túnel.

## Versiones: por qué estas y hasta cuándo

Fijadas el **26 de octubre de 2026**, contra las fuentes oficiales:

| Imagen | Versión | Por qué esa |
| --- | --- | --- |
| `node` | `24.22.1-bookworm-slim` | LTS *Krypton*; `24.22.1` (20 oct 2026) es un release **de seguridad** según el índice de `nodejs.org/dist`. La línea 26 (26.12.1, 21 oct) también está parcheada pero todavía no es LTS. |
| `golang` | `1.27.3-alpine` | última estable en `go.dev/dl`; 1.26.10 es la de la línea anterior. Un release de Go es, en la práctica, un parche de la stdlib que el binario lleva adentro. |
| `alpine` | `3.24.3` | runtime del CLI: solo `ca-certificates` y un binario estático |

El árbol de dependencias de producción del relay se consultó contra OSV el mismo
día: **sin advisories** para `@nestjs/common`, `@nestjs/core`,
`@nestjs/platform-express`, `@nestjs/typeorm`, `better-sqlite3`, `reflect-metadata`,
`rxjs` ni `typeorm` en las versiones del lockfile. `npm audit` no se pudo usar aquí
(el registry corta la conexión), y por eso la consulta fue directa a OSV.

### Cómo se actualiza

Los números están en los `ARG` del `Dockerfile`, y nada más:

```dockerfile
ARG NODE_VERSION=24.22.1
ARG GO_VERSION=1.27.3
ARG ALPINE_VERSION=3.24.3
```

Al subir cualquiera, `docker compose build` reconstruye. No hay tags flotantes
(`node:24`, `golang:latest`) a propósito: un `latest` convierte cada build en un
cambio de runtime silencioso, y con un `node_modules` compilado contra otro ABI el
síntoma aparece en runtime, no en el build.

## Qué está verificado

El job `docker` de [`ci.yml`](../.github/workflows/ci.yml) construye las dos imágenes
en cada push y en cada PR, y hace un smoke test de cada una: `feed --version` y un
`GET /history` contra el relay recién iniciado. La primera corrida pasó el **26 de
octubre de 2026**, así que ambos targets construyen y ambos contenedores arrancan.

En la máquina de origen —donde no hay Docker— se verificó además que los tags de las
imágenes existen en Docker Hub, que las versiones son las publicadas en los índices
oficiales, el resultado de OSV y que los dos YAML parsean.

Lo que **no** se probó todavía es el recorrido completo a mano: un webhook firmado
entrando por el túnel hasta un `feed watch` corriendo en `docker compose`. Los pasos
para hacerlo están en [`end-to-end-testing.md`](end-to-end-testing.md).

## Seguridad

- Las dos imágenes corren como usuario sin privilegios (`node`, y `feed` con uid
  10001), sin capabilities en el CLI y con `no-new-privileges` en ambos.
- El puerto se publica en `127.0.0.1`, no en `0.0.0.0`: el webhook entra por el
  túnel, y `/history` y `/stream` son **públicos por diseño** (ADR-0001). Publicar
  el relay en una interfaz abierta expone todo el historial a quien tenga la IP.
- El secret no está en la imagen ni en el `docker-compose.yml`: entra por el
  entorno.

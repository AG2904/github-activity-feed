# Ticket 01 - El binario arranca y alcanza el relay

## Resumen

El walking skeleton de la Fase 2. `client/` pasa a ser un módulo Go con un
binario `feed` que resuelve dónde está el relay, conoce su propia versión y
despacha sus dos subcomandos, `watch` y `history`. Ninguno de los dos pinta nada
todavía: el parser SSE y el renderer llegan con el ticket 02, y la forma del
historial con el ticket 07. Lo que este ticket entrega, además del binario, es
el harness de tests sobre el que se apoyan todos los tickets siguientes.

Un commit en `main`: `0d09d6b feat: boot the feed binary and reach the relay (ticket 01)`.

## Cambios Realizados

### client/go.mod

- Módulo `github.com/AG2904/github-activity-feed/client` (ADR-0009), anidado
  dentro del monorepo.
- Sin directiva `require`: el cliente se apoya sólo en la biblioteca estándar
  (ADR-0008). Tampoco hay `go.sum`, porque no hay dependencias que fijar.

### client/cmd/feed/main.go

- Punto de entrada mínimo: `main` delega en `cli.Run` y convierte su resultado
  en el código de salida del proceso.
- `var version = "dev"`, inyectada en el build con
  `-ldflags "-X main.version=<versión>"`. Un binario distribuido puede decir de
  dónde viene sin que el código sepa cómo fue compilado.

### client/internal/cli/cli.go

- **Despacho explícito.** `run` parsea el nivel raíz con `flag` y decide entre
  `watch`, `history` y `--version`; un `feed` sin subcomando imprime la ayuda y
  sale con `2`, y un subcomando desconocido hace lo mismo nombrando el argumento.
  Ningún subcomando se asume.
- **Resolución del relay.** `--relay` gana sobre `RELAY_URL`, que gana sobre
  `http://localhost:3000`. El default del flag ya carga el entorno y el fallback,
  así que un flag presente gana sin una segunda búsqueda. Se recortan las barras
  finales antes de componer la ruta.
- **`watch`.** Imprime `watching <relay>/stream` en stderr y abre `GET /stream`.
  Por ahora descarta los frames: el parser es del ticket 02. Si el relay no
  responde, sale con `1`; el reintento indefinido es del ticket 04.
- **`history`.** Abre `GET /history` y descarta el cuerpo; la forma que se
  imprime es del ticket 07. Un relay inalcanzable sale con `1`.
- El contrato de salida se respeta desde el primer día: stdout no lleva nada
  todavía, y el banner y los errores van por stderr.

### client/test/

- **`main_test.go`**: `TestMain` compila el binario una sola vez en un directorio
  temporal, con `-trimpath` y la versión inyectada por `-ldflags`, desde la raíz
  del módulo. Toda la suite corre ese binario.
- **`harness_test.go`**:
  - `fakeRelay`, un `httptest.Server` real que responde `/stream` con un frame
    SSE del shape de ocho campos y `/history` con `[]`, y registra cada path que
    recibió. Es la contraparte del proceso en la única seam, y el lugar donde los
    tickets 02–07 van a extender el habla del relay falso.
  - `runFeed` inicia el binario, espera, y devuelve stdout, stderr y el código de
    salida.
  - `environment` elimina `RELAY_URL` del entorno real antes de aplicar overrides,
    para que la configuración de la máquina del desarrollador no se filtre.
  - `firstWatchBanner` lee el banner del caso default sin depender de qué esté
    escuchando en el puerto 3000.
- **`e2e_test.go`**, 7 casos, todos por el límite del proceso: versión inyectada,
  `feed` sin subcomando, subcomando desconocido, `--relay` sobre `RELAY_URL`,
  `RELAY_URL` sin flag, default `localhost:3000`, y `history` alcanzando
  `/history`. Ningún test importa `internal/cli`.

## Comportamiento resultante

- `feed --version` imprime exactamente la versión inyectada y sale con `0`.
- `feed` sin argumentos imprime la ayuda y sale con `2`; `feed bogus` nombra el
  subcomando desconocido e imprime la ayuda, también con `2`.
- `feed watch --relay <url>` imprime `watching <url>/stream` en stderr y abre la
  conexión; `RELAY_URL` produce el mismo efecto cuando el flag falta, y
  `http://localhost:3000` es el fallback.
- `feed history --relay <url>` alcanza `GET /history` y sale con `0`.
- `go vet ./...` y `gofmt -l .` no reportan nada desde `client/`.

## Verificación

```text
cd client && go test -count=1 ./...
?   	github.com/AG2904/github-activity-feed/client/cmd/feed	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/cli	[no test files]
ok  	github.com/AG2904/github-activity-feed/client/test	7.155s

cd client && go vet ./...
(exit 0, sin salida)

cd client && gofmt -l .
(sin salida)

npm test
Test Files  1 passed (1)
Tests       54 passed (54)
```

Smoke manual del binario compilado con `-ldflags "-X main.version=v0.1.0-demo"`:

```text
feed --version  -> v0.1.0-demo (exit 0)
feed            -> ayuda en stderr (exit 2)
```

## Acceptance Criteria

- ✅ AC1: `client/` contiene un módulo Go en `github.com/AG2904/github-activity-feed/client`, con `cmd/feed` como entry point y `go.mod` sin directiva `require`.
- ✅ AC2: El relay se resuelve como flag sobre entorno sobre default: `--relay <url>`, luego `RELAY_URL`, luego `http://localhost:3000`.
- ✅ AC3: `feed --version` imprime la versión inyectada en el build con `-ldflags`, y nada más.
- ✅ AC4: `feed` sin subcomando imprime la ayuda y sale con `2`. `watch` e `history` son los dos subcomandos y ninguno se asume.
- ✅ AC5: `go vet ./...` está limpio desde `client/`.
- ✅ AC6: El harness compila el binario, lo corre contra un relay falso y verifica stdout, stderr y el código de salida; ningún test importa un paquete interno.

## Decisiones y trabajo pendiente

- **`watch` sale con `1` si el relay no responde.** Es provisional: el ticket 04
  lo reemplaza por el reintento indefinido. Se prefirió no adelantar la política
  de reconexión en el skeleton.
- **Ambos subcomandos descartan la respuesta.** El ticket 01 sólo prueba la seam
  de configuración y de red; renderizar sin el parser del ticket 02 o sin la
  forma del ticket 07 habría adelantado decisiones ajenas.
- **El default se verifica leyendo el banner, no ocupando el puerto 3000.** Así
  el test no colisiona con un relay que el desarrollador ya tenga corriendo, y el
  banner queda como señal observable de la resolución completa.

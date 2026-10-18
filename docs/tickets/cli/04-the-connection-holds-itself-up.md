# Ticket 04 - La conexión se sostiene sola

## Resumen

La CLI puede quedarse corriendo en una terminal durante días. `feed watch`
reabre la conexión cuando el relay la corta, cuando deja de escribir, o cuando
el relay ni siquiera está arriba al arrancar; espera con backoff exponencial y
jitter, resetea el backoff cuando llega un evento, y sale limpio con Ctrl-C.
stdout sigue llevando sólo eventos; stderr lleva el banner, un aviso por intento
de reconexión y el aviso de recuperación.

Dos commits en `main`: `68e10f9 feat: keep the feed alive through drops and
silence (ticket 04)` y `454edf1 fix: keep reconnect honest and local failures
fatal (ticket 04)`, el segundo con las correcciones de la revisión de dos ejes.

## Cambios Realizados

### client/internal/cli/watch.go (nuevo)

- **El loop.** `watch` abre una conexión, la sigue hasta que termina o se calla,
  y decide si reintentar. Cada intento fallido escribe exactamente una línea:
  `watch: reconnecting in <espera>: <motivo>`. Cuando una conexión vuelve
  después de una caída escribe `watch: reconnected`; el banner inicial no se
  repite.
- **`stream`.** Una conexión con `http.NewRequestWithContext`, status 200, y el
  parser SSE. Llama a `connected` cuando la respuesta fue aceptada, para que el
  loop pueda anunciar la recuperación en el momento en que ocurre.
- **`backoff`.** Escalones de 1, 2, 4, 8, 16 y 30 segundos. Cada espera suma un
  jitter de hasta un cuarto del escalón y se recorta al techo de 30s; un evento
  válido llama a `reset()` antes de calcular la espera siguiente. El ticket 05
  extendió ese reset a un evento que el filtro descarta: llegó, aunque no se
  pinte.
- **`idleReader`.** Resetea un `time.Timer` en cada `Read` que devuelve bytes,
  no en cada evento. A los cinco minutos sin bytes cierra el cuerpo, la lectura
  se desbloquea y el loop reconecta con el motivo `no bytes for 5m0s`. Los
  heartbeats del relay son comentarios SSE y también cuentan.
- **`emitter`.** La política de salida (renderizado, raw o descarte) quedó en un
  solo valor que recibe `stream`, en lugar de tres parámetros sueltos.
- **Fallos locales vs reintentables.** Una URL inválida o un stdout cerrado se
  envuelven en `fatalError` y salen con 1; una conexión caída, un non-200 o un
  stream que termina se reintentan. Antes, `feed watch --relay ://bad` entraba en
  un loop infinito.
- **Test seam.** `idleTimeoutSetting` está vacío en un build normal; el harness
  inyecta `300ms` por `-ldflags` en un segundo binario, para probar la regla de
  silencio sin esperar cinco minutos. El binario distribuido conserva el default.

### client/cmd/feed/main.go

- `signal.NotifyContext` con `os.Interrupt`. El primer Ctrl-C cancela el request
  o la espera; `watch` devuelve 0. Apenas se cancela el contexto se restaura el
  handler por defecto, así que un segundo Ctrl-C puede terminar un proceso
  trabado.
- `Run` recibe el `context.Context`; el interior sigue sin saber de señales.

### client/internal/cli/cli.go

- `Run` y `history` aceptan el contexto. `history` usa
  `http.NewRequestWithContext` y devuelve 0 si el Ctrl-C interrumpe el fetch o
  la copia; de otro modo una señal quedaría ignorada durante un one-shot.

### client/test/

- **fake relay:** handshake de conexión con canal sin buffer, para que cada
  señal pertenezca a la conexión actual; `dropStream` corta el stream activo
  desde el servidor; y cada request a `/stream` registra su `Last-Event-ID`.
- **`idleFeedBinary`:** segundo binario compilado con el timeout de 300ms.
- **Casos nuevos:** relay inalcanzable reintentado (dos intentos, exactamente
  tres líneas de stderr contando el banner), URL inválida fatal, backoff que
  crece y se resetea con un evento, reconexión sin replay (y sin
  `Last-Event-ID`), stream silencioso reabierto, y heartbeats que mantienen viva
  una conexión sin eventos.

## Comportamiento resultante

```text
$ feed watch
watching http://localhost:3000/stream
[pr.merged] Fix memory leak in parser component
AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/pull/42

# el relay se cae
watch: reconnecting in 1.08s: connection closed
# vuelve
watch: reconnected

# el stream queda mudo más de cinco minutos
watch: reconnecting in 2.21s: no bytes for 5m0s

# Ctrl-C
(sale 0, sin stack trace)
```

## Verificación

```text
cd client && go test -count=1 ./...
?   	github.com/AG2904/github-activity-feed/client/cmd/feed	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/activity	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/cli	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/render	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/sse	[no test files]
ok  	github.com/AG2904/github-activity-feed/client/test	20.973s

cd client && go vet ./...
(exit 0, sin salida)

cd client && gofmt -l .
(sin salida)

cd client && GOOS=linux go build ./...   → exit 0
cd client && GOOS=darwin go build ./...  → exit 0

npm test
Test Files  1 passed (1)
Tests       54 passed (54)
```

La suite del cliente tiene 19 casos, todos por el límite del proceso.

## Acceptance Criteria

- ✅ AC1: Una conexión caída se reabre sin intervención, con backoff exponencial de un segundo a un techo de treinta, con jitter.
- ✅ AC2: El backoff se resetea apenas llega un evento.
- ✅ AC3: Un stream que pasa más de cinco minutos en silencio se considera muerto y se reabre.
- ✅ AC4: Cada intento de reconexión escribe exactamente una línea en stderr; stdout lleva sólo eventos.
- ✅ AC5: Un relay inalcanzable al arrancar se reintenta indefinidamente, no es fatal.
- ✅ AC6: Una reconexión no reproduce nada: ningún evento ya impreso se imprime de nuevo.
- ✅ AC7: Ctrl-C sale con 0 después de hacer flush, sin stack trace.

## Decisiones y límites conocidos

- **Un aviso de recuperación además del de intento.** El ticket pide una línea
  por intento; la historia 9 pide saber cuándo la conexión volvió. Son líneas
  distintas y sólo la de intento cuenta para el criterio.
- **El backoff arranca en un segundo, no en medio.** El jitter suma hasta un
  cuarto del escalón en lugar de restar; el primer reintento espera 1–1.25s. En
  el techo, el clamp deja la espera en 30s exactos.
- **El default de producción no se espera en un test.** El caso de silencio usa
  un binario compilado con 300ms; el código y el camino son los mismos. Esperar
  los cinco minutos reales haría la suite inusable.
- **Ctrl-C no está automatizado.** En Windows no hay una forma portable de
  mandar un interrupt a un proceso hijo desde el test; el camino se verificó por
  código y la suite cubre la cancelación por contexto en el resto del loop.
- **El techo de 30s no se espera.** Se verifican los dos primeros escalones y el
  reset; llegar al techo requeriría minutos.
- **`history` también honra el contexto.** No es el alcance del ticket, pero si
  `main` captura la señal, un one-shot que la ignore dejaría el proceso sordo a
  Ctrl-C. Un `--json` interrumpido puede dejar un documento truncado en stdout;
  es el costo de salir limpio.
- **Deuda ajena:** el árbol de `ARCHITECTURE.md` y los scripts npm siguen en el
  ticket 08.

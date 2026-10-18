# Ticket 02 - Los eventos en vivo se pintan en la terminal

## Resumen

El cliente SSE y el renderer de dos líneas. `feed watch` mantiene abierta la
conexión a `GET /stream`, parsea los frames a mano (ADR-0007) y convierte cada
ActivityEvent en la forma que el desarrollador ve: badge y subject en la primera
línea, repositorio, actor y tiempo relativo en la segunda. Un frame que no
parsea se descarta sin terminar el proceso.

El trabajo aterrizó en dos commits: `ba2be89 feat: paint live events in the
terminal (ticket 02)` y `98f9fe6 fix: align the feed with the event contract and
the console (ticket 02)`, el segundo con las correcciones de la revisión de dos
ejes.

## Cambios Realizados

### client/internal/sse/sse.go (nuevo)

- Parser SSE sobre `bufio`, sin dependencias (ADR-0007). `Next()` devuelve sólo
  el payload de `data:`, uniendo data multilínea con `\n`.
- Se ignoran `id:`, `event:`, `retry:`, los frames de comentario (incluido el
  heartbeat del relay) y cualquier campo desconocido. Un frame sin `data` se
  consume y se salta.
- Un frame que termina en EOF sin su línea en blanco igual se entrega; el
  `io.EOF` limpio se distingue del error de lectura.

### client/internal/activity/activity.go (nuevo)

- `Event` con los ocho campos del contrato; `Repository` y `Assignee` son
  punteros porque son nullables. `When` ya es `time.Time`.
- `Parse` rechaza JSON inválido y payloads a los que les falta un campo que el
  contrato siempre trae (`type`, `actor`, `subject`, `url`, `when`). Un evento a
  medio formar no se renderiza.

### client/internal/render/render.go (nuevo)

- Dos líneas por evento. Primera: badge `[type]`, el assignee cuando existe, y
  el subject. Segunda: repositorio, actor y tiempo relativo.
- Con color: badge por tipo (`pr.merged` verde, `issue.assigned` amarillo), el
  assignee en bold, y el subject envuelto en un hyperlink OSC 8 al `url`.
- Sin color: no se emite ningún escape y el `url` crudo se agrega a la segunda
  línea. Un `repository: null` (fila anterior a la columna, ADR-0010) omite el
  segmento en lugar de inventar un nombre.
- El tiempo relativo se mantiene relativo a cualquier edad (`2h ago`, `3d ago`),
  nunca cambia a una fecha absoluta.

### client/internal/cli/cli.go

- `watch` acepta `--relay` y `--no-color`, imprime el banner en stderr, abre
  `GET /stream` y pinta cada payload parseable en stdout.
- `colorEnabled` es la única regla de escapes: `--no-color`, luego `NO_COLOR`,
  y sólo si stdout es una terminal. Un pipe nunca ve un escape.
- `terminal_windows.go` usa `GetConsoleMode` y habilita
  `ENABLE_VIRTUAL_TERMINAL_PROCESSING` mediante `SetConsoleMode` (cargado con
  `NewLazyDLL`, porque `syscall` no lo expone y ADR-0008 prohíbe dependencias).
  `terminal_other.go` usa `os.ModeCharDevice`. Sin esa distinción, un conhost de
  Windows imprimiría los escapes como texto.
- La creación del `FlagSet` y el parseo común se extrajeron a `newFlagSet` y
  `parseFlags`; `history` sigue haciendo su fetch y descarte hasta el ticket 07.

### client/test/

- `fakeRelay` ahora sostiene el stream abierto y expone `send`, para que un test
  escriba frames **después** de que el CLI se conectó (`waitConnected`), tal como
  pide la decisión de testing del spec.
- `startFeed`/`runningFeed` permiten observar stdout/stderr de un proceso que no
  termina solo y detenerlo en el cleanup.
- Casos nuevos: pull request mergeado, asignación con assignee, y una secuencia
  de frames inútiles (`id:` solo, comentario, `event:` solo, `retry:`, JSON
  truncado, cuerpo que no es JSON) seguida de dos eventos válidos. En todos se
  verifica que stdout no contenga `\x1b`, que el `url` crudo esté en la segunda
  línea, y que los frames descartados no corten la lectura.

## Comportamiento resultante

```text
$ feed watch
[pr.merged] Fix memory leak in parser component
AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/pull/42

[issue.assigned] lucia.gomez Ship the onboarding checklist
AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/issues/7
```

Con stdout en una terminal, el badge va coloreado, el assignee en bold y el
subject es un hyperlink OSC 8; el `url` crudo desaparece de la segunda línea.
Con `--no-color`, `NO_COLOR`, o un pipe, la salida es la de arriba.

## Verificación

```text
cd client && go test -count=1 ./...
?   	github.com/AG2904/github-activity-feed/client/cmd/feed	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/activity	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/cli	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/render	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/sse	[no test files]
ok  	github.com/AG2904/github-activity-feed/client/test	7.178s

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

La suite del cliente tiene 10 casos, todos por el límite del proceso. La ruta
con color se verificó a mano: se ejecutó el renderer contra un evento de ejemplo
y se inspeccionaron los bytes, que muestran `\x1b[33m[issue.assigned]\x1b[0m`,
`\x1b[1mlucia.gomez\x1b[0m` y el envoltorio
`\x1b]8;;<url>\x1b\\<subject>\x1b]8;;\x1b\\`.

## Acceptance Criteria

- ✅ AC1: `feed watch` mantiene abierta una conexión a `GET /stream` y pinta un evento cuando el relay escribe uno.
- ✅ AC2: Cada evento son dos líneas: badge, assignee cuando existe y subject; luego repositorio, actor y tiempo relativo.
- ✅ AC3: En `issue.assigned` el assignee va en bold inmediatamente después del badge; en `pr.merged`, donde es `null`, el slot desaparece sin dejar hueco.
- ✅ AC4: El subject lleva un hyperlink OSC 8 al `url` del evento.
- ✅ AC5: Los escapes se usan sólo cuando stdout es una terminal. Con pipe, `NO_COLOR` o `--no-color` no hay escapes y el `url` crudo aparece en la segunda línea.
- ✅ AC6: Se ignoran `id:`, `event:`, `retry:` y los frames de comentario; sólo los payloads de `data:` se convierten en eventos.
- ✅ AC7: Un frame que no parsea - JSON truncado, cuerpo que no es JSON, frame sólo con `id:` - se descarta sin terminar el proceso.

## Decisiones y límites conocidos

- **La ruta con color no se puede cubrir por el seam.** El spec prohíbe que un
  test importe el renderer, y un test de proceso no puede asignar un pty. Quedan
  cubiertos pipe, `NO_COLOR` y `--no-color`; bold y OSC 8 se verificaron a mano.
  El runbook del ticket 08 es el lugar para verlo contra el relay real.
- **Windows:** se habilita el procesamiento de terminal virtual antes de emitir
  escapes. Si el modo no se puede habilitar, `isTerminal` devuelve `false` y la
  salida va sin color antes que imprimir secuencias literales.
- **El assignee se pinta sin `@`.** El ticket pide el assignee en bold, no una
  decoración del login.
- **`repository: null` omite el segmento.** El spec pide no inventar un nombre
  para una fila anterior a la columna (ADR-0010).
- **Sin reconexión todavía.** Al cerrarse el stream, `watch` sale con `0`; el
  reintento con backoff y el timeout de inactividad son del ticket 04.
- **Deuda ajena a este ticket:** el árbol de `ARCHITECTURE.md` y los scripts npm
  siguen pendientes, y su criterio de aceptación está en el ticket 08.

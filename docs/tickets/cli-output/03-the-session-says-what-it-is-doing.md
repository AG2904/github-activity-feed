# Ticket 03 - The session says what it is doing

## Resumen

`feed watch` pasó de abrir y guardar silencio a narrar su propia vida en tres
líneas —banner, primera conexión, cuenta de salida—, todas en stderr, todas en
texto plano y ninguna gating por TTY. Los conteos son de los eventos
*mostrados*, filtro incluido, y salen del único lugar donde todos los caminos
(backfill, stream, `--json`) cruzan hacia stdout: `paint()`. No hay heartbeat ni
spinner: la única línea afirmativa es la de la primera conexión. La revisión de
dos ejes encontró un bug real —la línea `connected` se salteaba cuando el primer
intento fallaba— y dos pins que faltaban, todos corregidos antes del cierre.

## Cambios Realizados

### `client/internal/cli/watch.go`

- **Banner**: `watching <url> · backfill <n>`, con ` · user <login>` y
  ` · repo <owner/name>` en ese orden cuando los flags están puestos; sale antes
  de que algo pueda fallar.
- **Primera conexión**: el callback que `stream()` invoca cuando la respuesta
  fue aceptada conmuta sobre un booleano `accepted` —la primera conexión
  aceptada imprime `connected · <n> events from history` cuando el backfill
  imprimió algo, `connected · waiting for events` cuando no; toda conexión
  posterior imprime `watch: reconnected`, así la línea afirmativa aparece
  exactamente una vez. (La primera versión conmutaba sobre `attempts > 0` —es
  decir, sobre intentos *fallidos*—; si el relay arrancaba caído, la primera
  conexión aceptada imprimía `reconnected` y `connected` nunca aparecía. Lo
  encontró la revisión de spec y quedó pineado por
  `TestWatchSaysConnectedWhenTheFirstAttemptIsRefused`.)
- **Cuenta de salida**: `defer out.summarize(stderr)` cubre los cinco caminos de
  salida de `watch()` —Ctrl-C tras el stream, Ctrl-C durante el backoff, fallo de
  escritura del backfill, URL unusable, fallo de escritura en stdout—. Imprime
  `stopped · <n> events · <k> merged · <l> assigned` cuando se mostró algo, o
  `stopped · no events`. El defer está después del parseo de flags: un error de
  uso no recibe chrome.
- **Conteos**: `emitter` ganó `total` / `merged` / `assigned` y `count()`,
  invocado desde `paint()` —el único punto donde backfill, stream vivo y
  `--json` escriben—, contado recién cuando la escritura tuvo éxito. Los tipos
  usan las constantes de `activity` (ver más abajo). Un evento que el filtro
  descarta no llega a `paint()` y por eso no cuenta.
- El banner no lleva escape y ninguna línea de chrome llega a stdout; la línea
  de `connected` no se repite y no hay ningún heartbeat o spinner (pin de
  `TestWatchSaysConnectedOnceAcrossReconnects`, con un stderr exacto de 4
  líneas que también pinea el "no heartbeat").

### `client/test/harness_test.go`

- `startFeedWithPipeStdout` (stdout en un pipe controlable), `readUntil` y
  `waitExit`; después de la revisión, `startProcess` comparte el setup de
  arranque que antes duplicaba `startBinary`, y `endSession` concentra la
  coreografía de salida por plataforma: Windows rompe el pipe de stdout (la
  escritura falla, `watch` sale con 1 y el defer narra), no-Windows manda
  `os.Interrupt` —`signal.NotifyContext` en `main.go` cancela el contexto,
  `watch` sale con 0 y el defer narra—. Rama sin skip: es código ejecutado
  cuando el `GOOS` corresponde.

### `client/test/e2e_test.go`

- Nuevos: `TestWatchBannerNamesTheBackfillAndFilters`,
  `TestWatchSaysConnectedAfterTheFirstConnection`,
  `TestWatchSaysWaitingWhenTheBackfillPrintedNothing`,
  `TestWatchSaysConnectedWhenTheFirstAttemptIsRefused` (el bug de la revisión,
  en rojo primero),
  `TestWatchSaysConnectedOnceAcrossReconnects`,
  `TestWatchSaysStoppedWhenItLeavesThroughAFatalError` (URL unusable ⇒
  `stopped · no events`, exit 1), `TestWatchSaysStoppedWithTheCountsItShowed`,
  `TestWatchStoppedCountsLeaveOutWhatTheFilterDiscarded` (un evento descartado
  por el filtro no aparece en la cuenta).
- Reforzados: `TestWatchDefaultsToLocalhost3000` (banner completo) y
  `TestWatchJSONWritesTheFramePayloadVerbatim` (agrega `connected`).
- La revisión extrajo `hasEscape` y `connectedLines` para no repetir
  `ContainsRune(…, '\x1b')` y `Count(…, "connected · ")` por todo el archivo.

### Documentación

- `docs/cli-testing.md`: las tres líneas, a qué stream van y cómo se automatiza
  la cuenta de salida (Paso 12 reescrito tras los fixes: decía que no había
  automatización del interrupt y listaba dos caminos de salida).
- `docs/cli-flow.md` y `docs/end-to-end-testing.md`: descripción del chrome.
  Estaban correctas —el código estaba mal, no los docs—, y quedaron sin cambios
  salvo lo que la forma de la tarjeta ya traía del ticket 04.
- `.scratch/cli-output/spec.md`: una sola enmienda autorizada —el ejemplo del
  banner contradecía la prosa del mismo bullet y el ticket 03
  (`url · user · repo · backfill` contra `url · backfill · user · repo`); se
  corrigió el ejemplo, con nota en `## Comments`.

## Comportamiento resultante

```text
watching http://localhost:3000/stream · backfill 10 · user maria · repo AG2904/x
connected · 3 events from history
maria.perez merged into main
  [bug] Fix memory leak in parser component
  AG2904/github-activity-feed · 2h ago

stopped · 4 events · 3 merged · 1 assigned
```

- La línea `connected` aparece una sola vez, en la primera conexión *aceptada*;
  un relay que rechaza el primer intento la recibe igual (bug corregido).
- Reconexiones posteriores imprimen `watch: reconnected`; un segundo `connected`
  jamás.
- Toda salida de `watch` —Ctrl-C incluido, en cuanto la plataforma permite
  entregar la señal— cierra con `stopped · …`; la cuenta excluye lo que el
  filtro descartó; sin eventos mostrados, `stopped · no events`.
- stderr sigue siendo messaging: ninguna línea de chrome tiene escape ni llega
  a stdout; los fixtures exactos de stdout pasaron sin cambios y `--json` sigue
  byte-idéntico.

## Verificación

```text
go build ./...            exit 0
go vet ./...              exit 0
gofmt -l .                sin archivos
go test -count=1 ./...    ok  github.com/AG2904/github-activity-feed/client/test  47.287s
npm.cmd test              55/55 en 14.79s
```

(`-count=1` importa: el paquete de e2e compila el binario en `TestMain`, y sin
`-count=1` un cambio solo en `internal/` devuelve resultados cacheados.)

TDD en orden: cada caso fue rojo primero contra el binario anterior. La revisión
de dos ejes sobre la rama de integración aportó el bug del gating de `connected`
(corregido, rojo primero), el pin de "lo que el filtro descartó no cuenta"
(agregado), la cuenta de salida en ambas plataformas (la prueba de conteos solo
corría en Windows) y los refactors de standards citados arriba.

**No verificado**: (1) la rama no-Windows de `endSession` —`os.Interrupt`— está
escrita pero no se ejecutó en esta máquina: no hay WSL ni Docker, y en Windows
`Signal(os.Interrupt)` devuelve "not supported"; hace falta un `go test ./...`
en Unix para cerrarla. (2) El camino de Ctrl-C en Windows sigue siendo manual
(la consola no puede entregar la señal a un hijo): el defer lo cubre
estructuralmente y el paso manual está documentado en `docs/cli-testing.md`.

## Acceptance Criteria

- ✅ AC1: el banner dice `watching <url> · backfill <n>` con ` · user <login>` y
  ` · repo <owner/name>` en ese orden cuando los flags están puestos.
- ✅ AC2: tras la primera conexión aceptada, stderr lleva exactamente una línea
  —`connected · <n> events from history` o `connected · waiting for events`—;
  `<n>` cuenta lo que se imprimió, después del filtro.
- ✅ AC3: una reconexión posterior conserva `watch: reconnected` y no imprime un
  segundo `connected`.
- ✅ AC4: en toda salida de `watch`, stderr lleva `stopped · <n> events · <k>
  merged · <l> assigned` cuando se mostró algo, `stopped · no events` si no; la
  cuenta es de eventos mostrados, filtro incluido; otros tipos cuentan en `<n>`
  pero no en categoría.
- ✅ AC5: ninguna línea de chrome llega a stdout ni lleva escape; los fixtures de
  stdout exacto pasan sin cambios y `--json` sigue byte-idéntico.
- ✅ AC6: no hay heartbeat, spinner ni línea repetida: la única afirmativa es la
  de la primera conexión.
- ✅ AC7: `docs/cli-testing.md`, `docs/cli-flow.md` y `docs/end-to-end-testing.md`
  muestran las tres líneas y dicen a qué stream van.

# Ticket 04 - The reason rides with the title

## Resumen

La enmienda del 2026-10-22 del spec movió las etiquetas de la línea de contexto al
frente de la línea del título: el *por qué* llega antes que el *qué*. La segunda
línea de la tarjeta pasó de `Título` a `[bug] Título` —un espacio después del
corchete de cierre, en el orden que manda el evento— y la línea de contexto quedó
sólo con `repository · tiempo`. El cambio se implementó con TDD sobre el seam de
proceso (fixtures de stdout exacto primero) y la revisión de dos ejes agregó el pin
que faltaba para una tarjeta merged con etiquetas.

## Cambios Realizados

### `client/internal/render/render.go`

- `title()` ahora prefija las etiquetas: con `len(event.Labels) > 0` arma
  `[bug, feedback]` pintado en rojo bold (`p.paint(red, …)`) más un espacio, y lo
  antepone al título. Con color, el título sigue siendo bold y el hyperlink OSC 8
  —dos segmentos con un reset entre los dos, la pareja que resiste conhost—.
  Sin color, la línea arranca en `labels + event.Subject`.
- Un evento sin etiquetas no genera prefijo: sin `[]` vacío, sin espacio inicial,
  sin renglón extra.
- `context()` perdió el segmento de etiquetas: queda `repository · tiempo`,
  unido por el `·` gris, con cada segmento omitido si el evento no lo lleva
  (fila legacy con `repository: null` ⇒ sólo el tiempo). La capacidad del builder
  bajó de 3 a 2.
- Las tarjetas agrupadas pasan por el mismo `card()`: misma forma, sin el
  repositorio. `--json` no toca el renderer — byte-idéntico.

### `client/test/e2e_test.go`

- Se movieron los fixtures con la forma nueva: `assignedOutput` y
  `assignedGrouped` ahora esperan `[bug] Ship the…` en la línea 2 y el contexto
  sin el segmento de etiquetas; lo mismo en los `want` inline de
  `TestWatchRendersAnAssignmentWithoutAnAssignee`, `TestWatchRendersEveryLabel`
  (`[bug, feedback]`), `TestWatchRendersALegacyRowWithoutARepository`,
  `TestWatchFiltersCombineAsConjunction` y `TestHistoryFiltersCombineAsConjunction`.
- Tras la revisión, `mergedFrame` pasa a llevar `["bug"]`: la tarjeta merged pinea
  `[labels] title`, que era la mitad que el spec pinea y no estaba cubierta
  (`TestWatchPaintsAMergedPullRequest`). La otra mitad —una tarjeta sin etiquetas
  arranca en el título— quedó con caso propio,
  `TestWatchStartsLineTwoAtTheTitleWhenTheEventHasNoLabels`.
- Los fixtures byte de `--json` (`rawPayload`, `historyBody`) no se tocaron.

### Documentación

- `docs/cli-testing.md`: la muestra de la tarjeta y su prosa muestran las
  etiquetas antes del título y el contexto con `repository · tiempo`.
- `docs/end-to-end-testing.md`: el paso 6 muestra las dos tarjetas con la forma
  nueva.
- `docs/cli-flow.md`: la fila de `Labels`, la nota de `Parse()` sobre nil contra
  vacío, y la descripción de las tres líneas de la tarjeta.

## Comportamiento resultante

```text
lucia.gomez assigned to tomas.ferreira
  [bug, feedback] Ship the onboarding checklist
  AG2904/github-activity-feed · 2h ago

maria.perez merged into main
  [bug] Fix memory leak in parser component
  AG2904/github-activity-feed · 2h ago
```

Sin etiquetas, la línea 2 empieza en el título (así queda en la tarjeta merged
sin labels del caso pineado). Con color, `[bug]` sale `\x1b[1;31m` + un espacio +
el título en `\x1b[1m` con su par OSC 8; sin hyperlink no hay línea de URL, y
con un pipe la cuarta línea con la URL sigue apareciendo sin un solo `\x1b`.

## Verificación

```text
go build ./...            exit 0
go vet ./...              exit 0
gofmt -l .                sin archivos
go test -count=1 ./...    ok  github.com/AG2904/github-activity-feed/client/test  47.287s
npm.cmd test              55/55 en 14.79s
```

(`-count=1` importa: el paquete de e2e compila el binario en `TestMain`, y sin
`-count=1` un cambio sólo en `internal/` devuelve resultados cacheados.)

TDD en orden: los fixtures se movieron primero y cuatro casos fallaron en rojo
contra el binario viejo antes de tocar `render.go`. La revisión de dos ejes sobre
la rama de integración:

- **Spec**: faltaba el pin de `[labels] title` en una tarjeta merged con
  etiquetas —el spec lo pinea y su propio ejemplo lo muestra— (corregido con
  `mergedFrame` en `["bug"]`, rojo primero); el orden del ejemplo del banner en
  el spec contradecía su prosa (el ejemplo se corrigió, la corrección es del
  ticket 03).
- **Standards**: los literales `pr.merged` / `issue.assigned` estaban en dos
  packages —`render` y `watch`—, y un tipo nuevo exigía editar dos switches
  (corregido con constantes en `activity`, usadas en ambos sitios); el duplicado
  de setup del harness y la repetición de asserts de escapes en los tests
  (corregido con `startProcess` y los helpers `hasEscape` / `connectedLines`).

**No verificado**: los colores reales en una terminal —la misma frontera que el
ticket 02: un pipe no es TTY. La inspección de bytes con un programa temporal
(implementación del ticket, borrado después) confirmó el rojo bold del prefijo,
el bold del título, el par OSC 8 y la ausencia de línea de URL con color; el paso
manual está en `docs/cli-testing.md`.

## Acceptance Criteria

- ✅ AC1: la línea 2 de la tarjeta es `[labels] título` —un espacio exacto después
  del corchete, en el orden del evento (`[bug, feedback]`)—.
- ✅ AC2: un evento sin etiquetas arranca la línea 2 en el título: sin `[]` vacío,
  sin espacio inicial, sin línea en blanco extra (caso propio
  `TestWatchStartsLineTwoAtTheTitleWhenTheEventHasNoLabels`).
- ✅ AC3: la línea de contexto perdió las etiquetas y conserva `repository ·
  tiempo`, omitiendo cada segmento ausente; legacy `repository: null` ⇒ sólo el
  tiempo.
- ✅ AC4: con color, las etiquetas quedan rojo bold, el título sigue bold y es el
  hyperlink OSC 8, y la línea cruda de la URL aparece sólo sin hyperlinks.
- ✅ AC5: las tarjetas del reporte agrupado llevan la misma forma —`[labels]
  título` sin repositorio—.
- ✅ AC6: los fixtures sin color se movieron con la forma y nada más cambió;
  `--json` sigue byte-idéntico.
- ✅ AC7: `docs/cli-testing.md`, `docs/cli-flow.md` y `docs/end-to-end-testing.md`
  muestran la tarjeta con las etiquetas antes del título.

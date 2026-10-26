# Ticket 01 - The feed reads as a card

## Resumen

La salida humana del CLI dejó de ser un registro de dos líneas (`[pr.merged] …`) y
pasó a ser una tarjeta: una frase con quién actuó y dónde terminó, el título en su
propia línea, y el contexto con repositorio, etiquetas y tiempo relativo. El cambio se
implementó como una sola unidad, con los fixtures del seam de proceso actualizados
primero y la paleta por rol diferida al ticket 02, que está bloqueado por éste.

## Cambios Realizados

### `client/internal/render/render.go`

- La tarjeta se arma en `card()`: `headline()` (actor + verbo + destino), `title()`
  (título, negrita y OSC 8 cuando hay color) y `context()` (repositorio · etiquetas ·
  tiempo). La URL pasa a ser una cuarta línea **solo** cuando no hay hyperlink.
- `headline()` traduce el tipo a vocabulario humano —`merged` / `assigned`— y deriva el
  destino del tipo, porque el evento no lleva rama. Un tipo desconocido cae a su valor
  crudo, sin destino, en vez de imprimir una línea vacía.
- Las etiquetas, que antes se parseaban y se descartaban, se muestran entre corchetes.
  Un evento sin etiquetas omite el segmento en vez de imprimir `[]`.
- `relative()` gana `yesterday` en la ventana de 24 a 48 horas.
- `badge()` y `subject()` desaparecen; el peso y el hyperlink viven en `title()`. El
  archivo ya no define colores por rol: eso es el ticket 02.

### `client/internal/render/grouped.go`

- El reporte gana la línea en blanco entre el encabezado y su primera tarjeta, y entre
  tarjeta y tarjeta; la unidad sin repositorio también queda separada.
- Todas las escrituras de separación se controlan (`blank()`) y un stdout cerrado
  devuelve error en vez de perderse; `heading()` escribe con `io.WriteString`.

### `client/internal/cli/watch.go`, `stored.go`, `backfill.go`

- `emitter` pasa a puntero y recuerda si ya imprimió una tarjeta (`printed`): la línea
  en blanco se escribe **antes** de cada tarjeta que no es la primera. Así el último
  evento no deja un hueco bajo el prompt y `--json` no recibe ninguna.
- `paint()` es el único lugar que decide entre payload crudo y tarjeta; el backfill y
  el stream siguen compartiendo esa regla.

### `client/test/e2e_test.go`

- Fixtures nuevos para las dos tarjetas y para sus formas agrupadas; el helper `cards()`
  une tarjetas con la línea en blanco que las separa.
- Casos nuevos: asignación sin assignee, fila legacy sin repositorio, dos etiquetas y
  la frontera `yesterday` (30 h) contra `2d ago` (50 h).
- El caso del backfill por defecto cuenta cuatro líneas por tarjeta sin color (40 para
  diez eventos, no 20).

### Documentación

- `docs/cli-testing.md`: salida esperada del paso 4 y la descripción del render.
- `docs/cli-flow.md`: el render es una tarjeta; la referencia a `Palette.badge()` se
  reemplaza por el puntero al ticket de color.
- `docs/end-to-end-testing.md`: el paso 6 muestra tarjetas y explica la cuarta línea;
  la prosa del paso 4 deja de hablar de "dos líneas" en modo `--json`.
- `.scratch/cli-output/spec.md`: la decisión de separación se enmienda para nombrar la
  excepción del reporte agrupado, que es un documento y no una secuencia.

## Comportamiento resultante

```text
maria.perez merged into main
  Fix memory leak in parser component
  AG2904/github-activity-feed · 2h ago
  https://github.com/AG2904/github-activity-feed/pull/42

maria.perez assigned to lucia.gomez
  Ship the onboarding checklist
  AG2904/github-activity-feed · [bug] · 2h ago
  https://github.com/AG2904/github-activity-feed/issues/7
```

Sin color (pipe o `--no-color`) la tarjeta tiene cuatro líneas; en una terminal con
color el título es el hyperlink y la cuarta desaparece. `--json` sigue entregando los
bytes del relay, uno por línea, sin líneas en blanco.

## Verificación

```text
go build ./...            exit 0
go vet ./...              exit 0
gofmt -l .                sin archivos
go test -count=1 ./...    ok  github.com/AG2904/github-activity-feed/client/test  38.195s
npm.cmd test              55/55 tests en 14.59s (relay intacto)
```

Revisión de dos ejes sobre el diff sin commitear, contra `019f76a`:

- **Standards**: cinco escrituras de `fmt.Fprintln` sin controlar en `grouped.go`, dos
  referencias a métodos/pasos borrados en la documentación y un comentario de
  `history.go` que seguía diciendo "dos líneas". Todo corregido; `line()` se renombró a
  `card()` porque ya no describe una línea.
- **Spec**: faltaban casos para dos etiquetas y para la fila legacy *con* etiqueta (el
  test usaba lista vacía), y la separación del reporte agrupado contradecía la decisión
  "la separación es del emisor". Se agregaron los dos casos y se enmendó el spec con la
  excepción documentada.

**No verificado**: el camino con `Color = true`. La suite corre con stdout en un pipe,
así que solo puede afirmar la ausencia de escapes; el hyperlink y la negrita del título
se verifican a mano cuando se implemente el ticket 02.

## Acceptance Criteria

- ✅ AC1: un merge renderiza tres líneas — `actor merged into main`, el título y
  `repository · [labels] · time`.
- ✅ AC2: una asignación renderiza `actor assigned to assignee`, y `assigned` solo
  cuando el evento no lleva assignee.
- ✅ AC3: la línea de contexto omite el segmento que el evento no trae (sin repositorio
  legacy, sin corchetes cuando la lista está vacía).
- ✅ AC4: las etiquetas aparecen entre corchetes y separadas por coma (`[bug]`,
  `[bug, feedback]`), con un caso pinned para las dos.
- ✅ AC5: `yesterday` entre 24 y 48 horas, y `Nd ago` después (probado a 30 h y 50 h).
- ✅ AC6: sin color la URL es una cuarta línea indentada; con color el título es el
  hyperlink y no se imprime la URL.
- ✅ AC7: una línea en blanco entre tarjetas, ninguna antes de la primera ni después de
  la última; `--json` sin líneas en blanco.
- ✅ AC8: el vivo, el backfill y `history` imprimen la misma tarjeta; el reporte
  agrupado conserva el encabezado y omite el repositorio en cada tarjeta.
- ✅ AC9: `--json` idéntico byte a byte y relay intacto (55/55).
- ✅ AC10: fixtures, `cli-testing.md`, `cli-flow.md` y `end-to-end-testing.md`
  actualizados.

# Ticket 02 - Colour tells the roles apart

## Resumen

La tarjeta del ticket 01 estrena paleta: el color codifica roles —personas, estado,
lugar y motivo— y no decora. Se aplicó con códigos de 16 colores, sólo en terminal, y
sin que el significado dependa del color. La verificación es de bytes, porque el seam
del proceso no puede observar un TTY; el paso manual quedó escrito en el runbook.

## Cambios Realizados

### `client/internal/render/render.go`

- Constantes de rol: cian bold para personas, verde bold para integración, ámbar bold
  para asignación, azul para el repositorio, rojo bold para etiquetas y gris para el
  contexto.
- `paint()` es la única puerta de los segmentos planos: con `Color` apagado devuelve el
  texto sin un byte de escape, así que un pipe no puede recibir color por accidente.
- `headline()` pinta actor, verbo, conector y destino; `context()` pinta repositorio,
  etiquetas, separadores y tiempo. El separador se pinta entero —`" · "`— para que el
  gris no quede sólo en el glifo.
- Un tipo de evento desconocido se pinta en gris, no en ámbar: no tiene rol propio y la
  regla del spec es que herede contexto hasta que se le asigne uno.
- El gris es *bright black* (`90`), no *faint* (`2`), porque conhost con VT ignora
  *faint* y el contexto perdería su de-énfasis justo en la consola que la paleta toma
  como límite.

### `client/internal/render/grouped.go`

- El encabezado del repositorio pasa a bold azul y usa `paint()` como el resto, en vez
  de componer sus propios escapes.

### Documentación

- `docs/cli-testing.md`: tabla de roles, la aclaración de `90` contra `2`, y qué puede y
  qué no afirmar la suite (ausencia de escapes por partida doble: casos explícitos y
  fixtures de salida exacta).
- `docs/cli-flow.md`: el bullet de color describe la paleta que vive en `render.go`.
- `.scratch/cli-output/spec.md`: la tabla pasa de "dim" a "gris tenue (`90`)" y registra
  el motivo de portabilidad.

## Comportamiento resultante

Con color, la tarjeta queda así (se muestra el escape como `<E>`, salida real del
programa de verificación):

```text
<E>[1;36mmaria.perez<E>[0m <E>[1;32mmerged<E>[0m <E>[90minto<E>[0m <E>[1;32mmain<E>[0m
  <E>[1m<E>]8;;https://example/pull/42<E>\Fix memory leak in parser component<E>]8;;<E>\<E>[0m
  <E>[34mAG2904/github-activity-feed<E>[0m<E>[90m · <E>[0m<E>[1;31m[bug]<E>[0m<E>[90m · <E>[0m<E>[90m2h ago<E>[0m
```

Sin color, exactamente el mismo texto del ticket 01, sin un solo `\x1b`.

## Verificación

```text
go build ./...            exit 0
go vet ./...              exit 0
gofmt -l .                sin archivos
go test -count=1 ./...    ok  github.com/AG2904/github-activity-feed/client/test  38.349s
npm.cmd test              55/55 en 14.44s
```

Además, un programa temporal (no commiteado) imprimió las tarjetas con `Color = true`
para inspeccionar los bytes: confirmó `1;36`, `1;32`, `90`, `1;33`, `34`, `1;31`, `1m`
y el par OSC 8, y que el encabezado agrupado sale `1m` + `34m`.

Revisión de dos ejes sobre el diff sin commitear, contra `ca72dbd`:

- **Standards**: el encabezado agrupado recomponía escapes en vez de pasar por `paint`
  (corregido), y el comentario de `paint` afirmaba una invariante que el título no
  cumple (corregido). Sugirió un test unitario del paquete `render` para pinnear los
  tonos; se descartó porque la decisión de testing del spec fija el seam en el proceso y
  prohíbe tests que importen paquetes internos.
- **Spec**: el separador quedaba medio gris (corregido), el tipo desconocido tomaba
  prestado el ámbar de la asignación (corregido) y `dim` no existe en conhost con VT
  (cambiado a `90`, con la nota en el spec).

**No verificado**: la inspección visual en una terminal real. La evidencia es de bytes y
el paso manual —comparar contra la tabla en Windows Terminal y en un conhost con VT—
está escrito en `docs/cli-testing.md`.

## Acceptance Criteria

- ✅ AC1: actor y assignee en cian bold; `merged` en verde bold; `assigned` en ámbar
  bold; `main` en verde bold.
- ✅ AC2: `into` y `to` en gris; repositorio azul; etiquetas rojo bold; separadores y
  tiempo en gris; título bold.
- ✅ AC3: el título sigue siendo el hyperlink OSC 8 y la línea cruda de la URL no se
  imprime con color.
- ✅ AC4: ningún escape cuando stdout no es terminal, con `--no-color` o con `NO_COLOR`;
  los fixtures sin color siguen pasando sin cambios.
- ✅ AC5: sólo códigos SGR de 16 colores; sin 256, emoji, box drawing ni reverse video.
- ✅ AC6: `docs/cli-testing.md` documenta la verificación manual y el límite de la suite.

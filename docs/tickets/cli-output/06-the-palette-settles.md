# Ticket 06 - The palette settles — soft roles, one reason

## Resumen

La paleta deja de gritar. Los seis roles se muestran en peso regular (el viejo usaba
`1;3x` en cuatro de ellos, que la mayoría de terminales renderiza como *bright*), el
repositorio baja a gris de contexto, la rama conserva el azul de ubicación, el motivo
pasa de rojo bold a magenta y el título sigue siendo el único bold de la tarjeta.
La elección se hizo con los ojos, desde el prototipo descartable de la rama
`prototype/palette-options` (`ea100ee`, variante `soft reason`). Siguen siendo 16
colores: cambian los slots, no el piso de portabilidad. La salida sin color quedó
byte-idéntica.

## Cambios Realizados

### `client/internal/render/render.go`

- Constantes por rol en peso regular: `cyan` (`36`), `green` (`32`), `yellow`
  (`33`), `blue` (`34`), `magenta` (`35`), `grey` (`90`), más `bold` (`1`) y
  `reset`. No queda ningún `1;3x`: el bold se reserva para el título.
- El repositorio se muestra en `grey`; la rama conserva `blue`; las etiquetas pasan de
  `red` bold a `magenta`; el conector y el tiempo siguen en gris.
- `paint()` sigue siendo la única puerta de los segmentos planos; el título arma su
  propio `bold` + OSC 8, como antes.
- La revisión corrigió el comentario de roles (decía que solo el título llevaba bold
  —el encabezado agrupado también—, llamaba "el lugar" al repositorio y afirmaba que
  magenta era la única nota cálida con amarillo en la paleta) y renombró `amber` a
  `yellow`, porque SGR `33` es amarillo y así lo nombran el spec y los docs.

### `client/internal/render/grouped.go`

- Solo el comentario: el encabezado agrupado sigue en bold + azul porque es la única
  estructura del documento, y el azul se explica como el rol de ubicación que la rama
  lleva en el feed. El código no cambió.

### Spec y documentación

- `.scratch/cli-output/spec.md`: la tabla de la paleta se reemplaza por los slots
  nuevos con sus razones (rama = la ubicación que importa; repositorio = contexto como
  el tiempo; etiquetas = el motivo, cálido sin la alarma del rojo), se agrega el
  comentario fechado que nombra el prototipo y retira la tabla vieja, y se marca como
  superada la frase del ticket 05 que ubicaba a la rama "en el rol de lugar del
  repositorio". La justificación de la enmienda se corrige: la causa era el peso
  (cuatro roles bold renderizados como bright) y el rojo bold de las etiquetas, no la
  cantidad de tonos.
- `docs/cli-testing.md`: tabla de roles y paso manual actualizados; la verificación de
  tono queda escrita para **Windows Terminal** y un **conhost con VT**, porque el tema
  aplica el RGB y el binario solo elige el rol. La nota del reporte agrupado aclara que
  el gris del repositorio en la tarjeta y el azul bold del encabezado no se
  contradicen: uno es contexto, el otro es la estructura del documento.
- `docs/cli-flow.md`: el párrafo de la paleta describe el mapeo nuevo.
- `docs/adr/0013-branch-as-a-field-of-the-event.md`: se quita la frase que decía que el
  repositorio compartía el rol de ubicación de la rama.

## Comportamiento resultante

Byte check con un programa temporal (borrado después; el seam no tiene TTY), tarjeta
merged:

```text
\x1b[36mmaria.perez\x1b[0m \x1b[32mmerged\x1b[0m \x1b[34mfix/memory-leak\x1b[0m \x1b[90minto\x1b[0m \x1b[32mmain\x1b[0m
  \x1b[35m[bug]\x1b[0m \x1b[1m\x1b]8;;https://github.com/AG2904/github-activity-feed/pull/42\x1b\\Fix memory leak in parser component\x1b]8;;\x1b\\\x1b[0m
  \x1b[90mAG2904/github-activity-feed\x1b[0m\x1b[90m · \x1b[0m\x1b[90m2h ago\x1b[0m
```

Tarjeta assigned: actor `36`, `assigned` `33`, `to` `90`, assignee `36`, etiquetas
`35`, título `1` + OSC 8, repositorio/tiempo `90`. No aparece ningún `1;3x`.

Sin color, la tarjeta es exactamente la misma de antes: los fixtures del seam no se
tocaron.

## Verificación

```text
go build ./...            exit 0
go vet ./...              exit 0
gofmt -l .                sin archivos
go test -count=1 ./...    ok  github.com/AG2904/github-activity-feed/client/test  45.8s
```

La implementación corrió en 44.8 s y los fixes en 41.9 s; el gate final, tras el merge
de fixes, en 45.8 s. El relay no se tocó (el cambio es solo del cliente) y el diff de
la integración no incluye `src/` ni `client/internal/activity`.

Revisión de dos ejes sobre la paleta: tabla del spec y código coinciden slot por slot,
`paint()` sigue siendo la única puerta, la salida sin color no se movió. Los hallazgos
fueron de prosa y un nombre: `amber` → `yellow`, el comentario de roles, el comentario
del encabezado agrupado, la nota de supersesión en el spec, la justificación de
"cinco tonos" y las tres menciones de "la única nota cálida" (spec, `cli-testing`,
`cli-flow`, más el propio ticket). Todo corregido en `6f0290e`.

**No verificado**: la inspección visual de los tonos en una terminal real. La suite
solo puede afirmar la ausencia de escapes; el paso manual —Windows Terminal y conhost
con VT— quedó escrito en `docs/cli-testing.md` y lo corre una persona.

## Acceptance Criteria

- ✅ AC1: la tabla de la paleta del spec se reemplaza por los slots elegidos —actor y
  assignee `36`, `merged` y `main` `32`, `assigned` `33`, rama `34`, repositorio y
  `·`/tiempo `90`, etiquetas `35`, título `1` + OSC 8—, con el comentario fechado que
  nombra el prototipo y explica el retiro de la tabla vieja.
- ✅ AC2: `render.go` muestra los slots nuevos y nombra los constantes honestamente; no
  queda ningún `1;3x` y `paint()` sigue siendo la única puerta.
- ✅ AC3: con color apagado la salida es byte-idéntica: los fixtures pineados pasan
  sin tocarse.
- ✅ AC4: el byte check (programa temporal) muestra exactamente esos códigos y ningún
  `1;3x`.
- ✅ AC5: `docs/cli-testing.md` (tabla de roles y paso manual) y `docs/cli-flow.md`
  (descripción de la paleta) describen el mapeo nuevo; las muestras siguen al cambio.
- ✅ AC6: el paso manual está escrito para Windows Terminal y conhost con VT.
- ✅ AC7: la suite del cliente está verde; el relay no se tocó y no es parte de este
  cambio.

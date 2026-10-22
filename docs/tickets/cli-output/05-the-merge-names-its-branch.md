# Ticket 05 - The merge names its branch

## Resumen

La tarjeta de un merge deja de decir sólo dónde terminó el trabajo y dice de dónde
vino: `maria.perez merged fix/memory-leak into main`. El relay lee
`pull_request.head.ref` —un dato que ya llegaba en el webhook y se descartaba— y
`ActivityEvent` gana `branch` como noveno campo **nullable**, con las mismas
consecuencias que ADR-0010 fijó para `repository`: las filas guardadas antes de la
columna responden `null` y la frase cae al `merged into main` de antes. `--json`
cambia porque cambia la forma del relay, no porque el CLI reescriba bytes: el
cliente sigue entregando el payload verbatim.

## Cambios Realizados

### Relay (`src/relay/`)

- `github/merge-to-main.ts`: el payload declara `head.ref` y lo mapea a `branch`.
  Un head ref ausente, no-string o vacío normaliza el evento con `branch: null`;
  el evento nunca se descarta por un detalle suplementario.
- `domain/activity-event.ts`: noveno campo `branch: string | null`; el comentario
  de encabezado pasa a "Nine fields, no more" y describe la rama como la línea de
  trabajo de la que vino el cambio, no como el destino.
- `activity-event.entity.ts`: columna `text` nullable documentada con sus **dos**
  causas —la fila que predata la columna y el evento que no tiene rama por diseño
  (asignación o head ref inutilizable)—, manteniendo la explicación de SQLite y
  `synchronize: true` que hace obligatoria la nulabilidad.
- `activity-events.service.ts`: `fromStoredRow` sirve `branch`.
- `github/issue-assignment.ts`: las asignaciones llevan `branch: null`.
- `test/app.e2e-spec.ts`: fixtures y aserciones de forma con nueve campos; casos
  de head ref ausente/no-string/vacío; arranque sobre una base pre-columna con
  `repository: null, branch: null`.

### Cliente (`client/`)

- `internal/activity/activity.go`: `Event.Branch *string`, campo de wire y `Parse`;
  el comentario de nulabilidad distingue las tres razones (columna nueva, evento
  sin rama, `assignee` en todo `pr.merged`).
- `internal/render/render.go`: `headline()` arma `actor merged <branch> into main`
  cuando la rama existe y conserva `merged into main` cuando es `null` o vacía. La
  rama se pinta con `p.paint(blue, …)` —rol de lugar, el mismo del repositorio—, y
  la tarjeta agrupada pasa por el mismo `card()` (sólo se le quita el repositorio).
- `test/e2e_test.go`: `mergedFrame` y los fixtures JSON con `branch`;
  `TestWatchPaintsAMergedPullRequest` pinea la frase con rama y
  `TestWatchRendersAMergeWithoutABranch` el fallback; grouped y backfill siguen
  la misma forma.

### Contrato, ADR y documentación

- `docs/adr/0013-branch-as-a-field-of-the-event.md`: nuevo ADR que decide el
  noveno campo, sus opciones descartadas y sus consecuencias (legacy null,
  fallback, `--json`). ADR-0005 y ADR-0010 llevan el bloque "Amended by
  ADR-0013" —sólo cambia el conteo de campos, el resto de cada decisión sigue
  en pie— y ADR-0013 apunta de vuelta.
- `.scratch/cli-output/spec.md`: retira la nota "The event carries no branch",
  suma la fila azul de la rama a la tabla de la paleta, enmienda la cláusula de
  Out of Scope y corrige la línea de Scope para no prometer más que eso:
  `--json` sigue verbatim del relay, y lo que cambió es la forma del relay.
- `.scratch/relay/spec.md` (historia 10 y Normalization) y `.scratch/cli/spec.md`
  (Normalization) cuentan nueve campos.
- `docs/relay-flow.md` ("Nueve campos", nulabilidad por obligación y por diseño),
  `docs/manual-testing.md` (payload y JSON de historia con `branch`),
  `docs/cli-testing.md`, `docs/cli-flow.md`, `docs/end-to-end-testing.md`,
  `ARCHITECTURE.md` y `GLOSSARY.md`.
- ADR-0009 y los artefactos generados de `.archify/` conservan la mención
  histórica de ocho campos: son registros de su momento, no guía vigente.

## Comportamiento resultante

```text
maria.perez merged fix/memory-leak into main
  [bug] Fix memory leak in parser component
  AG2904/github-activity-feed · 2h ago
```

- Una fila legacy —o un merge sin head ref utilizable— sigue leyéndose
  `maria.perez merged into main`; el fallback es deliberado y está pineado.
- La misma tarjeta aparece en vivo, en el backfill, en `history` y en el reporte
  agrupado.
- `--json` lleva `"branch"` para eventos nuevos y `null` para los legacy; los
  demás campos conservan nombre, orden y valor, y el CLI sigue escribiendo los
  bytes del relay tal cual.

## Verificación

```text
npm.cmd run build         exit 0
npm.cmd test              57/57 en 14.69s (integración); 18.85s (post-fixes)
go build ./...            exit 0
go vet ./...              exit 0
gofmt -l .                sin archivos
go test -count=1 ./...    ok  github.com/AG2904/github-activity-feed/client/test  50.2s
```

TDD en orden: `TestWatchPaintsAMergedPullRequest` falló en rojo contra el binario
anterior (`merged into main` contra `merged fix/memory-leak into main`), y en el
relay fallaron cinco casos antes de implementar (rama ausente en el evento, head
ref inutilizable, deep equality de nueve campos y el arranque sobre base legacy).

Revisión de dos ejes sobre la rama de integración:

- **Standards**: el comentario de `activity.go` había invertido `pr.merged` por
  `issue.assigned` en un find-and-replace (las asignaciones sí tienen assignee);
  ADR-0005 seguía afirmando "exactly eight fields" sin marca de enmienda; la
  entidad decía que `branch` era nullable "only because of the upgrade" cuando
  ADR-0013 registra dos causas. Corregidos.
- **Spec**: la línea de Scope del spec seguía prometiendo que `--json` no cambia;
  `relay-flow.md`, `manual-testing.md`, `.scratch/relay/spec.md`, `.scratch/cli/spec.md`
  y el comentario de `history.go` seguían contando ocho; el encabezado del dominio
  etiquetaba la rama como "where it landed". Corregidos.
- **Judgement calls aceptados con razón escrita**: `eventFrame` con nueve
  argumentos (helper de test, se revisa si aparece un décimo campo); el guard
  `Branch != nil && *Branch != ""` del cliente (espeja la regla `hasRepository`
  y protege de un relay malformado); `head: { ref: string }` declarado requerido
  y leído defensivamente (estilo del resto de las interfaces de payload); los dos
  tests de null (pinean causas distintas: rama ausente y repositorio legacy).

**No verificado**: el azul del segmento de rama no se inspeccionó en bytes ni a
ojo. La suite del seam corre sin color, la misma frontera que el ticket 02; el
paso manual queda en `docs/cli-testing.md`.

## Acceptance Criteria

- ✅ AC1: un merge a main lleva la rama de origen; el relay mapea
  `pull_request.head.ref` a `branch`, y un head ref ausente, no-string o vacío
  normaliza con `branch: null` sin perder el evento.
- ✅ AC2: `ActivityEvent` gana `branch: string | null` como noveno campo y el
  comentario de "ocho campos" se actualiza; la columna es nullable por la razón
  de SQLite/`synchronize` (ADR-0010) y las filas previas leen `null`.
- ✅ AC3: el cliente parsea `branch` como nullable; la tarjeta dice
  `actor merged <branch> into main` cuando existe y `actor merged into main`
  cuando es null, sin segmento vacío ni `into main` duplicado.
- ✅ AC4: la misma tarjeta aparece en vivo, backfill, `history` y el reporte
  agrupado (que sigue omitiendo sólo el repositorio).
- ✅ AC5: `--json` lleva `branch` y `null` para legacy; el contrato y los
  fixtures se movieron juntos; el CLI sigue entregando los bytes verbatim y los
  demás campos conservan nombre, orden y valor.
- ✅ AC6: la decisión quedó registrada en un ADR nuevo (0013) y en la enmienda
  del spec, que levanta la cláusula de out-of-scope y retira la nota de "event
  carries no branch".
- ✅ AC7: `docs/cli-testing.md`, `docs/cli-flow.md` y
  `docs/end-to-end-testing.md` muestran la frase con rama y el fallback;
  `GLOSSARY.md` nombra el campo.
- ✅ AC8: ambas suites verdes, con fixtures que cubren merged con rama, merged
  con `branch: null` y la forma de `--json`.

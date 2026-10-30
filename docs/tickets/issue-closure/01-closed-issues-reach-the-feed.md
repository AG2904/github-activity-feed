# Un issue cerrado con label crítica llega al feed

## Resumen

El relay captaba el comienzo del trabajo urgente —un issue con `bug` o `feedback`
asignado— pero no su final. Un cierre no dejaba rastro: ni fila en el historial ni
línea en el feed. Ahora una entrega `issues` con `action: "closed"` y una etiqueta
crítica se guarda como evento, y el motivo que reporta GitHub lo separa en dos tipos
para no mezclar noticias distintas: `issue.closed` cuando el cierre se reporta como
resuelto (`completed`, o sin motivo) y `issue.dismissed` cuando se reporta como
`not_planned`. El CLI muestra `actor closed` o `actor dismissed as not planned`.

El contrato de nueve campos no se movió: el motivo viaja en `type`, que ya era un
conjunto cerrado de literales (ADR-0016). No hay campo nuevo, no hay migración y las
filas existentes no cambian.

## Cambios Realizados

### Relay

- **`src/relay/domain/activity-event.ts`**: `ActivityEventType` suma `issue.closed` e
  `issue.dismissed`.
- **`src/relay/github/issue-closure.ts`** (nuevo): filtro y normalización del cierre
  en una sola pasada. Filtra por `action === 'closed'` y por la etiqueta crítica en
  ese momento; normaliza el tipo desde `issue.state_reason`, el actor desde
  `sender.login`, el assignee desde el issue (no desde el nivel superior de la
  entrega) y `when` desde `issue.updated_at`. Un cierre que pasa el filtro y no se
  puede normalizar responde `malformed`.
- **`src/relay/github/to-activity-event.ts`**: el cierre es el tercer paso de la
  composición, después del merge y de la asignación, y `malformed` sigue siendo
  terminal.
- **`src/relay/github/payload.ts`** / **`issue-assignment.ts`**: `reportedAssignees`
  se mudó a `payload.ts`, donde vive la lectura defensiva compartida; los dos
  adaptadores la importan en lugar de duplicarla.
- **`test/app.e2e-spec.ts`**: cuatro casos nuevos del cierre (resuelto, descartado sin
  assignee, sin motivo reportado, sin actor) y el caso negativo del barrido (cierre
  sin etiqueta crítica).

### CLI

- **`client/internal/activity/activity.go`**: `TypeIssueClosed` y
  `TypeIssueDismissed`.
- **`client/internal/render/render.go`**: `closed` en verde y `dismissed as not
  planned` en amarillo, con las etiquetas al frente del título como en todo evento.
- **`client/internal/cli/emitter.go`**: contadores `closed` y `dismissed`, y la línea
  de cuenta `stopped · N events · N merged · N assigned · N closed · N dismissed`.
- **`client/test/e2e_test.go`**: las dos tarjetas, el filtro `--user` por el assignee
  de un cierre, y la cuenta de la sesión con los cuatro tipos.

### Documentación

- `docs/adr/0016-issue-closure-splits-by-state-reason.md`: la decisión de llevar el
  motivo en `type` y las alternativas descartadas.
- `GLOSSARY.md`: `Closure` y `Dismissal`, y la definición de evento crítico.
- `docs/relay-flow.md`, `docs/cli-flow.md`, `ARCHITECTURE.md`, `README.md`,
  `docs/manual-testing.md`, `docs/end-to-end-testing.md`, `.scratch/relay/spec.md` y
  `.scratch/cli-output/spec.md`.

## Evidencia

- `npm test`: 2 archivos, **67 casos verdes** (eran 62; cinco casos nuevos, ninguno
  existente editado).
- `npm run build`: sin errores. `npm run lint`: 0 warnings, 0 errores.
- `npm run cli:test`: `ok github.com/AG2904/github-activity-feed/client/test`
  (55.9 s). `go vet ./...`: limpio. `gofmt -l .`: sin archivos.
- `npm run check:docs`: 104 archivos markdown en UTF-8 plano.
- **Diferencial**: los casos existentes de `pr.merged` e `issue.assigned` siguen
  comparando el objeto completo de nueve campos y pasan sin edición, y las tarjetas
  de ambos tipos siguen comparando stdout exacto. La fila del merge y la de la
  asignación son idénticas a la base para los mismos payloads.

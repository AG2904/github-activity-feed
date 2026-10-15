# Ticket 02 - Un merge a `main` se convierte en evento

## Resumen

Un pull request que GitHub reporta como cerrado y mergeado hacia `main` se
almacena como `ActivityEvent` y se puede leer de vuelta por HTTP. Es el primer
camino real del Relay: **filtrar → normalizar → persistir → leer**.

Ticket ejecutado ya con la migración a GitHub cerrada: el filtro keyea sobre el
estado de merge reportado por GitHub (`action: closed` + `pull_request.merged`),
nunca sobre detalles intermedios del payload.

## Cambios Realizados

### src/relay/domain/activity-event.ts

- El contrato del dominio (vocabulario del GLOSSARY): `type`, `actor`, `subject`,
  `assignee`, `labels`, `url`, `when`. `when` es la forma canónica UTC ISO-8601.

### src/relay/activity-event.entity.ts

- Entidad TypeORM `ActivityEventEntity` sobre la tabla `activity_events`
  (ADR-0004). `labels` se persiste como columna JSON.

### src/relay/merge-to-main.ts

- **`isMergeToMain(payload)`**: predicado puro sobre el estado reportado —
  `action === "closed"` + `pull_request.merged === true` + `base.ref === "main"`.
- **`toActivityEvent(payload)`**: normalización a la forma `ActivityEvent` —
  `type: "pr.merged"`, `actor = sender.login` (quien mergeó), `subject` = título
  del PR, `assignee` vacío para merges, `labels` aplanadas a strings,
  `url = html_url`, `when = new Date(merged_at).toISOString()`.
- **Guardas de normalización**: si el payload pasa el filtro pero no puede
  normalizarse (sin `labels`, timestamp ilegible), devuelve `undefined` — el
  request se acusa igual sin almacenar, como manda la spec ("ignored without
  error") en vez de responder un `500`.

### src/relay/activity-events.service.ts

- `store(event)` y `list()`. La lectura proyecta explícitamente los siete campos
  del contrato: el `id` de storage nunca se fuga al shape servido. Orden:
  newest-first.

### src/relay/webhook.controller.ts

- Orquestación tras validar la firma: parseo del cuerpo crudo (JSON malformado se
  ignora), filtro, normalización y persistencia. Crítico o no, siempre responde
  `200 { received: true }`.

### src/relay/history.controller.ts

- `GET /history` ahora lee del servicio y devuelve los `ActivityEvent` guardados.

### src/app.module.ts

- La entidad se registra en el wiring de TypeORM y la tabla se crea con
  `synchronize: true` (convención de la fase de desarrollo sobre SQLite).

### test/app.e2e-spec.ts

- Casos nuevos sobre la misma seam HTTP: PR mergeado a `main` → guardado y
  legible con la forma exacta esperada; timestamp con offset `+02:00` → guardado
  como `12:00:00.000Z`; PR mergeado a otra rama → no guardado; PR cerrado sin
  merge → no guardado; payload "crítico" no normalizable → no guardado (antes:
  `500`). Total de la suite: 13 casos.

## Comportamiento resultante

- Un merge a `main` queda en el historial de auditoría con actor, subject,
  labels, URL y timestamp canónico, en la misma forma en que se almacena.
- Los eventos no críticos (otra rama, sin merge) y los no normalizables se acusan
  con `200` sin persistir: activar una integración ajena en GitLab/GitHub no puede
  romper ni ensuciar el historial.
- La base sobrevive reinicios: un archivo SQLite embebido, una tabla.

## Verificación

```text
npm test        → 13 casos e2e en verde
tsc --noEmit    → exit 0
oxlint          → 0 warnings, 0 errors
```

La revisión previa al cierre destapó el `500` ante payloads no normalizables
(hallazgo del eje Spec): corregido con guardas en el normalizador y un caso e2e
propio. La forma canónica del timestamp quedó fijada por test
(`toISOString()`, UTC con milisegundos).

## Acceptance Criteria

- ✅ AC1: Un PR reportado como cerrado y mergeado con base `main` se almacena.
- ✅ AC2: La detección keyea sobre el estado de merge reportado
  (`action: closed` + `merged`), nunca sobre detalle intermedio del payload.
- ✅ AC3: El evento guardado lleva type, actor, subject, assignee, labels, URL y
  timestamp.
- ✅ AC4: Los labels se guardan como strings planos, no como objetos de GitHub.
- ✅ AC5: El evento es legible desde el endpoint de historial con la misma forma
  en que se almacena.
- ✅ AC6: Los timestamps se guardan en una única forma canónica (UTC ISO-8601)
  sin importar el timezone del emisor.

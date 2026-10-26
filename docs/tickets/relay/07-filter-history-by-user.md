# Ticket 07 - El historial se filtra por usuario

## Resumen

`GET /history?user=<nombre>` acota el historial de auditoría a un usuario, con el
mismo shape de siete campos que sirve sin filtro y que emite el stream. Cierra la
Fase 1: es el último ticket del plan, y el que convierte el historial en la consulta
retroactiva que el proyecto viene prometiendo desde el `feed --user` del documento
de arquitectura.

La pregunta que el ticket deja abierta —qué es "un usuario" para un evento— la
resuelve el spec: *"the events involving a given user"*, y la story 15 dice para qué
—*"reconstruct what a teammate merged or was assigned"*—. Un evento involucra a
alguien en uno de los dos roles que el glosario nombra: **actor** o **assignee**. El
filtro matchea los dos.

## Cambios Realizados

### src/relay/activity-events.service.ts

- **`list(user?)`**: con filtro, `where: [{ actor: user }, { assignee: user }]` — el
  array es el `OR` de TypeORM, y el `order: { id: 'DESC' }` sigue en el mismo `find`,
  así que filtrar no cambia el orden en que se sirve el historial.
- **La regla de "filtro ausente o vacío" vive aquí**, en un solo lugar: es el servicio
  el que decide que no hay usuario por el cual acotar. El controlador no normaliza
  nada.
- El JSDoc cita la story 15, para que el `OR` no parezca una generosidad sin motivo.

### src/relay/history.controller.ts

- **`@Query('user')`** y pass-through. Una línea: el controlador expone el parámetro
  y delega la decisión.

### test/app.e2e-spec.ts

- **El vocabulario de tests se consolidó**: `postDelivery(body, event, deliveryId?)`
  es ahora el único posteador, y `postMerge` / `postAssignment` son las dos fixtures
  con nombre sobre él. Esto **revierte** una decisión registrada en el ticket 05, que
  había dejado dos formas parecidas separadas por "ganancia marginal"; con la tercera
  variante, la regla de tres que el ticket 06 ya había aplicado en los tests de
  streaming pasa a aplicar aquí. La reversión queda registrada, no silenciosa.
- **`openStream` y `streamConnections` subieron a nivel de módulo**, porque los
  bloques 06 y 07 los necesitan igual, y se cierran en el `afterAll` de afuera.
- **`withUrl` pasó a llamarse `eventsAtUrl`**: el nombre viejo decía "URL" y la
  función devolvía una lista filtrada.
- **5 casos nuevos**, uno por criterio más el filtro vacío. Total de la suite: 48.

## Comportamiento resultante

- `GET /history?user=maria.perez` devuelve solo los eventos en los que
  `maria.perez` es el actor.
- `GET /history?user=tomas.ferreira` devuelve la asignación aunque él no la haya
  hecho: es el assignee, o sea la persona de la que el evento trata.
- Un usuario sin eventos devuelve `[]` con `200`, nunca un error ni un 404.
- `GET /history?user=` devuelve el registro completo: filtro vacío es filtro ausente.
- El evento filtrado llega con los mismos siete campos que el que emite el stream y
  que el que sirve el historial completo, para los dos tipos de evento.
- El orden sigue siendo el más nuevo primero, y filtrar no lo altera.

## Verificación

```text
vitest run (suite completa)     → Test Files 1 passed (1) | Tests 48 passed (48)
tsc --noEmit                    → exit 0
oxlint --type-aware src/ test/  → Found 0 warnings and 0 errors (17 files, 107 rules)
```

Evidencia del rojo, antes de implementar (el filtro se ignoraba):

```text
FAIL ... > returns the events of the actor asked for, newest first, and nobody else's
  AssertionError: expected [ …(19) ] to not deep equally contain ObjectContaining{…}
FAIL ... > returns an empty collection for a user the relay has never seen
  AssertionError: expected [ …(20) ] to deeply equal []
```

Un commit en `main`: `feat: filter the audit history by the user involved
(ticket 07)`.

## Acceptance Criteria

- ✅ AC1: El endpoint de historial acepta un filtro por usuario (`?user=`).
- ✅ AC2: Con eventos de dos actores distintos almacenados, filtrar por uno devuelve
  solo los de ese. Y son dos actores de verdad: `maria.perez` mergea, `lucia.gomez`
  asigna.
- ✅ AC3: La respuesta filtrada usa el mismo shape que la sin filtrar y que el
  stream. Probado por igualdad profunda en las dos direcciones y para los dos tipos
  de evento, no asumido.
- ✅ AC4: Un usuario sin coincidencias devuelve una colección vacía, no un error.

## Decisiones y desvíos

- **"Involucra" son los dos roles, y eso incluye al asignador.** La story 15 nombra
  al que mergea y al que *fue asignado*, no al que asigna; el filtro matchea también
  al actor de la asignación (`lucia.gomez`), porque el ticket dice "involving" y el
  glosario define **Actor** como "quien mergeó el PR, o **quien asignó el issue**".
  Es un ensanchamiento deliberado respecto de la letra de la story, y está fijado por
  test para que sea una decisión y no un accidente.
- **El match es exacto y sensible a mayúsculas.** GitHub trata los logins como
  case-insensitive, así que `?user=Maria.Perez` sobre un evento de `maria.perez`
  devuelve `[]` — indistinguible de "ese usuario no tiene eventos", que es justo el
  modo de falla silencioso que este repo viene cazando. Ninguna línea del spec exige
  ni prohíbe el match case-insensitive (`Normalization` solo dice que se usan los
  usernames), así que es una decisión de producto abierta: **queda sin fijar por
  test**, para no congelar una conducta que el dueño todavía no eligió. Si se quiere
  case-insensitive, es un cambio chico en la query y el test correspondiente.
- **`?user=` vacío devuelve el registro completo**, y ahora está probado. El JSDoc
  del controlador lo afirmaba sin respaldo —la misma clase de error que el ticket 06
  encontró en un comentario mío—, así que se agregó el caso que lo verifica.
- **El orden newest-first no tenía test.** Este ticket toca justamente la query que
  lleva el `order`, así que el filtrado ahora lo fija: es una propiedad que el cambio
  podría haber roto sin que nadie se enterara.
- **Se revirtió la decisión del ticket 05 sobre los helpers de test** (ver Cambios
  Realizados). La convención pide que la reversión quede registrada en el registro
  del ticket que la hace.
- **Los eventos de los fixtures siguen siendo sintéticos**, misma brecha declarada
  desde el ticket 02.

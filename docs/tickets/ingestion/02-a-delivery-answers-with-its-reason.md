# Ticket 02 - Una entrega responde con un outcome, o con su motivo

## Resumen

Una entrega de webhook produce ahora exactamente una de dos cosas: un evento
guardado, o un rechazo que dice por qué no lo hay. `undefined` dejó de cruzar la
costura del adaptador de GitHub; los predicados de tipo desaparecieron junto con la
segunda validación que los compensaba, y el parseo del cuerpo nombra su propio fallo.
El contrato de cable, las filas y el historial no se movieron.

## Cambios Realizados

### `src/relay/github/ingestion.ts` (nuevo)

El tipo `Ingestion`, unión discriminada de `{ kind: 'event', event }` y
`{ kind: 'ignored', reason }`, con `IngestionRefusal = 'not-critical' | 'malformed'`.
El comentario fija la semántica: `not-critical` es una decisión del filtro; `malformed`
es un defecto —el payload pasó un filtro y un campo necesario faltaba o era inusable—.
El módulo existe aparte para que los dos adaptadores y el controlador lo importen sin
ciclo.

### `merge-to-main.ts` e `issue-assignment.ts`

Filtro y normalización en una sola pasada, respondiendo el `Ingestion` directamente.
Desaparecen `isMergeToMain`, `isCriticalAssignment`, `toMergeToMainEvent`,
`toAssignmentEvent`, y las interfaces `PullRequestPayload`, `IssuePayload` y
`GitHubDelivery`: las tres declaraban campos que ningún guard verificaba.

En la asignación, el asignado se lee **una vez** y sirve para las dos preguntas que
antes hacían dos funciones distintas (¿hay a quién avisar? ¿a quién?). El orden de los
filtros queda explícito —acción, etiqueta crítica, asignado— y después el payload se
considera aceptado, así que de ahí en adelante lo que falte es `malformed`, no
`not-critical`.

### `to-activity-event.ts`

`toActivityEvent()` pasa a ser `ingest()`: pregunta por el merge y, solo si la
respuesta fue `not-critical`, por la asignación. Un `malformed` corta la composición,
que es la regla que impide que un merge crítico e ilegible se guarde como asignación.

### `webhook.controller.ts`

El parseo del cuerpo responde `{ kind: 'parsed' | 'unparseable' }` en lugar de
`undefined`, y el controlador ramifica por `kind` en vez de por truthiness. Un cuerpo
ilegible se acusa con `200` y se descarta sin llegar al adaptador, igual que antes,
pero ahora se distingue de una entrega que no traía evento crítico.

### `payload.ts`

Se elimina `GitHubDelivery` y su comentario se reemplaza por uno que explica por qué no
hay una interfaz de sobre compartida: nada estrecha un payload entero, así que una
interfaz de sobre solo describiría campos que ningún lector verificó.

### Documentación

`docs/relay-flow.md`: el diagrama, el paso 2 (cuerpo ilegible), los pasos 3 y 4
(`ingest()`, los motivos, la pasada única) y la tabla de archivos con `ingestion.ts`.
`ARCHITECTURE.md`: el árbol de `src/relay/github/`. `GLOSSARY.md`: dos entradas,
**Ingestion** y **Refusal**.

### `docs/adr/0015-the-adapter-names-its-refusals.md`

El registro de la decisión (ticket 01), y el lugar donde quedan escritas las
alternativas rechazadas.

## Comportamiento resultante

- Una entrega autenticada responde `200 {"received":true}` en todos los casos: guardado,
  rechazo por no crítico, rechazo por malformado y duplicado. Firma inválida sigue siendo
  `401`.
- Los mismos payloads se guardan y se descartan que antes, con los mismos valores de
  campo, incluidos `repository: null` y `branch: null` donde correspondía.
- Un duplicado sigue guardándose una sola vez y sin volver al stream.
- Los motivos del rechazo existen en el tipo y no en el cable: ningún cliente ni
  consumidor puede verlos todavía.

## Verificación

Suite del relay, sin editar un solo test:

```text
Test Files  2 passed (2)
     Tests  62 passed (62)
```

Build y lint:

```text
build exit: 0
Found 0 warnings and 0 errors.
```

Suite del cliente, sin cambios en esta mitad:

```text
ok  github.com/AG2904/github-activity-feed/client/test
```

Chequeo diferencial, misma base de datos vacía y los mismos 19 envíos firmados
—merge válido, duplicado, merge a `develop`, PR abierto, merge sin `merged`, asignación
con `bug`, con `chore`, sin asignado, sin etiquetas, merge sin `html_url`, sin `head`,
con `merged_at` ilegible, con dos etiquetas, payload desconocido, sin array de
etiquetas, cuerpo no-JSON, objeto vacío y firma inválida— contra el commit base
`94e72d2` y contra el cambio:

| | commit base | cambio |
| --- | --- | --- |
| eventos guardados | 4 | 4 |
| bytes de `/history` | 1006 | 1006 |
| SHA-256 del historial | idéntico | idéntico |
| códigos y cuerpos de las 19 respuestas | idéntico | idéntico |

## Acceptance Criteria

- ✅ AC1: una entrega produce un evento guardado o un rechazo con motivo
  (`not-critical` / `malformed`); el tipo de retorno del adaptador no tiene variante
  `undefined` y el compilador rechaza a un llamador que ignore el `kind`.
- ✅ AC2: no queda ningún predicado de tipo ni aserción de tipo en el adaptador de
  GitHub; cada adaptador recorre el payload una vez y cada campo que lee se verifica en
  la misma pasada que decide.
- ✅ AC3: lo que pasa un filtro y no se puede normalizar es `malformed`; lo que no pasa
  un filtro es `not-critical`; la composición se detiene en `malformed`.
- ✅ AC4: un cuerpo ilegible se distingue de un payload sin evento crítico, sin cambiar
  lo que ve el emisor.
- ✅ AC5: el contrato de cable no se movió —`200` en toda entrega autenticada, `401`
  para firma inválida, nada guardado en un rechazo.
- ✅ AC6: los duplicados siguen guardándose una vez y sin llegar dos veces al stream.
- ✅ AC7: la suite e2e existente pasa sin editar ningún test.
- ✅ AC8: el chequeo diferencial confirma que el historial servido es idéntico byte a
  byte al del commit base para los mismos payloads.
- ✅ AC9: build, lint y suite del relay en verde.
- ✅ AC10: sin dependencias nuevas; el tipo es la unión discriminada de TypeScript.
- ✅ AC11: el documento de flujo y el árbol de arquitectura describen los outcomes
  nombrados y los módulos que existen después del cambio.

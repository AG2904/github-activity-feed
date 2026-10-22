# El flujo del relay

Cómo se mueve un dato por el relay: qué archivo hace qué, en qué orden, y qué
invariantes sostienen el conjunto. Está escrito para alguien que abre el repo por
primera vez y quiere entender el código, no para justificar decisiones —eso último
vive en [`docs/adr/`](adr/) y en [`ARCHITECTURE.md`](../ARCHITECTURE.md)—.

Si preferís ver antes de leer, hay dos diagramas versionados:

- `.archify/architecture-relay-20261017-193000/relay.html` — la arquitectura por
  componentes.
- `.archify/architecture-file-calls-20261017-193000/file-calls.html` — cada archivo
  de `src/` como nodo y cada flecha con el nombre de la llamada que la cruza.

## 1. Los tres recorridos

El relay tiene exactamente tres caminos de entrada. Todo lo demás son hojas.

### 1.1 Una entrega de GitHub (escritura)

```
GitHub ──POST / ──▶ webhook.controller.accept()
                        │ 1. verifica HMAC sobre los bytes crudos
                        │ 2. recién ahora parsea
                        ▼
                   github/to-activity-event.ts ──▶ merge-to-main | issue-assignment
                        │                              │
                        │                              └──▶ github/payload.ts
                        │                                   domain/critical.ts
                        ▼
                   activity-events.service.store()  ──▶ activity-event.entity ──▶ SQLite
                        │ devuelve si la fila se escribió
                        ▼
                   activity-stream.deliver()        ──▶ el Subject del canal en vivo
```

Paso por paso:

1. **`src/relay/ingress.ts`** apaga los parsers de body de Nest
   (`bodyParser: false`) y deja los bytes exactos en `req.rawBody`
   (`rawBody: true`). Sin esto, un body que Nest no puede parsear se respondería
   `400` **antes** de que nadie mire la firma, y GitHub lo registraría como entrega
   fallida. `main.ts` y el harness e2e llaman al mismo `configureIngress()`, para no
   divergir.
2. **`src/relay/webhook.controller.ts`** verifica `X-Hub-Signature-256` —HMAC-SHA256
   de esos bytes, comparado en tiempo constante— y **después** parsea. Firma
   inválida: `401`. Cuerpo ilegible: `undefined`, que se acusa y se descarta.
3. **`src/relay/github/to-activity-event.ts`** es la única entrada al adaptador:
   prueba `isMergeToMain()` y luego `isCriticalAssignment()`, y devuelve
   `undefined` en los dos casos en que no hay evento —payload no crítico, o crítico
   que no se puede normalizar—. Desde afuera son indistinguibles a propósito: los
   dos se acusan con `200` y ninguno se guarda.
4. **`merge-to-main.ts` / `issue-assignment.ts`** aplican el filtro y normalizan. Acá
   es donde se leen `base.ref`, `issue.assignees` y las etiquetas, y donde el
   repositorio sale de `repository.full_name` (nunca del `url`).
5. **`github/payload.ts`** y **`domain/`** son las hojas compartidas: lectura
   defensiva de GitHub, y las reglas de qué hace crítico a un evento. Ninguna conoce
   a un controlador.
6. **`activity-events.service.ts`** escribe la fila y **devuelve un booleano**: si la
   escritura chocó con el índice único de `deliveryId`, la entrega era un reenvío y
   el booleano es `false`.
7. **`activity-stream.ts`** entrega el evento al canal en vivo **solo si se guardó**.
   Un reenvío no vuelve a emitirse.
8. La respuesta es `{"received":true}` con `200` en **todos** los casos
   autenticados: guardado, ignorado y duplicado son la misma respuesta.

### 1.2 Una consulta al historial (lectura)

```
GET /history?user=&repository= ──▶ history.controller.listAuditHistory()
                                        │ @Query('user'), @Query('repository')
                                        ▼
                                   activity-events.service.list({ user, repository })
                                        │ clausesForUser() + order: id DESC
                                        ▼
                                   activity-event.entity ──▶ SQLite
                                        │
                                        ▼
                                   fromStoredRow()  ──▶  ActivityEvent[] (JSON)
```

- **`src/relay/history.controller.ts`** no normaliza nada: pasa los dos query params
  al servicio. Las reglas viven en un solo lugar.
- **`list()`** combina los filtros con un **AND**. La trampa está en la forma que pide
  TypeORM: un *array* de cláusulas es un `OR`, y las claves *dentro* de una cláusula
  son un `AND`. Por eso un usuario —que puede estar involucrado como actor **o** como
  assignee— necesita una cláusula por rol, y el repositorio se reparte dentro de cada
  una: `[{ actor, repository }, { assignee, repository }]`.
- Un filtro ausente **o vacío** (`?user=`) pide el registro completo, no cero filas.
- **`fromStoredRow()`** es la frontera de confianza del almacenamiento: la fila
  guarda `type` y `when` como texto, y el dominio los recibe tipados. El `id` de
  storage y el `deliveryId` **nunca** salen de acá.
- El orden es `id DESC` —lo más nuevo primero— y **no** hay paginación.

### 1.3 Una conexión al canal en vivo

```
GET /stream ──▶ stream.controller.events()
                    │ merge(
                    │   stream.receive()  ──▶ { data: evento }
                    │   stream-heartbeat.heartbeats()  ──▶ { comment: 'heartbeat' }
                    │ )
                    ▼
              Nest escribe los frames SSE y sostiene la conexión
```

- **`src/relay/activity-stream.ts`** es un `Subject` de RxJS: emite a quien esté
  escuchando **ahora**. No guarda historia y no reproduce nada.
- **`src/relay/stream.controller.ts`** compone dos fuentes en una sola conexión. El
  evento viaja como el `data` del mensaje y **no** como el mensaje mismo: un
  `ActivityEvent` trae su propio `type`, y un mensaje SSE lee `type` como el
  **nombre del evento**, así que pasarlo crudo produciría `event: pr.merged` sin
  línea `data:` y el payload no llegaría nunca.
- **`src/relay/stream-heartbeat.ts`** aporta la liveness. Un stream ocioso escribe un
  frame de **comentario** cada 15 s, y como un comentario no lleva `data:` ningún
  cliente puede confundirlo con un evento. El temporizador se reinicia con cada
  evento, así que un stream que lleva eventos no escribe ninguno.
- Nest **desuscribe** el observable cuando el cliente se desconecta, así que el
  temporizador del heartbeat no queda huérfano.

## 2. El modelo

### 2.1 El `ActivityEvent` — el contrato

`src/relay/domain/activity-event.ts`. **Nueve campos, ni uno más**, y el mismo shape
sirve el stream y el historial para que un cliente parsee una sola cosa:

| Campo | Tipo | Notas |
| --- | --- | --- |
| `type` | `'pr.merged' \| 'issue.assigned'` | conjunto cerrado |
| `repository` | `string \| null` | `owner/name` |
| `branch` | `string \| null` | la rama de origen de un merge (`head.ref`); `null` si el evento no tiene una |
| `actor` | `string` | quien causó el evento |
| `subject` | `string` | título del PR o del issue |
| `assignee` | `string \| null` | `null` si el evento no tiene uno |
| `labels` | `string[]` | nombres planos, sin los objetos de GitHub |
| `url` | `string` | el link al PR o al issue |
| `when` | `IsoInstant` | UTC ISO-8601 con milisegundos, forma canónica |

Los nulos **no significan lo mismo**, y confundirlos es el error fácil:
`assignee` es `null` porque una *clase* de evento no tiene asignado; `repository` es
`null` porque una *fila* es anterior a que existiera la columna. `branch` puede ser
`null` por las dos razones a la vez: una asignación no tiene rama, y una fila anterior
a la columna tampoco; el cliente las trata igual y cae en la frase derivada del tipo
(ADR-0013).

`IsoInstant` es un tipo nominal: no se puede pasar un string cualquiera donde se
espera un instante canónico sin pasar por `isoInstant()`, que es el único productor.

### 2.2 La fila — la tabla

`src/relay/activity-event.entity.ts`, tabla `activity_events`:

| Columna | Tipo | Por qué |
| --- | --- | --- |
| `id` | entero, PK | orden del historial; no sale al cable |
| `deliveryId` | texto, **único**, nullable | la identidad de la *entrega*, no del evento; el índice único **es** el dedupe |
| `type` · `repository` · `branch` · `actor` · `subject` · `assignee` · `labels` · `url` · `when` | — | los nueve campos del contrato |

`repository` y `branch` son **nullable**, pero no por la misma razón. Para `repository`
es **solo por obligación**: SQLite no puede agregar una columna `NOT NULL` a una tabla
poblada, y TypeORM responde a un cambio de esquema recreando la tabla y copiando las
filas, así que una columna `NOT NULL` hace fallar la copia y el relay no arranca. Para
`branch` es **por obligación y por diseño**: la misma mecánica de upgrade deja sin rama a
una fila anterior a la columna, y además una asignación —o un merge cuyo payload no trae
un `head.ref` usable— no tiene rama que nombrar, y el evento se guarda igual con
`branch: null` (ADR-0013). Está medido, y la suite lo prueba levantando una app contra
una base escrita con el esquema viejo.

La tabla la crea `synchronize: true` al arrancar. **No hay migrations.**

## 3. Las reglas de dominio

`src/relay/domain/critical.ts` — dos constantes y dos predicados, sin una línea de
JSON:

- `MAIN_BRANCH = 'main'`, `CRITICAL_LABELS = ['bug', 'feedback']`.
- `isMainBranch(target)` y `carriesCriticalLabel(labels)`.

El adaptador extrae los datos del payload y le **pregunta** a estas funciones si el
resultado es crítico. Así "qué hace crítico a un evento" se responde en un solo lugar.

Las reglas completas, incluidos los casos que **no** disparan, están en la decisión
*Filtering* de [`.scratch/relay/spec.md`](../.scratch/relay/spec.md). El resumen: un
merge cuenta solo si está cerrado **y** mergeado **y** su base es `main`; una
asignación cuenta con la acción que asigna y una etiqueta crítica puesta **en ese
momento**, y una acción que solo toca un issue ya asignado no produce nada.

## 4. El cableado (declaraciones, no llamadas)

Estos archivos no aparecen en el grafo de llamadas porque no llaman a nadie: **declaran**.

- **`src/main.ts`** — crea la app con `INGRESS_OPTIONS`, llama a
  `configureIngress()`, y escucha en `PORT ?? 3000`.
- **`src/app.module.ts`** — el `DataSource` de `better-sqlite3` sobre
  `DATABASE_PATH ?? 'relay.db'`, con la entidad registrada y `synchronize: true`.
  Y un **fail-fast**: si falta `GITHUB_WEBHOOK_SECRET`, el relay se niega a arrancar.
- **`src/relay/relay.module.ts`** — declara 3 controladores y 2 providers; Nest los
  instancia por inyección.

## 5. Mapa de archivos

| Archivo | Responsabilidad | Quién lo llama |
| --- | --- | --- |
| `src/main.ts` | arranque y puerto | — |
| `src/app.module.ts` | DataSource, entidad, fail-fast del secreto | — |
| `src/relay/relay.module.ts` | cableado de Nest | — |
| `src/relay/ingress.ts` | `INGRESS_OPTIONS` + `configureIngress()` | `main.ts`, el harness e2e |
| `src/relay/webhook.controller.ts` | `POST /`: firma, parseo, store, deliver | GitHub |
| `src/relay/history.controller.ts` | `GET /history`: `?user=`, `?repository=` | clientes |
| `src/relay/stream.controller.ts` | `GET /stream`: compone eventos + heartbeat | clientes |
| `src/relay/github/to-activity-event.ts` | única entrada al adaptador | `webhook.controller` |
| `src/relay/github/merge-to-main.ts` | filtro + normalización de `pr.merged` | `to-activity-event` |
| `src/relay/github/issue-assignment.ts` | filtro + normalización de `issue.assigned` | `to-activity-event` |
| `src/relay/github/payload.ts` | lectura defensiva de payloads + `GitHubDelivery` | los dos normalizadores |
| `src/relay/domain/activity-event.ts` | el contrato de 9 campos + `isoInstant()` | adaptador, servicio, controladores |
| `src/relay/domain/critical.ts` | qué hace crítico a un evento | los dos normalizadores |
| `src/relay/activity-events.service.ts` | `store()`, `list()`, `HistoryFilter`, `fromStoredRow()` | los dos controladores de lectura y el de ingreso |
| `src/relay/activity-event.entity.ts` | la fila persistida | el servicio (TypeORM) |
| `src/relay/activity-stream.ts` | el broadcast: `deliver()` / `receive()` | `webhook.controller`, `stream.controller` |
| `src/relay/stream-heartbeat.ts` | liveness: comentario SSE cuando está ocioso | `stream.controller` |
| `test/app.e2e-spec.ts` | la suite entera, sobre la frontera HTTP | — |

## 6. Los invariantes que no se ven en el código

Son las cosas que un lector nuevo rompe sin querer. Cada una tiene su ADR.

- **El cuerpo se verifica como bytes, nunca como una re-serialización.** La firma se
  calcula sobre `req.rawBody`. Parsear y volver a serializar cambiaría los bytes y la
  firma dejaría de corresponder (ADR-0002).
- **La respuesta no distingue guardado de ignorado de duplicado.** Es `200
  {"received":true}` siempre. Un cliente no puede inferir nada de la respuesta; para
  saber qué se guardó, consulta `/history` (ADR-0005).
- **Nada se lee antes de escribir.** El dedupe es el índice único, no un `SELECT`
  previo: leer primero haría que dos entregas simultáneas no encontraran nada y las
  dos insertaran (ADR-0005).
- **El canal en vivo no guarda historia y no reproduce nada.** Un cliente que se
  reconecta tiene un hueco que el stream no puede llenar; `/history` es la respuesta.
- **Solo el evento realmente guardado llega al canal.** El `deliver()` está detrás
  del booleano de `store()`.
- **Los nulos del contrato significan cosas distintas.** Ver §2.1.
- **El heartbeat es un comentario, no un dato.** Cambiarlo a un frame `data:` haría
  que los clientes lo parsearan como un evento y rompería el contrato por la puerta
  de atrás.
- **Las lecturas no tienen autenticación, por diseño.** El canal es un broadcast:
  cada cliente recibe todo y filtra para sí (ADR-0001).
- **Un payload que no se puede normalizar se ignora, no es un error.** Es la misma
  postura que con un hook desconocido: una integración ajena encendida en GitHub no
  puede romper el relay.
- **La forma es una sola.** Si agregás un campo al `ActivityEvent`, hay aserciones de
  igualdad profunda en la suite que fallan a propósito para obligarte a decidirlo
  (ADR-0005, ADR-0010).

## 7. Qué no está en este mapa

- **El CLI en Go** (Fase 2): tiene su propio mapa en
  [`docs/cli-flow.md`](cli-flow.md).
- **CI, README y distribución de binarios** (Fase 3).
- **La elección de qué repositorios escucha el relay**: se configura del lado de
  GitHub, no acá.
- **Retención o purgado del historial**: los eventos se guardan indefinidamente.

## 8. Por dónde empezar a leer

Si tenés veinte minutos y querés entender el sistema de punta a punta, este orden
funciona:

1. `src/relay/domain/activity-event.ts` — el contrato, que es de lo que habla todo lo
   demás.
2. `src/relay/webhook.controller.ts` — el camino de escritura completo, en 86 líneas.
3. `src/relay/github/to-activity-event.ts` y después `merge-to-main.ts` — cómo un
   payload se vuelve un evento.
4. `src/relay/activity-events.service.ts` — el almacenamiento, el dedupe y los filtros.
5. `src/relay/stream.controller.ts` y `stream-heartbeat.ts` — la parte en vivo.
6. `test/app.e2e-spec.ts` — la suite es la especificación ejecutable: si algo no está
   claro arriba, acá está el comportamiento esperado, escrito como una aserción.

Para **ejecutarlo y verlo con tus ojos**, [`docs/manual-testing.md`](manual-testing.md)
tiene los pasos con los comandos y la salida esperada de cada uno.

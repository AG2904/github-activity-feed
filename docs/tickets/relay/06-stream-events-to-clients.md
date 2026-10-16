# Ticket 06 - Los eventos se emiten a los clientes conectados

## Resumen

El canal en tiempo real. `GET /stream` sostiene la conexión abierta y emite cada
evento que el relay almacena mientras haya alguien escuchando, como Server-Sent
Events sobre HTTP plano (ADR-0001): sin upgrade, sin negociación y sin nada que el
cliente deba mandar de vuelta. Cada cliente conectado recibe todos los eventos, y
filtra por su cuenta — el canal no lleva identidad ni autenticación.

Es también el ticket donde el reenvío del 05 cobra sentido hacia afuera: `store`
ahora responde si la fila se escribió, porque un duplicado no puede llegar dos
veces al stream aunque la respuesta HTTP no distinga los dos casos.

## Cambios Realizados

### src/relay/activity-stream.ts (nuevo)

- **`ActivityStream`**: el broadcast del glosario, como `@Injectable` — mismo
  patrón que `ActivityEventsService` envolviendo el `Repository`.
- **`deliver(event)` / `receive()`**: los verbos del glosario. Se evita el
  vocabulario de pub/sub, que el glosario pide no usar para nombrar el modelo.
  `receive()` devuelve el observable, no el `Subject`: los controladores no pueden
  emitir ni completar el canal.
- **No guarda historia.** Un evento almacenado mientras no había nadie conectado no
  se reproduce cuando alguien se conecta después: el historial de auditoría es lo
  que reconstruye el pasado, y esto es sólo lo que está pasando.

### src/relay/stream.controller.ts (nuevo)

- **`@Sse('stream')`**, con una sola línea de lógica:
  `this.stream.receive().pipe(map((event) => ({ data: event })))`.
- **Por qué el evento va dentro de `data` y no como mensaje**: un `ActivityEvent`
  trae su propio campo `type`, y un mensaje SSE lee `message.type` como el *nombre
  del evento*. El frame salía como `event: pr.merged` **sin línea `data`**, así que
  el payload nunca llegaba al cable. Lo destaparon los tests, no la lectura del
  código: ver Verificación.

### src/relay/activity-events.service.ts

- **`store()` responde `boolean`** (si la fila se escribió). El ticket 05 lo dejaba
  devolviendo vacío a propósito, porque no había consumidor; el consumidor llegó
  acá. El comentario del método dice ahora para qué sirve esa respuesta.

### src/relay/webhook.controller.ts

- Orquesta el caso de uso completo: verifica la firma, parsea, filtra, guarda y
  —sólo si se guardó— entrega al stream comentando por qué.

### src/relay/relay.module.ts

- Registra `StreamController` y `ActivityStream`.

### test/app.e2e-spec.ts

- **La app ahora escucha en un puerto efímero.** Un stream sostenido no es algo que
  supertest pueda observar: sus requests se responden, no se transmiten. Los demás
  casos siguen usando supertest contra el mismo servidor.
- **`openStream()` / `nextEvent()`**: un lector SSE mínimo sobre `fetch` y su
  reader, que ignora los frames sin `data` (el newline suelto que Nest escribe al
  commitear los headers y cualquier frame de comentario). Es el mismo trabajo que
  hará el cliente Go.
- **La barrera de conexión es el propio header de respuesta**, no un timer ni un
  saludo: Nest suscribe el handler del route y recién después commitea los headers
  SSE en el próximo macrotask, así que recibir bytes implica que la suscripción
  existe. Determinista por construcción.
- **`postMerge` y `storedFor` subieron a nivel de módulo**: los bloques 05 y 06 los
  necesitan igual, y era el momento de extraerlos antes de una tercera copia.
- **3 casos nuevos**, uno por criterio. Total de la suite: 43.

## Comportamiento resultante

- Un cliente que hace un `GET /stream` recibe la respuesta de inmediato y la
  conexión queda abierta; cada evento almacenado mientras está conectado llega como
  un frame `data:` con el mismo shape que sirve `/history`.
- Dos clientes conectados a la vez reciben los dos el mismo evento.
- La misma conexión sigue viva y sigue recibiendo el evento siguiente.
- Un duplicado (mismo `X-GitHub-Delivery`) no vuelve a emitirse, aunque se acuse
  igual.
- Un evento almacenado antes de que el cliente se conectara no se reproduce.

## Verificación

```text
vitest run (suite completa)     → Test Files 1 passed (1) | Tests 43 passed (43)
tsc --noEmit                    → exit 0
oxlint --type-aware src/ test/  → Found 0 warnings and 0 errors (17 files, 107 rules)
```

Evidencia del defecto del frame, tomada de un socket crudo contra la app real
(antes del fix):

```text
stream: status=200 content-type=text/event-stream
        bytes="\n: connected\n\nevent: pr.merged\nid: 0\n\n"
```

No hay línea `data:` en ninguna parte: el cliente recibía el nombre del evento y
nada más. Los tres tests de streaming fallaban por timeout esperando un frame que
nunca iba a llegar, que es exactamente cómo se encontró.

Evidencia del rojo, antes de implementar:

```text
FAIL ... > holds a plain HTTP connection open ...
FAIL ... > delivers every stored event to every connected client
FAIL ... > does not replay what was stored before a client connected
AssertionError: expected 404 to be 200
```

Un commit en `main`: `feat: stream every stored event to connected clients
(ticket 06)`.

## Acceptance Criteria

- ✅ AC1: El endpoint de stream sostiene la conexión abierta y emite eventos a
  medida que se almacenan. El test lee **dos** eventos por la misma conexión, que es
  la forma de probar que sigue abierta.
- ✅ AC2: Un evento posteado con un cliente conectado llega por esa conexión.
- ✅ AC3: Todos los clientes conectados reciben todos los eventos (dos conexiones
  abiertas, un post, los dos lo reciben).
- ✅ AC4: El stream viaja sobre HTTP plano y no necesita ningún handshake más allá
  del request mismo. Se observa como: un `GET` pelado, sin headers especiales, es
  respondido `200` con `content-type: text/event-stream` y nada se negocia ni se
  manda de vuelta. Es la forma más fuerte que admite esta seam — el criterio es una
  ausencia, y no hay un artefacto que afirmar "no hubo upgrade".
- ✅ (Extra, del spec) La user story 16 —el cliente parsea una sola cosa— queda
  probada por comparación directa: el evento que llega por el stream es **igual** al
  que sirve `/history` para el mismo almacenamiento.

## Decisiones y desvíos

- **Agregué un saludo y lo saqué, con la lección escrita.** Implementé un frame de
  comentario (`: connected`) al conectar, justificado en que Nest diferiría los
  headers SSE hasta el primer mensaje y que, sin él, ni un cliente ni un test
  podrían saber que la conexión estaba viva. Esa premisa era **falsa**: leí
  `router-response-controller.js` hasta la línea 139 e inferí el resto. El archivo,
  veinte líneas más abajo, hace `stream.commitHeaders()` en un `setTimeout(…, 0)`
  inmediatamente después de suscribir, con el comentario explícito de que los
  headers "se mandan enseguida en vez de esperar la primera emisión del
  Observable". El saludo era innecesario y su justificación estaba mal; lo sacó la
  revisión de dos ejes, no yo. Se quitó y los tests pasan igual, que es la prueba.
  **El aprendizaje**: no afirmar en un comentario una causa que no se leyó hasta el
  final del camino relevante.
- **Sin heartbeat.** Nada en el spec lo pide y `EventSource` reconecta solo. Si un
  proxy corporativo empieza a cortar conexiones ociosas, un frame de comentario
  periódico es el próximo paso obvio; queda anotado como límite conocido, no como
  olvido.
- **Sin reproducción del backlog, y sin `Last-Event-ID`.** El canal es en vivo. Un
  cliente que se reconecta después de una caída pierde lo que pasó mientras estaba
  afuera, y lo recupera consultando `/history`. El spec dice "emite cada evento
  almacenado a todos ellos", que se puede leer como reproducir el backlog; el
  criterio del ticket —"emite eventos a medida que se almacenan"— resuelve la
  ambigüedad hacia lo vivo. Queda fijado por test para que la decisión no se
  descubra por sorpresa.
- **"client" vs "CLI" del glosario.** El glosario define **CLI** y pide evitar
  "client"; los criterios de este ticket, las user stories del spec y el propio
  ADR-0001 dicen "client" todo el tiempo. Mantuve "client" en los nombres de test
  porque espejan la palabra del criterio que prueban, y dejo la tensión anotada: es
  una decisión de domain modeling, no de este ticket.
- **La user story 13 (reconexión sin intervención) no agrega código propio.** El
  relay acepta una conexión nueva sin estado que lo impida (cada test abre una), y
  el reintento es de `EventSource`. No se agrega un test de reconexión explícito
  porque no hay comportamiento del relay que probar más allá de eso.
- **Los fixtures siguen siendo sintéticos** y ahora también los eventos que se
  emiten; misma brecha declarada en los tickets 02 a 05.

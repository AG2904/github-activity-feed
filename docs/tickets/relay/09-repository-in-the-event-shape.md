# Ticket 09 - El repositorio se vuelve un campo del evento

## Resumen

El `ActivityEvent` pasó de siete campos a ocho: `repository`, como `owner/name`, leído
de `repository.full_name` del payload y guardado en su propia columna. `GET /history`
ahora acepta un filtro por repositorio, solo o combinado con el de usuario.

El ticket no viene del plan original de la Fase 1: lo abrió la sesión de grilling que
definió el CLI, que había especificado derivar el repositorio del `url` del evento
(ADR-0006) y dejó anotado que eso convertía un link en portador de datos. Es el
segundo de los dos tickets que ese grilling dejó abiertos —el otro, el heartbeat, es
el 08—.

**Es el único ticket del proyecto que reescribe una decisión anterior en vez de
agregar una nueva**, y por eso tiene un ADR propio: el **ADR-0010** supersede al
**ADR-0006**. El 0006 no se borró ni se reescribió: se le puso un puntero arriba y se
dejó su razonamiento intacto, porque es el argumento que alguien va a repetir si
alguna vez se propone sacar el campo.

## Cambios Realizados

### `src/relay/domain/activity-event.ts`

- El `ActivityEvent` gana `repository: string | null`, después de `type`.
- El encabezado del archivo decía "Seven fields, no more" y ahora dice ocho, con
  `where` sumado a la enumeración de qué describe un evento.
- El comentario del campo explica por qué es nullable y por qué eso **no** es lo mismo
  que `assignee`: uno es nulo porque una *clase* de evento no tiene repositorio,
  el otro porque una *fila* es anterior a la columna.

### `src/relay/github/payload.ts`

- **`repositoryName(repository)`** nuevo, al lado de `loginOf` y `labelNames`: lee
  `full_name` de forma defensiva y devuelve `null` si no hay un valor usable. Es el
  único lugar donde se conoce la forma del objeto repositorio de GitHub.
- **`GitHubDelivery`** nuevo, y salió de la revisión de dos ejes: `action`, `sender` y
  `repository` son los tres campos que **todo** webhook lleva, y los dos payloads los
  declaraban por separado. Ahora `PullRequestPayload` e `IssuePayload` extienden ese
  tipo. Es la forma sana del *Data Clumps* que la revisión marcó: los valores que viajan
  juntos piden un tipo propio, y el clump acababa de pasar de dos campos a tres.

### `src/relay/github/merge-to-main.ts` y `issue-assignment.ts`

Los dos normalizadores leen `payload.repository` y lo ponen en el evento. Los dos
`PullRequestPayload`/`IssuePayload` declaran el campo, como ya declaraban `sender`.

### `src/relay/activity-event.entity.ts`

`@Column({ type: 'text', nullable: true })`. **Nullable por obligación, no por
elección**: ver *Decisiones y desvíos*.

### `src/relay/activity-events.service.ts`

- `fromStoredRow` proyecta la columna, con las otras dos aserciones que son la
  frontera de confianza del almacenamiento.
- **`HistoryFilter`** nuevo: el filtro pasó de un `user?` posicional a un objeto, para
  que el call site nombre lo que pide.
- **`clausesForUser(user, repository)`**: un usuario está *involucrado* como actor o
  como assignee, así que necesita una cláusula por rol; el repositorio entra en cada
  una. Es la diferencia entre un AND y un OR, y es la parte del ticket más fácil de
  escribir mal.

### `src/relay/history.controller.ts`

`@Query('repository')` al lado de `@Query('user')`, y las dos reglas siguen viviendo
en el servicio.

### `test/app.e2e-spec.ts`

- Los dos constructores de fixtures ganan `repository` (con `REPOSITORY` como valor por
  defecto), así que cada entrega realista de la suite dejó de ser una captura recortada
  y lleva el objeto que GitHub realmente manda. Los dos payloads inline que imitan
  entregas también lo llevan. Los que quedaron sin él son los que **deliberadamente** no
  son entregas —el `zen` de un ping, un push, un `issue: null`, un string donde debería
  ir un objeto, y los cuerpos de las pruebas de firma— donde la minimalidad es
  justamente lo que prueban.
- Tres aserciones de igualdad profunda pasaron de siete campos a ocho.
- **5 casos nuevos**: el repositorio que llega al cable; el filtro por repositorio; el
  filtro por repositorio *y* usuario combinados; el repositorio desconocido; y el
  arranque contra una base con el esquema viejo.
- El caso de la migración es el único que levanta su propia app, porque lo que prueba
  es qué le pasa a una base **que ya tiene filas** cuando el esquema se mueve debajo.

### `docs/adr/0010-repository-as-a-field-of-the-event.md` (nuevo)

Registra la decisión, el costo que el ADR-0006 había anticipado, y las dos
consecuencias que importan: por qué la columna es nullable, y por qué `repository` y
`assignee` son nulos por razones distintas.

### Documentos vivos movidos juntos (AC4)

`.scratch/relay/spec.md` (story 10, story 25 nueva, Normalization, History),
`ARCHITECTURE.md` (§6: el ejemplo y "ocho campos"), `docs/adr/0005` ("exactly eight
fields"), `docs/adr/0006` (puntero al 0010), `docs/adr/0009` ("the eight-field shape"),
`docs/manual-testing.md` (los dos bodies, el oráculo de `/history`, el frame del stream
y "ocho campos"), `.scratch/cli/spec.md` (Filtering, Out of Scope, Normalization,
Fixtures, y un comentario nuevo) y `.scratch/cli/issues/03` y `05` (los dos ACs que
nombraban los siete campos, o mandaban derivar el repositorio del `url`).

**La revisión de dos ejes encontró cuatro documentos que mi propio grep no cazó**,
porque yo buscaba `seven fields` y los textos decían `seven-field`, `seven keys` y
`the seven fields`. Es exactamente el modo de falla que AC4 nombra al pedir que todo
se mueva "en vez de separarse": un invariante no se retira en los lugares donde uno se
acordó de buscarlo, y por eso el criterio habla de los documentos y no de una lista.

## Comportamiento resultante

Un evento guardado con la columna llega al cable así:

```json
{"type":"pr.merged","repository":"AG2904/github-activity-feed","actor":"maria.perez","subject":"…","assignee":null,"labels":["bug"],"url":"…","when":"…"}
```

Un evento guardado **antes** de que la columna existiera llega con
`"repository": null`, y eso es deliberado: recuperarlo del `url` es exactamente la
derivación que este ticket vino a sacar, y sólo funcionaría para URLs de la forma que
este relay produce.

El filtro por repositorio es una igualdad exacta sobre `owner/name`. Combinado con el
de usuario es un **AND**: `?repository=X&user=Y` devuelve los eventos de X en los que Y
es actor o assignee, no la unión.

## Verificación

**Fase roja primero.** Con los casos escritos y sin implementación: `7 failed | 47
passed (54)`. Y el caso de la migración ya arrancaba y servía la fila legacy — lo único
que fallaba era la clave `repository` faltante —, lo que probaba el diseño nullable
antes de escribir una línea de implementación.

**El experimento que decidió el diseño.** Antes de elegir, medí las dos alternativas
contra una base real con una fila:

- **Columna `NOT NULL`**: el relay **no arranca**. TypeORM no hace `ALTER TABLE ADD
  COLUMN` en SQLite: recrea la tabla y copia las filas, y la copia falla con
  `NOT NULL constraint failed: temporary_activity_events.repository`. El módulo
  reintenta para siempre (`Unable to connect to the database. Retrying (1)…`). La
  tabla original sobrevive por el rollback, pero el servicio queda inservible.
- **Columna nullable**: arranca, la fila legacy se sirve, y el esquema pasa a
  `"repository" text`.

**Después de implementar:**

```
npx tsc --noEmit -p tsconfig.json     → exit 0
npx vitest run --config ./vitest.config.e2e.ts
  Test Files  1 passed (1)
       Tests  54 passed (54)
npm run lint
  Found 0 warnings and 0 errors. (17 files, 107 rules)
```

Dos fallas intermedias fueron **bugs de mis propios tests**, no del código, y las dos
enseñaron algo: una aserción vieja del ticket 03 que mi grep no había encontrado
porque su `assignee` no era `null`, y una premisa falsa en el caso del AND —afirmaba
que maria.perez no había mergeado en cierto repositorio, cuando mis propios fixtures la
habían hecho mergear ahí dos veces—. El filtro estaba bien; la expectativa estaba mal.

## Acceptance Criteria

- ✅ **AC1: el shape lleva el repositorio como `owner/name`, leído de
  `repository.full_name` del payload.** Cubierto por las tres aserciones de igualdad
  profunda y por un caso dedicado que usa un payload cuyo `url` y cuyo `repository`
  **no coinciden** — una forma que GitHub no manda, y que es la única que distingue un
  relay que lee `repository.full_name` de uno que parsea el `url`. El caso lo dice en
  su comentario, para que nadie lo "arregle" creyendo que es un fixture descuidado.
- ✅ **AC2: la fila gana una columna, y un historial existente sobrevive.** Probado por
  un caso que crea una base con el esquema anterior **congelado como literal** (una
  migración se prueba contra la forma de la que viene, y ninguna declaración actual
  puede producirla), le inserta una fila, levanta la app contra esa base y afirma que
  `/history` la sirve. El `NOT NULL` que falla está medido y transcripto arriba.
- ✅ **AC3: `GET /history` acepta un filtro por repositorio.** Tres casos: el filtro
  aísla un repositorio del otro; el combinado con usuario es un AND —probado con un
  tercer repositorio que tiene eventos propios y ningún evento de maria.perez, donde un
  OR devolvería sus merges y el AND devuelve nada—; y un repositorio desconocido
  devuelve `[]` con `200`.
- ✅ **AC4: el invariante de siete campos se retira deliberadamente, y todo se mueve
  junto.** Movidos: el spec (story 10 + Normalization), el documento de arquitectura
  (§6), ADR-0005, ADR-0006 (puntero), ADR-0009, el runbook manual, el spec del CLI y dos
  de sus tickets abiertos, y las tres aserciones de igualdad profunda de la suite.
  **El glosario no necesitó cambios**, y vale decirlo en vez de dejarlo ambiguo: nunca
  enumeró los campos, y su entrada **Repository** describe el concepto sin nombrar ni su
  forma ni su origen, así que no había nada que corregir. Decir que "ya decía exactamente
  lo mismo" habría sido exagerar: la entrada no dice `owner/name` ni de dónde sale.
- ✅ **AC5: los fixtures dejan de ser payloads recortados.** Los dos constructores
  emiten `repository.full_name`, así que todos los payloads de la suite lo llevan.

## Decisiones y desvíos

- **La columna es nullable, y eso lo decidió un experimento.** No es una preferencia:
  SQLite no puede agregar una columna `NOT NULL` a una tabla poblada, y el
  comportamiento de TypeORM es recrear la tabla y copiar. La primera versión del
  diseño —`NOT NULL`, que es lo que el dominio pide— dejaba el relay sin arrancar. El
  costo honesto es que una fila vieja responde `null` en vez de un valor inventado.
- **No hay backfill, y la razón es que no podría ser total.** La alternativa a
  `null` era rellenar las filas viejas derivando el repositorio de su `url` al
  arrancar. Se descartó porque la derivación **no es total** —una `url` que no tenga la
  forma que este relay produce no da nada—, así que el campo tendría que ser nullable
  igual, y el backfill sólo agregaría maquinaria sin quitar el caso nulo.
- **Desvío: limpié la deuda de URLs de los fixtures.** AC5 obliga a que los fixtures
  sean coherentes, y un fixture cuyo `url` dice `Gitlab-notifier` y cuyo `repository`
  dice `github-activity-feed` sería internamente incoherente —un payload real tiene la
  url *dentro* del repositorio—. Eran 24 ocurrencias en la suite y 5 en el runbook, más
  el ejemplo del §6. Era la deuda que había quedado anotada en el rename, y agregar el
  campo la volvía insostenible. **No** se tocaron las referencias históricas a GitLab
  en ADR-0002, el research doc ni los registros de tickets.
- **El filtro pasó de posicional a objeto.** `list(user?, repository?)` con dos strings
  opcionales es un call site que se lee mal y se rompe callado al agregar un tercero;
  `list({ user, repository })` nombra lo que pide. El spec del relay lo llama *Data
  Clumps* en su forma sana: dos valores que viajan juntos piden un tipo.
- **Los registros de evidencia (`docs/tickets/**`) y los comentarios fechados de tickets
  anteriores no se tocaron**, aunque varios digan "siete campos": son evidencia de lo que
  esos tickets construyeron y decidieron, y era cierto cuando se escribieron. **Sí se
  tocaron** los tickets del CLI que planifican trabajo que todavía no existe
  —`.scratch/cli/issues/03` y `05`— porque un AC abierto que pide "siete claves", o que
  manda derivar el repositorio del `url`, es un plan equivocado y no un registro. Este
  documento es el que dice que el invariante se retiró, y el ADR-0010 el que dice por qué.
- **Lo que el ticket no pide y no se hizo.** No se agregó un filtro por repositorio al
  stream (el canal es un broadcast y cada cliente filtra para sí, ADR-0001), ni un
  endpoint para listar los repositorios conocidos, ni una columna `repository` en
  ningún índice —el historial es un `find` sin paginar y con `order by id`, y agregar
  un índice para una tabla de este tamaño sería adivinar dónde duele.

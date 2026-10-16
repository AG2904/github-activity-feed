# Ticket 05 - Un evento reenviado se almacena una sola vez

## Resumen

La entrega de webhooks de GitHub es **at-least-once**: una entrega que no recibe
respuesta dentro del presupuesto de 10 segundos se reintenta, y el historial de
entregas permite reenviarla a mano. El identificador `X-GitHub-Delivery` viaja en
un header y **no cambia entre reenvíos**, así que pasa a ser la clave de
idempotencia: se guarda en la fila del evento bajo un índice único y el segundo
intento choca contra ese índice, el evento se descarta, y la respuesta es
indistinguible de la de un almacenamiento nuevo.

El identificador es una columna de la fila y nunca un campo del `ActivityEvent`:
el evento describe un *hecho*, el identificador identifica una *entrega* (ADR-0005).

## Cambios Realizados

### src/relay/activity-event.entity.ts

- Columna **`deliveryId`** (`text`, `nullable: true`, `unique: true`), ubicada junto
  al `id` y no entre los siete campos del dominio. La posición es el argumento: lo
  que está arriba del evento es identidad de la fila (el `id` de storage y la
  entrega que la escribió), y lo que está abajo es el hecho.
- **Nullable a propósito.** GitHub estampa todas sus entregas, así que una entrega
  sin identificador es un caso de borde; pero si igual llega un evento crítico con
  firma válida, es un hecho y se guarda. SQLite admite varios `NULL` en un índice
  único, así que esa vía no colisiona consigo misma. El costo queda dicho: sin
  identificador no hay dedupe.

### src/relay/activity-events.service.ts

- **`store(event, deliveryId)`**: un argumento de dominio y uno de infraestructura,
  que es exactamente la asimetría que ADR-0005 declara como su costo honesto.
- **`isDuplicateDelivery(error)`**: reconoce el rechazo por el **código del driver**
  (`SQLITE_CONSTRAINT_UNIQUE` sobre `driverError`, que TypeORM copia en el
  `QueryFailedError`), nunca por el texto del mensaje. Importa la precisión en las
  dos direcciones: un rechazo por otra razón se propaga en vez de confundirse con
  un reenvío, y el identificador de entrega es la única columna única de la tabla.
- **Sin lectura antes de la escritura.** Un `SELECT` previo al `INSERT` haría que
  dos entregas que llegan juntas no encuentren nada y las dos inserten: justo el
  caso que el constraint existe para resolver.

### src/relay/webhook.controller.ts

- Lee `X-GitHub-Delivery` y lo pasa junto al evento. Header ausente o vacío →
  `null`, porque las dos cosas significan "la entrega llegó sin identificar".

### test/app.e2e-spec.ts

- **Seis casos nuevos**, uno por cada criterio, sobre la misma seam HTTP. Total de
  la suite: 40.
- `postDelivery(body, deliveryId)`: postea como GitHub, con el hook y la entrega.
- Cada fixture nombra su propio identificador: la suite comparte una sola base, y
  un identificador compartido entre tests hace que un test dependa de otro (pasó, y
  el rojo lo delató con dos `500` en el caso concurrente).
- `pullUrl` e `issueUrl` subieron a nivel de módulo: el bloque 04 y el 05 los
  necesitan igual.

## Comportamiento resultante

- La misma entrega, posteada dos veces, deja una fila y las dos respuestas son
  `200 { received: true }`.
- Dos entregas distintas, una atrás de la otra, sobreviven las dos.
- Una entrega cuyo identificador ya está almacenado se descarta aunque su payload
  sea distinto: el identificador nombra una entrega, no un hecho.
- El `ActivityEvent` servido por `/history` sigue teniendo siete campos: la columna
  nueva no llega al cable.
- Un evento sin identificador se guarda igual, y dos llegadas sin identificador son
  dos filas: no hay nada con lo que deduplicarlas.

## Verificación

```text
vitest run (suite completa)     → Test Files 1 passed (1) | Tests 40 passed (40)
tsc --noEmit                    → exit 0
oxlint --type-aware src/ test/  → Found 0 warnings and 0 errors (15 files, 107 rules)
```

Evidencia del rojo, antes del `catch` — el constraint existe y se vio disparar:

```text
QueryFailedError: SqliteError: UNIQUE constraint failed: activity_events.deliveryId
  driverError: SqliteError { code: 'SQLITE_CONSTRAINT_UNIQUE' }

expected 200 "OK", got 500 "Internal Server Error"
expected [ 500, 500 ] to deeply equal [ 200, 200 ]
```

Un commit en `main`: `feat: store a delivery once, however many times it arrives
(ticket 05)`.

## Acceptance Criteria

- ✅ AC1: El identificador de entrega (`X-GitHub-Delivery`) se guarda en la fila del
  evento bajo un índice único. La columna se creó con `unique: true` y el índice se
  vio rechazar una inserción (evidencia del rojo, arriba).
- ✅ AC2: El identificador es una columna de storage y nunca un campo del
  `ActivityEvent`. El test compara el evento servido con **igualdad profunda** sobre
  los siete campos, así que un octavo campo hace fallar el test.
- ✅ AC3: Postear la misma entrega dos veces almacena una sola fila.
- ✅ AC4: El duplicado se acusa con un código de éxito, y con exactamente la misma
  respuesta que un almacenamiento nuevo.
- ✅ AC5: Dos eventos distintos posteados uno atrás del otro sobreviven los dos.
- ⚠️ AC6: El duplicado lo rechaza el constraint y nunca una lectura previa. El
  mecanismo es así en el código (`unique: true` y ningún `SELECT` antes del
  `INSERT`), pero **el test no lo prueba** y ningún test en esta seam puede
  probarlo: ver Decisiones.

## Decisiones y desvíos

- **El criterio 6 no es verificable en la seam, y el test no lo afirma.** El test de
  concurrencia (`Promise.all`) sostiene el contrato observable —las dos entregas se
  acusan, queda una fila— pero **no distingue** un insert rechazado por el
  constraint de una lectura seguida de una escritura: el driver de better-sqlite3 es
  síncrono, así que las dos escrituras se serializan en el event loop y en ningún
  momento hay dos lecturas que no vean nada. Una implementación lectora-primero
  pasaría ese test igual. Y no se puede hacer mejor en esta seam: las decisiones de
  testing del spec prohíben tocar el ORM directamente, que es donde el mecanismo
  viviría. Lo que sí se hizo: (1) un caso donde el identificador se repite y el
  payload **difiere** —eso prueba que la clave guardada es el valor del header y no
  el contenido del evento, y que el descarte no depende de releer campos—; (2) el
  comentario del test dice explícitamente qué puede y qué no puede distinguir; (3) el
  código no tiene ninguna lectura previa, que es la garantía estructural. Queda como
  el límite conocido de este ticket, no como una deuda escondida.
- **Columna nullable, y su costo asumido.** Alternativa considerada: hacer la
  columna obligatoria y descartar cualquier entrega sin identificador, que haría el
  dedupe total. Se descartó con el mismo argumento que ADR-0005 usa para rechazar el
  hash del evento: **perder un evento real es peor que guardar un duplicado**. El
  test que fija el comportamiento también documenta el costo (dos llegadas sin
  identificador son dos filas), para que nadie lo descubra por sorpresa.
- **`GLOSSARY.md` no cambió.** El identificador de entrega no es lenguaje de dominio
  —ADR-0005 lo deja deliberadamente fuera del shape y el glosario describe el
  contrato compartido con la CLI, que nunca lo ve—. Su lugar es ADR-0005 y el
  comentario de la columna.
- **Duplicación aceptada entre helpers de test.** `postDelivery` (ticket 05) y
  `postFixture` (ticket 04) tienen la misma forma de "postear como GitHub". Se
  dejaron separados porque el 04 postea fixtures con nombre y el 05 postea cuerpos
  con identificador; unificarlos tocaría los dos bloques por una ganancia marginal.
- **Los fixtures siguen siendo sintéticos**, y ahora también los identificadores de
  entrega (los reales son GUID). Misma brecha declarada en los tickets 02 a 04: falta
  una entrega real de GitHub que capturar, y con ella un identificador real.

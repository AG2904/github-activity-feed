# Ticket 04 - Todo lo demás se ignora

## Resumen

El barrido negativo del filtro: todo webhook que no sea un merge a `main` o una
asignación con label `bug`/`feedback` se acusa con éxito y se descarta, sin tocar
el historial. Es la mitad donde un filtro se rompe en silencio — nada falla, sólo
entra basura — así que se prueba como un set: cada fixture verifica el historial
completo contra su estado previo, y un caso final postea el set entero y confirma
que no entró ninguna fila y que ninguna se reemplazó.

El ticket destapó además un defecto que su enunciado no mencionaba: el parser JSON
del framework respondía `400` **antes** de que se verificara la firma, así que un
webhook que el relay debe acusar y descartar se le reportaba a GitHub como entrega
fallida. Se cerró dentro de este ticket, por decisión explícita: un `2xx` tiene que
seguir significando "firma verificada".

## Cambios Realizados

### src/relay/ingress.ts (nuevo)

- **`INGRESS_OPTIONS`**: `rawBody: true` (los bytes exactos en `req.rawBody`) junto
  con `bodyParser: false`, que da de baja los parsers JSON y urlencoded de Nest.
  Esos parsers leen el cuerpo antes de que el controlador pueda verificar nada: con
  `json`, un body ilegible devuelve `400`, y su modo `strict` rechaza incluso un
  escalar JSON perfectamente válido.
- **`configureIngress(app)`**: registra el único parser que el ingress necesita —
  bytes crudos, `type: '*/*'`, para cualquier content type.
- Los dos los comparten `src/main.ts` y el harness e2e, para que producción y tests
  no puedan divergir en lo único que este wiring existe para hacer.

### src/main.ts

- Crea la app con `INGRESS_OPTIONS` y llama a `configureIngress`. Sin esto, el
  defecto no se corrige en producción por más que los tests pasen.

### src/relay/webhook.controller.ts

- La lógica no cambia: el controlador ya leía `req.rawBody`, verificaba la firma y
  recién después parseaba. Lo que cambia es que ahora ese orden es **real**: antes
  el parser corría primero y el `try/catch` de `parseBody` era inalcanzable para
  `Content-Type: application/json`. Se documenta el orden en el comentario del
  endpoint, que es donde un futuro editor podría romperlo.

### docs/adr/0002-webhook-auth-via-hmac-signature.md

- Párrafo nuevo: la verificación ocurre antes del parseo, y por qué el wiring es
  este. Se agrega la alternativa considerada (dejar el parser de Nest y capturar
  `entity.parse.failed` aguas abajo, descartada: deja el modo de falla en manos del
  framework, obliga a keyear sobre la forma del error de express, y hace que la
  respuesta dependa de qué parser corrió) y la consecuencia (`req.body` es un
  `Buffer` en este endpoint).

### test/app.e2e-spec.ts

- **`NON_CRITICAL_WEBHOOKS`**: 11 fixtures en una tabla, cada una declarando el hook
  que GitHub nombraría y enviándolo en `X-GitHub-Event`, para que sean entregas
  reales y quede ejercitado el header que el relay ignora. Cada caso compara el
  historial completo (`toEqual`) contra su estado previo, y cuando el payload tiene
  URL, verifica además que esa URL no esté.
- **El barrido**: postea el set entero y verifica cantidad **y** contenido.
- **El body ilegible, clavado de los dos lados**: `200` y nada almacenado con firma
  válida; `401` sin firma y `401` con la firma de otro secreto. Sin esos dos, "la
  firma va antes del parseo" sería una afirmación, no una prueba.
- **`pullRequestPayload`** ahora eleva `action` al nivel superior desde sus
  overrides. Antes los esparcía dentro de `pull_request`, así que un `action`
  overrideado se ignoraba en silencio (ver Decisiones).
- Total de la suite: 34 casos (eran 20).

## Comportamiento resultante

- Un merge a otra rama, un PR abierto/cerrado/reabierto/editado, una asignación sin
  label crítica, un `ping`, un push a `main`, un payload con forma inservible, uno
  que no es objeto y uno que no se puede leer: todos se acusan con `200 { received:
  true }` y ninguno deja fila.
- Editar un PR ya mergeado (acción `edited`, estado de merge intacto) no vuelve a
  almacenar el merge: la acción es la mitad del filtro, no un detalle.
- Un body que el relay no puede leer ya no es una entrega fallida: con firma válida
  se acusa y se descarta. Sin firma, o con una que no valida, sigue siendo `401` y
  el cuerpo nunca se parsea.
- El historial queda idéntico tras el set negativo completo, que es la afirmación
  que el ticket pone por escrito en su último criterio.

## Verificación

```text
vitest run (suite completa)     → Test Files 1 passed (1) | Tests 34 passed (34)
tsc --noEmit                    → exit 0
oxlint --type-aware src/ test/  → Found 0 warnings and 0 errors (15 files, 107 rules)
```

El commit del fix queda verde por sí solo: con los tests previos (20) el suite pasa
igual, así que la historia es bisecable y cada commit es una unidad revisable.

Evidencia del rojo, antes del fix — los dos caminos del parser, medidos contra la
app real:

```text
body '{ "action": "closed", "pull_request": { "merged": true,   → 400
  {"message":"Expected property name or '}' in JSON at position 2 (line 1 column 3)",
   "error":"Bad Request","statusCode":400}

body '"a string where an object should be"'  (JSON válido)      → 400
  expected 401 "Unauthorized", got 400 "Bad Request"
```

Tres commits, hechos en la rama `feat/04-ignore-everything-else`:
`fix: verify the webhook signature before parsing the body`,
`test: prove every non-critical webhook is acknowledged and discarded (ticket 04)`
y `docs: add the ticket 04 record and refresh the repo tree`. Los tres se mergearon
en `main` por fast-forward y se pushearon a `origin/main` sin PR — decisión de
Andrés para un repo personal, donde el feature-branching no aporta. La rama local
quedó apuntando al mismo commit.

## Acceptance Criteria

- ✅ AC1: Un PR mergeado a una rama distinta de `main` no almacena nada.
- ✅ AC2: Un PR abierto, cerrado, reabierto o editado no almacena nada (el de
  `edited` con el estado de merge intacto es el filoso: el estado solo lo guardaría).
- ✅ AC3: Un issue asignado sin label `bug` o `feedback` no almacena nada.
- ✅ AC4: Un hook no reconocido no almacena nada (el `ping` de configuración y un
  push a `main`, que no es un merge).
- ✅ AC5: Un payload malformado se maneja sin error y no almacena nada. Se cubre en
  las dos lecturas de "malformado": JSON válido con forma inservible, y un body que
  no se puede leer. El segundo exigió el fix del ingress.
- ✅ AC6: Todos los anteriores se acusan con un código de éxito.
- ✅ AC7: El endpoint de historial no reporta filas nuevas después de postear el set
  negativo completo.

## Decisiones y desvíos

- **El body ilegible se cerró acá, no se documentó como desviación.** La decisión
  fue de Andrés (2026-10-16). El contrato elegido: un `2xx` sólo con firma
  verificada, y un body firmado que no se puede leer se acusa y se descarta. La
  alternativa era dejarlo en `400` y registrar la brecha; se descartó porque el
  registro del ticket 02 ya afirmaba "JSON malformado se ignora" — o sea que la
  intención estaba escrita y el cable decía otra cosa.
- **`INGRESS_OPTIONS` y `configureIngress` no se pueden unificar.** Son dos mitades
  de un mismo wiring y aplicarlas por separado rompe el ingress, pero la API de Nest
  separa las opciones de creación de la instancia de la app. Queda como el costo
  aceptado, mitigado porque la constante y la función viajan juntas y las usan
  producción y tests.
- **Se aceptó duplicar fixtures con los tickets 02 y 03.** Cuatro escenarios ya
  tienen su test propio en esos bloques. El barrido negativo completo es, además,
  un criterio del ticket (AC7), y el registro del ticket 03 dice explícitamente que
  el 04 conserva el set entero. La duplicación es el precio de que el set sea
  completo y legible de una sola pasada.
- **Las fixtures siguen siendo sintéticas.** Misma brecha declarada en los tickets
  02 y 03: el spec pide payloads capturados de GitHub y todavía no hay una entrega
  real que capturar. Lo que sí se agregó ahora es el header `X-GitHub-Event`, para
  que la entrega sea fiel a lo que GitHub manda.
- **Un fixture mal escrito se delató solo.** El primer fixture de `edited` se
  almacenó: el helper esparcía los overrides dentro de `pull_request` y
  `action: 'edited'` nunca llegaba al nivel superior, así que la top-level quedaba
  en `closed` y el relay hacía lo correcto con un fixture que probaba lo contrario
  de lo que decía. Lo destapó la aserción sobre las filas, no la del status.
- **`ARCHITECTURE.md` (entonces `gitlab_feed_architecture.md`): §5 en el pase de docs, §7 y §9 en uno posterior.**
  El plan de fases daba la idempotencia como "tickets 04–05" (es sólo el 05) y no
  marcaba cerrado el ticket 03, así que se sincronizó en un commit aparte
  (`docs: sync the phase plan with what has landed`) en vez de mezclar la evidencia
  del ticket con la planificación del repo.

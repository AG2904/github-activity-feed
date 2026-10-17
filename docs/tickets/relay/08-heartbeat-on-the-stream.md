# Ticket 08 - El stream avisa que está vivo

## Resumen

`GET /stream` ahora escribe un frame de comentario cada quince segundos cuando la
conexión no tiene nada que decir. Cierra el límite conocido que el registro del
ticket 06 había dejado anotado —*"un frame de comentario periódico es el próximo
paso obvio; queda anotado como límite conocido, no como olvido"*— y con él la
premisa sobre la que la Fase 2 había argumentado la regla de liveness del cliente.

El ticket no viene del plan original de la Fase 1: lo abrió la sesión de grilling
que definió el CLI, al encontrar que un cliente no puede distinguir un relay
callado de uno muerto. Se dejó como ticket de relay en lugar de hacer depender la
Fase 2 de un cambio de la Fase 1, y es uno de los dos que ese grilling dejó
abiertos —el otro es el `repository` como campo del shape—.

## Cambios Realizados

### `src/relay/stream-heartbeat.ts` (nuevo)

- **`heartbeats(activity)`**: compone la mitad de liveness del stream. Un
  `merge(of(null), activity)` con `switchMap` sobre un `timer` periódico: cada
  evento **reinicia** el temporizador, así que un stream que lleva eventos no
  escribe ningún heartbeat y uno que se quedó callado escribe uno por intervalo.
  El `of(null)` arranca el primer temporizador, porque un cliente que se conecta a
  un relay sin nada que decir no tiene ningún evento que lo reinicie.
- **`heartbeatMs()`**: resuelve el intervalo desde `STREAM_HEARTBEAT_MS`, con el
  default de 15 s. Un valor ausente, vacío o no positivo se trata como no puesto.
- El frame viaja como `comment` y no como `data`, que es la decisión de la que
  depende todo lo demás (ver *Decisiones y desvíos*).

### `src/relay/stream.controller.ts`

El handler quedó en composición pura: el `merge` del evento como `data` con
`heartbeats(events)`. La política del heartbeat (el intervalo, la resolución del
entorno, el texto del comentario) se fue al módulo nuevo, para que el controlador
siga cambiando por una sola razón —la forma del frame— y no además por la
configuración del despliegue.

### `test/app.e2e-spec.ts`

- **`openFrameStream()`** nuevo: lee el stream frame a frame, comentarios
  incluidos. `openStream()` —que devuelve eventos ya parseados— ahora se construye
  encima, así que el único lector que sabe de bytes es uno solo.
- **Caso nuevo** (ticket 08): sobre una conexión abierta y sin postear nada,
  afirma que llega un frame de comentario, que no lleva `data:`, y que **llega un
  segundo** — porque el heartbeat es periódico y no un saludo único.
- `beforeAll` fija `STREAM_HEARTBEAT_MS=100`, para que un caso pueda observar un
  heartbeat sin esperar quince segundos.

### `docs/adr/0007-hand-rolled-sse-parser.md`

El ADR argumentaba el parser a mano, entre otras cosas, en que *"the liveness rule
is ours because the relay sends no heartbeat"*. Esa premisa dejó de ser cierta con
este ticket, así que se corrigió la frase y se agregó un párrafo en *Consequences*:
el timeout del cliente pasa a ser una red de contención, y el heartbeat viaja como
comentario —un frame que el parser ya ignoraba— así que el contrato del parser no
cambió y el relay ganó liveness sin que el cliente tocara una línea.

### `.scratch/cli/spec.md` y `.scratch/cli/issues/04-the-connection-holds-itself-up.md`

La decisión de **Liveness** del spec del CLI, su entrada en *Out of Scope* y el
comentario de racionalidad del ticket 04 afirmaban que el relay no manda heartbeat.
Las tres quedaron corregidas. La regla del cliente **no** cambia —sigue asumiendo
muerte tras cinco minutos de silencio—, pero ahora es una red de contención y no
la señal principal.

## Comportamiento resultante

Sobre una conexión ociosa, el relay escribe:

```
: heartbeat
```

y nada más: sin `data:`, sin `event:`, sin `id:` y sin `retry:`. El `id:` no
aparece porque el escritor de SSE de Nest saltea el contador cuando el mensaje es
sólo un comentario (`isCommentOnly`), lo que importa: un `id:` en el frame haría
que un cliente genérico lo leyera como `Last-Event-ID` y ofreciera reanudar desde
ahí.

Un cliente que sólo mira frames con `data:` no ve nada distinto de antes. Ese es
el punto: la liveness y el stream de eventos comparten la conexión sin que ninguno
de los dos sepa del otro.

## Verificación

**Fase roja primero.** Con el caso escrito y sin implementación:

```
× writes a heartbeat while the connection has nothing to say 5010ms
Error: Test timed out in 5000ms.
```

**Después de implementar:**

```
npx tsc --noEmit -p tsconfig.json     → exit 0
npx vitest run --config ./vitest.config.e2e.ts
  Test Files  1 passed (1)
       Tests  49 passed (49)
npm run lint
  Found 0 warnings and 0 errors. (17 files, 107 rules)
```

Los 48 casos previos siguen pasando con el heartbeat activo en toda la suite (a
100 ms), lo que prueba de paso lo que pide AC2: el lector de eventos del harness
saltea el comentario sin romperse, igual que un cliente.

## Acceptance Criteria

- ✅ **AC1: el stream escribe un heartbeat a intervalo fijo cuando no se mandó
  ningún evento en ese intervalo.** El `switchMap` sobre el `timer` reinicia el
  temporizador en cada evento, así que el heartbeat sólo aparece en la ociosidad.
  Cubierto por el caso nuevo, que afirma **dos** frames consecutivos: un heartbeat
  único pasaría un test de un solo frame y no probaría el intervalo.
- ✅ **AC2: el heartbeat no llega a un cliente como ActivityEvent.** Viaja como
  `comment`, así que el frame no tiene línea `data:` y ningún cliente puede
  parsearlo como evento. Afirmado explícitamente en el caso nuevo, y probado de
  refilón por los 48 casos previos, que reciben heartbeats y los ignoran.
- ✅ **AC3: el intervalo es lo bastante corto para vencer el idle timeout de un
  proxy corporativo, y el valor está justificado con evidencia.** Los defaults
  documentados son de **60 segundos** en las dos capas que cortan un stream ocioso:
  - AWS, *Configure the idle connection timeout for your Classic Load Balancer*:
    *"By default, Elastic Load Balancing sets the idle timeout for your load
    balancer to 60 seconds."* — y, sobre la opción que se descartó en la Fase 2,
    *"TCP keep-alive probes do not prevent the load balancer from terminating the
    connection because they do not send data in the payload."*
    (`docs.aws.amazon.com/elasticloadbalancing/latest/classic/config-idle-timeout.html`)
  - nginx, `ngx_http_proxy_module`: `proxy_read_timeout` → *Default:
    `proxy_read_timeout 60s;`* (`nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_read_timeout`)

  Quince segundos son **cuatro veces** por debajo del default más ajustado, lo que
  deja margen para un frame demorado sin perder la conexión. El número no es
  medido: es elegido contra un default documentado, y el ticket lo pide así.
- ✅ **AC4: los casos de stream existentes siguen pasando, y un caso nuevo afirma
  el heartbeat sobre una conexión sostenida.** 49 de 49 en verde, 48 previos + 1
  nuevo.

## Decisiones y desvíos

- **El frame es un comentario, y de eso depende todo.** La alternativa era un frame
  `data:` con una forma distinguible, y es la equivocada por dos razones: un
  cliente tendría que parsearlo para saber que no es un evento, y el contrato de
  siete campos del `ActivityEvent` se rompería por la puerta de atrás. El
  comentario es, además, lo que el harness del ticket 06 ya ignoraba por regla, así
  que el cambio no obligó a tocar un solo caso existente. **Es el punto que un
  lector futuro intentaría "arreglar"**, así que está dicho en el código.
- **`STREAM_HEARTBEAT_MS` es superficie que el ticket no pedía**, y la revisión de
  dos ejes lo marcó como scope creep. Se sostiene igual: AC4 pide un caso que
  afirme el heartbeat, y un caso que espera quince segundos no es un caso que valga
  la pena tener. La palanca es lo que hace testeable el AC, y de paso es real —
  el timeout es del despliegue, no del relay.
- **Un bug real que encontró la revisión, no yo.** La primera versión resolvía el
  intervalo con `Number(process.env.STREAM_HEARTBEAT_MS ?? 15_000)`. `??` no atrapa
  la cadena vacía: `STREAM_HEARTBEAT_MS=""` da `0`, y `timer(0, 0)` escribe frames
  de heartbeat tan rápido como el socket los acepte. Un valor no numérico da `NaN`,
  con el mismo efecto. La revisión de spec lo verificó empíricamente. Ahora un
  valor ausente, vacío o no positivo se trata como no puesto. **Por qué no falla
  rápido**, a diferencia del secreto del webhook: el secreto no tiene default
  seguro y el relay no debe arrancar sin él; un intervalo mal tipeado sí tiene uno,
  y lo único que no puede pasar es que llegue a `timer` como cero.
- **El intervalo no se lee al cargar el módulo, se lee por conexión.** Leerlo una
  vez al importar habría hecho que el `beforeAll` de la suite llegara tarde, porque
  el import de `AppModule` ocurre antes. Leerlo por conexión es además lo que
  corresponde: el valor es del despliegue y puede cambiar sin reiniciar.
- **Desvío de la revisión: el orden de los `describe`.** El bloque nuevo había
  quedado entre los tickets 06 y 07; los bloques del archivo siguen el orden de los
  tickets, así que se movió al final.
- **Desvío de la revisión: la extracción del módulo.** La revisión marcó *Divergent
  Change* en el controlador —dos razones para cambiar: la forma del frame y la
  política del despliegue—. Se resolvió extrayendo `stream-heartbeat.ts`, que es lo
  que el resto del relay ya hace con sus piezas chicas.
- **Lo que la revisión marcó y NO se hizo.** El *Duplicated Code* entre los dos
  bucles de salteo del harness (`nextFrame` saltea frames vacíos, `nextEvent`
  saltea frames sin datos) se dejó como está: los predicados son distintos y un
  "leer hasta que se cumpla un predicado" genérico sería más maquinaria que la
  duplicación que ahorra. Es un juicio, no una violación.
- **Lo que queda sin test, y por qué.** El reinicio del temporizador en cada evento
  no está afirmado por ningún caso. Afirmar una **ausencia** de heartbeat mientras
  fluyen eventos exige una ventana temporal, y un test así es una fábrica de
  flakiness: si el entorno se demora, el caso falla por razones que no son del
  código. Lo que sí está afirmado es la periodicidad (dos frames seguidos), que es
  la parte que un test puede sostener. La decisión se registra acá en lugar de
  quedar implícita.
- **El registro del ticket 06 no se reescribió**, aunque afirme que no hay
  heartbeat. Es evidencia de lo que ese ticket construyó y decidió, y era cierto
  cuando se escribió; reescribirlo falsificaría el registro. Este documento es el
  que dice que ese límite se cerró.

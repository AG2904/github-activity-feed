# Ticket 03 - Una asignación de issue se convierte en evento

## Resumen

Un issue que recibe un assignee mientras lleva una label `bug` o `feedback` se
almacena como `ActivityEvent` y se puede leer de vuelta por HTTP — tanto si el
assignee se agrega después como si viene puesto desde la creación. Es el segundo
tipo de evento crítico, y el primero que obliga a que el modelo deje de ser un
caso único.

Antes de implementarlo se hizo una sesión de diseño del dominio que fijó la forma
que este ticket tenía que respetar: `type` pasa a ser un conjunto cerrado de
literales, `assignee` pasa a `null` en lugar de cadena vacía, el payload de
GitHub queda adentro de la capa de adaptación, y la regla de qué hace crítico a
un evento se muda al dominio. Las decisiones quedaron asentadas en el spec
(§Implementation Decisions), en `GLOSSARY.md` y en el ADR-0005.

## Cambios Realizados

### src/relay/domain/activity-event.ts

- **`ActivityEventType`**: unión de literales (`pr.merged` | `issue.assigned`), no
  un `enum` de TypeScript — el enum emite código en runtime y obliga a importarlo
  en cada sitio.
- **`IsoInstant`**: tipo nominal que sólo produce `isoInstant(...)`, normalizando
  a UTC con `toISOString()`. `when` deja de ser un `string` suelto: cualquiera
  pasaba el typecheck antes.
- **`assignee`** pasa de `string` a `string | null`. La cadena vacía afirmaba "hay
  un assignee y está vacío", que es una mentira sobre el dominio.

### src/relay/domain/critical.ts

- La regla de negocio: `CRITICAL_LABELS`, `MAIN_BRANCH` y los predicados
  `carriesCriticalLabel` / `isMainBranch`. Es vocabulario del producto, no rutas
  de campo de GitHub; el adaptador extrae y le pregunta.

### src/relay/github/

- Carpeta nueva: la frontera del payload, visible en el nombre del módulo.
- **`to-activity-event.ts`** es la única entrada del adaptador. El controlador
  pasa a llamar una sola función y deja de acumular un `if` por tipo de evento.
- **`merge-to-main.ts`** se muda desde la raíz del módulo sin cambiar de nombre:
  "Merge to main" es el término del glosario, y el nombre del archivo debe
  reflejarlo.
- **`issue-assignment.ts`** es el tipo nuevo.
- **`payload.ts`** concentra la lectura defensiva de payloads (`asRecord`,
  `loginOf`, `labelNames`), que antes se habría duplicado en cada adaptador.

### src/relay/activity-events.service.ts

- `fromStoredRow(row)` es una función nombrada en lugar de un objeto literal
  incrustado: el único lugar donde la fila y el shape se tocan.

### src/relay/activity-event.entity.ts

- La columna `assignee` pasa a `nullable: true`.

### test/app.e2e-spec.ts

- Cinco casos nuevos sobre la misma seam HTTP, y los dos casos negativos de
  fixture. Total de la suite: 20 casos.

## Comportamiento resultante

- Una asignación con label crítica queda en el historial con el actor (quien
  asignó), el assignee (quien recibió), las labels aplanadas y el timestamp
  canónico del momento en que GitHub registró la asignación.
- Una creación que ya viene asignada cuenta igual que una asignación posterior.
- Un issue asignado sin label crítica, o una acción que no asigna (labelar,
  editar), no almacenan nada.
- Los merges siguen funcionando, ahora con `assignee: null`.

## Verificación

```text
npm test        → 20 casos e2e en verde
tsc --noEmit    → exit 0
oxlint          → 0 warnings, 0 errors
```

## Acceptance Criteria

- ✅ AC1: Un issue al que se le agrega un assignee llevando `bug` se almacena.
- ✅ AC2: Un issue creado con assignee y llevando `feedback` se almacena. La
  creación lee `created_at` como instante del hecho, y el test usa un
  `updated_at` distinto justamente para probarlo.
- ✅ AC3: El evento almacenado nombra al actor (quien asignó) y al assignee (quien
  recibió), que en los fixtures son personas distintas.
- ✅ AC4: La asignación se detecta desde el conjunto de assignees que el payload
  reporta, nunca desde la acción sola: `opened` cuenta sólo si el issue ya viene
  asignado, y una acción que toca un issue ya asignado no cuenta.

## Decisiones y desvíos

- **`assignee` es `null` en adelante, sin backfill.** Las filas escritas antes de
  este ticket siguen sirviendo `""` desde `/history`, porque `synchronize` ensancha
  la columna pero no reescribe datos. Se decidió no migrar: no hay despliegue
  todavía (el plan de fases lo ubica en la Fase 3) y la base local es descartable.
  Queda anotado por si aparece una base con historial previo.
- **Los fixtures son sintéticos, no capturados.** El spec pide "captured, real
  GitHub payloads"; los de este ticket se derivaron de los ejemplos canónicos
  publicados en `octokit/webhooks` (acciones `assigned` y `opened`), no de una
  entrega real. Es la misma práctica que el ticket 02. Queda como brecha
  explícita hasta que haya un webhook real que capturar.
- **El gate de acción es más estricto que la redacción del spec.** El spec decía
  "asignado cuando el issue se reporta con un conjunto de assignees no vacío",
  sin mencionar la acción; implementado así, labelar un issue ya asignado con
  `bug` habría producido un evento de asignación atribuido a quien puso la label.
  Se endureció el filtro y se corrigió la redacción del spec en el mismo commit.
- **Se endureció la normalización del merge.** Los payloads que pasan el filtro
  pero no tienen actor, título o URL devolvían `500` (por ejemplo, un payload de
  merge sin `sender`). Ahora devuelven `undefined` y se ignoran, que es lo que el
  spec pide para lo malformado. Es la misma clase de defecto que el ticket 02
  había encontrado y corregido a medias; se cerró el resto y se agregó el caso
  que lo prueba.
- **Dos casos de fixture del ticket 04 se adelantaron acá** (issue sin label
  crítica, payload malformado), porque son el mismo predicado que este ticket
  introduce. El ticket 04 conserva el barrido negativo completo.

## Corrección posterior

- **2026-10-19**: el ticket 10 (ADR-0012) retiró a `opened` como trigger de
  asignación. Este ticket lo aceptaba cuando el issue llegaba ya asignado, pero
  GitHub manda `assigned` y `opened` como dos deliveries del mismo hecho, y el
  relay lo guardaba dos veces. La creación sigue almacenándose, por su delivery
  `assigned`. El resto de los criterios de acá siguen vigentes; ver
  `docs/tickets/relay/10-an-assignment-at-creation-counts-once.md`.

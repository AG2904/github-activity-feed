# Ticket 01 - El adaptador nombra sus rechazos

## Resumen

El ADR que registra la decisión de la spec `ingestion` antes de que exista código que
la contradiga: el adaptador de GitHub responde un *outcome* —el evento, o un rechazo
con su motivo— y `undefined` no cruza esa costura. No se modificó código fuente en
este ticket.

## Cambios Realizados

### `docs/adr/0015-the-adapter-names-its-refusals.md`

Documento nuevo. Registra el problema en dos partes: `undefined` borraba la diferencia
entre "no crítico" (el filtro haciendo su trabajo) y "malformado" (el adaptador y
GitHub discrepando sobre una forma que el filtro ya había aceptado); y los predicados
de tipo prometían campos que no verificaban, con lo cual cada mapper revalidaba todo y
el compilador no aportaba nada donde había sido contratado.

Define los dos motivos, fija que `malformed` es terminal en la composición —un payload
no se relee como asignación— y deja escrito que el rechazo es una garantía de
compilación, no observable: nada lo consume todavía, y el día que algo lo consuma, esa
feature trae su propia costura y sus propios tests.

### Decisiones rechazadas, con su razón

`Option`/`Maybe` no se rechaza por gusto: es informativamente idéntico a
`T | undefined` —`None` no puede decir *por qué* no hay nada—, derrota el narrowing
por flujo del compilador en el call site y filtraría un wrapper al JSON del contrato.
También quedan registrados el rechazo de `undefined` documentado en un comentario, el
de la excepción (incompatible con ADR-0002, que acusa y descarta), el del arreglo a
medias de estrechar solo los predicados, y el de una librería de esquemas, con su
precio: la primera dependencia de runtime del relay.

## Comportamiento resultante

Ninguno observable. Es documentación: el relay sigue respondiendo lo mismo. Lo que
cambia es que la decisión queda disponible antes que el código que la implementa, que
es como este repositorio trata sus ADR.

## Verificación

- No se tocó código fuente: el diff de este ticket es un archivo bajo `docs/adr/`.
- El ADR referencia y no contradice a ADR-0002 (firma antes de parsear y acuse sin
  rechazo), ADR-0003 (solo eventos críticos se persisten), ADR-0010 y ADR-0013 (campos
  del shape) y ADR-0005 (el identificador de entrega pertenece a la fila).
- Registra explícitamente el defecto de la misma familia que **queda** fuera de
  alcance: los `as` de fila a dominio en el servicio.

## Acceptance Criteria

- ✅ AC1: un ADR nuevo registra la decisión, en la forma de la casa, numerado a
  continuación (0015).
- ✅ AC2: define los dos motivos y dice cuál es normal y cuál es defecto.
- ✅ AC3: fija la regla de composición —`malformed` es terminal, un payload no produce
  dos lecturas.
- ✅ AC4: registra las alternativas rechazadas y por qué.
- ✅ AC5: cruza los ADR que toca y no contradice ninguno.
- ✅ AC6: dice qué **no** cambia: el `200` de toda entrega autenticada, las filas
  guardadas y el historial servido.
- ✅ AC7: sin cambios de código fuente.

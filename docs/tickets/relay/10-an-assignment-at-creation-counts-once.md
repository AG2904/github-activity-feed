# Ticket 10 - Una asignación al crear se guarda una vez

## Resumen

Un issue que nace ya asignado y con label `bug` o `feedback` producía dos filas en
el historial y dos líneas en el feed. GitHub manda tres deliveries para esa
creación (`labeled`, `assigned`, `opened`), y el relay aceptaba `assigned` y
`opened` como triggers de asignación, así que guardaba el mismo hecho dos veces.
Ahora `opened` no es un trigger: la creación se captura por su delivery
`assigned`, que GitHub emite para el mismo hecho.

## Cambios Realizados

### src/relay/github/issue-assignment.ts

- **`isCriticalAssignment`** acepta solo `action === 'assigned'`. `opened` deja de
  calificar aunque el issue llegue con assignee.
- **`toAssignmentEvent`** deja de leer `created_at`: todo evento que pasa el
  filtro es un `assigned`, y su `when` es `updated_at`.

### test/app.e2e-spec.ts

- El caso "issue created with an assignee" se partió en dos: uno verifica que el
  `opened` de esa creación se ignora, y el otro que el `assigned` del mismo hecho
  se guarda.

### Documentación

- `docs/adr/0012-issue-creation-is-not-an-assignment.md` registra la decisión y
  por qué se rechazó el dedupe por ventana temporal.
- `.scratch/relay/spec.md`: filtering, fixtures y decision history.
- `GLOSSARY.md`: la nota de `Assignment` aclara que ambos casos llegan como
  `assigned`.
- `docs/exposing-the-relay.md`: la nota del doble evento se reemplaza por el
  comportamiento nuevo.

## Evidencia

- `npm test`: 55 casos verdes, incluidos los dos que describen este cambio.
- `npm run build`: sin errores.
- Corrida real antes del fix: un issue creado con `--label bug --assignee`
  contra un Quick Tunnel produjo dos filas (`assigned` y `opened`).
- Corrida real después del fix: el mismo escenario (issue #7, deliveries
  `labeled`, `assigned` y `opened`, todas `200`) produjo **una** fila en
  `/history` y **una** línea en `feed watch`.

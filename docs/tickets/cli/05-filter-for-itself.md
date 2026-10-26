# Ticket 05 - La CLI filtra por su cuenta

## Resumen

Cada desarrollador puede quedarse con sus propios eventos. `feed watch --user
<login>` deja pasar los eventos donde ese login es el actor **o** el assignee;
`--repo <owner/name>` deja pasar solo los de ese repositorio, leído del campo
`repository` del evento. Los dos flags se combinan con AND. El relay sigue
mandando todo: el filtro es local, y un evento descartado no produce ninguna
salida, tampoco en `--json`.

Dos commits en `main`: `df9c4ac feat: filter the broadcast locally (ticket 05)`
y `589301e refactor: name the filtered event correctly (ticket 05)`, el segundo
con las correcciones de la revisión de dos ejes.

## Cambios Realizados

### client/internal/activity/activity.go

- **`Filter{User, Repository}`** y **`Match(event) bool`**: un predicado puro
  sobre el evento, sin flags ni conocimiento de la salida.
- `--user` compara contra `Actor` y, si no coincide, contra `Assignee`; es la
  definición de **Involvement** del glosario y la misma regla que el relay aplica
  en `/history?user=`.
- `--repo` compara contra `Repository`. Un `Repository == nil` (fila anterior a
  la columna, ADR-0010) no matchea ningún filtro de repositorio. No se deriva
  nada del `url`.
- El match es exacto y case-sensitive, igual que el relay. Queda pinneado por un
  test que manda `Maria.Perez` con `--user maria.perez` y exige que no salga.

### client/internal/cli/watch.go

- `watch` acepta `--user <login>` y `--repo <owner/name>` y los pasa al
  `emitter` como `activity.Filter`.
- `emit` devuelve `valid` (el payload era un evento) en lugar de `wrote` (se
  escribió): un evento válido que el filtro descarta igual prueba que la
  conexión está viva y resetea el backoff. La salida se decide después del
  filtro.
- En `--json`, el filtro corre antes de `io.WriteString`: un evento descartado
  no deja ni una línea, y el payload que sí pasa sigue saliendo byte a byte.

### client/test/

- Fixtures unificadas en `eventFrame(when, kind, repository, actor, subject,
  assignee, labels, url)`, con `ptr` para los nullables. `mergedFrame` y
  `assignedFrame` ahora son casos de ese helper, y `mergedOutput`/`assignedOutput`
  concentran la forma renderizada que varios tests esperaban repetida.
- Casos nuevos: filtro por actor, por assignee, por repositorio (incluyendo un
  evento con `repository: null`), conjunción de los dos, case sensitivity, y
  `--json --user` escribiendo solo el payload que matchea.
- El request del CLI sigue siendo `GET /stream` sin query: los casos mandan
  eventos que no matchean por el mismo stream y verifican que no aparezcan.

## Comportamiento resultante

```text
$ feed watch --user lucia.gomez --repo AG2904/github-activity-feed
[issue.assigned] lucia.gomez Ship the onboarding checklist
AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/issues/7
# el merge de maria y el evento de otro repo no se imprimen

$ feed watch --json --user lucia.gomez
{"type":"issue.assigned","repository":"AG2904/github-activity-feed",...}
# un solo renglón, el payload original, sin los eventos filtrados
```

## Verificación

```text
cd client && go test -count=1 ./...
?   	github.com/AG2904/github-activity-feed/client/cmd/feed	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/activity	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/cli	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/render	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/sse	[no test files]
ok  	github.com/AG2904/github-activity-feed/client/test	23.169s

cd client && go vet ./...
(exit 0, sin salida)

cd client && gofmt -l .
(sin salida)

cd client && GOOS=linux go build ./...   → exit 0
cd client && GOOS=darwin go build ./...  → exit 0

npm test
Test Files  1 passed (1)
Tests       54 passed (54)
```

La suite del cliente tiene 25 casos, todos por el límite del proceso.

## Acceptance Criteria

- ✅ AC1: `--user <login>` deja pasar un evento donde el usuario es el actor, uno donde es el assignee, y descarta el resto.
- ✅ AC2: `--repo <owner/name>` deja pasar solo los eventos cuyo repositorio coincide, y descarta el resto.
- ✅ AC3: El repositorio sale del campo `repository` del evento (ADR-0010); no hay derivación desde el `url`.
- ✅ AC4: `--user` y `--repo` se combinan como conjunción.
- ✅ AC5: El filtrado es local: la CLI sigue recibiendo todo lo que el relay emite, y un evento filtrado no produce ninguna salida.

## Decisiones y límites conocidos

- **Match exacto y case-sensitive.** Es lo que hace el relay hoy, y su registro
  deja la sensibilidad al caso como una decisión de producto abierta. El CLI la
  pinnea con un test para que un cambio futuro en cualquiera de los dos lados no
  pase inadvertido. La regla está duplicada en TypeScript y Go; compartirla
  requeriría algo que ninguno de los dos contextos tiene hoy.
- **Un evento filtrado resetea el backoff.** El criterio del ticket 04 dice que
  el reset ocurre cuando llega un evento; el filtro decide qué se muestra, no qué
  llegó. Se actualizó el registro del ticket 04 para reflejarlo.
- **El filtro se aplica solo a `watch`.** `history --user`/`--repo` son del
  ticket 07, aunque reutilizarán `activity.Filter`. *(Pagado en el ticket 07:
  `history` filtra con el mismo `activity.Filter` y la misma regla de aceptación
  que el stream, incluido `--json`.)*
- **No hay validación del formato de `--repo`.** Un valor sin `/` simplemente no
  matchea; el spec no pide rechazarlo.
- **Deuda ajena:** el árbol de `ARCHITECTURE.md` y los scripts npm siguen en el
  ticket 08.

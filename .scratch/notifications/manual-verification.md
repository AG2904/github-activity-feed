# Verificación manual pendiente — notificaciones, ticket 01

Documento de trabajo para quien retome esto en esta máquina. **No es un ADR ni una
spec**: es el contexto que hace falta para terminar la única verificación que no
pudo cerrarse, sin reconstruir la conversación. Se borra cuando los dos chequeos
de abajo queden registrados en `issues/01-the-watch-can-ring.md`.

## Estado

| Qué | Dónde |
| --- | --- |
| Decisión | `docs/adr/0017-notifications-are-an-escape-sequence-not-a-process.md` |
| Spec | `.scratch/notifications/spec.md` |
| Ticket implementado | `issues/01-the-watch-can-ring.md` (`Status: ready-for-human`) |
| Rama | `feat/notifications` |
| Tickets 02–04 | implementados en esta rama (`ec41468`, `bf6003c`, `16b4c19`) |

Lo que el ticket 01 dejó funcionando: `--notify=off|bell|osc|auto` en `feed watch`,
`off` por defecto, un bell por evento **mostrado** del stream vivo, el sink escrito
al dispositivo de consola (`CONOUT$` / `/dev/tty`) y nunca a stdout o stderr,
rechazo con código `2` de un valor desconocido, y `osc`/`auto` aceptados y mudos
hasta sus propios tickets.

## Ya está en verde — no repetir

- `gofmt -l .` sin salida, `go vet ./...` limpio, `go test -count=1 ./...` completo
  (incluye la suite e2e y los cuatro casos nuevos de `client/test/notify_test.go`).
- La tarjeta con `--notify bell` es **byte-idéntica** a la del mismo watch sin el
  flag, y ni stdout ni stderr llevan el bell: eso lo prueba
  `TestWatchNotificationsLeaveTheStreamsAlone` comparando dos corridas. No hace
  falta revisarlo a mano.

Queda abierto exactamente un punto: el que nunca se puede ver desde un pipe.

## Chequeo 1 — golangci-lint (hecho)

Corrió el 21 de noviembre de 2026 con la versión exacta de CI:

```powershell
$env:GOARCH = "amd64"
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
& "C:\Users\Andrés\go\bin\golangci-lint.exe" version   # v2.15.0, compilado con go1.27.0
cd client
& "C:\Users\Andrés\go\bin\golangci-lint.exe" run        # 0 issues
```

Resultado: **0 issues**. Encontró dos hallazgos reales en el primer intento —dos
parámetros sin usar en `internal/notify/notify_test.go`, `revive
unused-parameter`— que se arreglaron en el código, no con `//nolint`.

`go test -race ./...`, el otro paso del job `client` de CI, no se puede correr acá:
`-race` exige cgo y esta máquina no tiene gcc ni clang. Queda cubierto por CI.

## Chequeo 2 — que la campanada suene

Ningún test puede verla: el bell va al dispositivo de consola, no a un flujo. Se
verifica en tres capas, de la más barata a la más completa. Si una falla, las
siguientes no dicen nada.

### A. La terminal suena

```powershell
[Console]::Write([char]7)
```

Si esto no hace nada, el problema no es el cliente. En Windows Terminal, en
`settings.json`:

```json
"profiles": { "defaults": { "bellStyle": "all" } }
```

`audible` suena, `window` parpadea, `taskbar` destaca la barra, `all` hace todo,
`none` lo apaga. `none` es el falso negativo número uno. En la terminal de VS Code
el equivalente es `terminal.integrated.enableBell`.

### B. El cliente suena por un evento vivo

```powershell
# terminal 1 — el relay. GITHUB_WEBHOOK_SECRET es obligatorio: sin él no arranca.
$env:GITHUB_WEBHOOK_SECRET = "un-secreto-largo-de-prueba"
npm.cmd run start

# terminal 2 — el binario
cd client
go build -o feed.exe ./cmd/feed
.\feed.exe watch --relay http://localhost:3000 --notify bell --backfill 0

# terminal 3 — el webhook firmado, tal cual el Paso 3 de docs/manual-testing.md
```

Esperado: la tarjeta en stdout y **una** campanada. `--backfill 0` es deliberado:
con historial, las tarjetas viejas se imprimen en silencio y no se distingue "el
backfill está bien" de "el bell está roto".

### C. Las reglas que la suite no puede ver

| Qué | Cómo | Esperado |
| --- | --- | --- |
| el backfill no rinde | mandar 3 webhooks con `X-GitHub-Delivery` distinto, después `watch --backfill 10 --notify bell` | 3 tarjetas y **0** campanadas; el cuarto webhook suena |
| lo filtrado no rinde | `watch --user nadie --notify bell` y mandar un merge | ni tarjeta ni campanada; sin `--user`, tarjeta y campanada |
| `history` no rinde | `.\feed.exe history --relay http://localhost:3000` | ninguna señal; `history --notify bell` sale con `2` |

### Si A suena y B no

El problema está entre el flag y el device, no en la terminal. Revisar, en orden:
que el binario sea el recién compilado; que se esté en la terminal que hostea el
proceso (no un tmux ni una ventana reattachada, donde `CONOUT$` apunta a otra
consola); y que el modo sea `bell` y no `osc`/`auto`, que todavía callan a
propósito.

## Hallazgos del entorno

- **El Go local cambió a amd64**: `go env GOHOSTARCH` y `GOARCH` hoy responden
  `amd64` (la nota anterior decía 386). El `go install` de arriba igual fuerza
  `GOARCH=amd64` para que el binario coincida con la máquina y con CI
  (`ubuntu/amd64`).
- **`npm` no corre desde PowerShell** en esta máquina: `npm.ps1` está bloqueado por
  la política de ejecución. Usar `npm.cmd` (o `cmd /c npm ...`).
- **golangci-lint v2.15.0 quedó instalado** en
  `C:\Users\Andrés\go\bin\golangci-lint.exe` y corre limpio. CI sigue sin disparar
  con push a ramas.
- **`-race` no se puede correr acá**: exige cgo y no hay gcc ni clang. Lo cubre CI.

## Qué no hacer

- No agregar un seam inyectable ni un `FEED_NOTIFY_*` para volver observable el
  bell: la decisión está en ADR-0017 y en la spec, y el `off` por defecto es parte
  de ella.
- No tocar `allowOSC777`, `bellStyle` ni el switch de Ghostty desde el código. Son
  del usuario.
- No marcar el ticket `done`: eso pide merge y el registro de evidencia bajo
  `docs/tickets/notifications/`.

## Cómo cerrar

El chequeo de golangci-lint ya está registrado arriba y en el ticket 01. Falta solo
la campanada: registrar qué terminal se usó y qué casos B/C se observaron, y
después borrar este documento.

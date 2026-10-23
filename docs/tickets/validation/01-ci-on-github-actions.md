# Ticket 01 - CI runs both suites on every push

## Resumen

El repositorio estrena CI en GitHub Actions con dos jobs independientes: `relay`
(`npm ci`, build, lint, suite e2e) y `client` (`go build`, `go vet`, gate de `gofmt`,
suite e2e con `-count=1`). Node queda pinneado en 24 y Go se toma del módulo
(`go-version-file`), así que la versión del workflow no puede divergir de la que
compila el cliente. Los comandos se verificaron localmente uno por uno; la corrida
real en Actions —y con ella el AC de rojo/verde— queda **pendiente del primer push**,
y está registrada como gap explícito, no como hecho.

## Cambios Realizados

### `.github/workflows/ci.yml`

- Disparadores `push` y `pull_request` con destino `main`, `permissions:
  contents: read` y un bloque `concurrency` para no apilar corridas del mismo ref.
- **Job `relay`**: checkout, `actions/setup-node` con Node `24.x` y cache npm,
  `npm ci`, `npm run build`, `npm run lint`, `npm test`.
- **Job `client`**: `working-directory: client`, `actions/setup-go` con
  `go-version-file: client/go.mod`, build, `go vet`, un gate de `gofmt -l .` que falla
  si imprime algo, y `go test -count=1 ./...`.
- El cache de Go es explícito y cachea **sólo** `~/.cache/go-build`: el módulo no
  tiene dependencias externas, así que `~/go/pkg/mod` siempre está vacío. La revisión
  sacó ese path y el `restore-keys`, que no aportaba a una clave derivada de un único
  archivo constante.
- Comentarios en el archivo explican las dos decisiones que se leen como raras: por
  qué no se usa el cache nativo de setup-go (no hay `go.sum` que resolver) y por qué
  `-count=1` (la suite compila el binario en `TestMain`; sin él, Go devuelve
  resultados cacheados cuando sólo cambió `internal/`).

## Comportamiento resultante

Localmente, en el worktree y sobre la rama de integración:

```text
npm.cmd ci               exit 0, 327 paquetes
npm.cmd run build        exit 0
npm.cmd run lint         exit 0, 0 warnings / 0 errores, 17 archivos
npm.cmd test             exit 0, 57/57 en 16.07s
go build ./...           exit 0
go vet ./...             exit 0
gofmt -l .               sin salida
go test -count=1 ./...   exit 0, client/test ok en 56.4s
```

El gate de `gofmt` se ejerció contra un archivo temporal desformateado: `gofmt -l` lo
listó, así que la condición de falla funciona. El YAML parsea con PyYAML (jobs
`relay` y `client`, sin `secrets.` ni `needs`).

## Verificación

- Comandos de arriba, ejecutados en el worktree de la rama.
- Parseo del workflow con PyYAML 6.0.3.
- Revisión de dos ejes: el workflow no tiene violaciones duras; se simplificó el
  cache de Go y se anotó que el typecheck de la Fase 3 lo cubre `npm run build`
  (`nest build` → `tsc`).

**No verificado — gap explícito**: la corrida real en GitHub Actions. No hay runner
local (ni Docker ni `act`) y el AC de "una rama rota se ve roja, la misma sin el
error se ve verde" sólo se puede observar después del primer push a `main`. Por eso
el ticket queda `ready-for-human` con este comentario; se cierra cuando la corrida
exista y su resultado quede registrado acá. Tampoco se verificó el comportamiento de
los caches en un runner.

## Acceptance Criteria

- ✅ AC1: `.github/workflows/ci.yml` existe y dispara en `push` y `pull_request` con
  destino `main`, como dos jobs independientes.
- ✅ AC2: el job del relay corre `npm ci`, `npm run build`, `npm run lint` y
  `npm test`, y los cuatro pasan en un checkout limpio (verificado localmente).
- ✅ AC3: el job del cliente corre `go build ./...`, `go vet ./...`, un
  `gofmt -l .` que falla si imprime algo, y `go test -count=1 ./...` desde `client/`.
- ✅ AC4: versiones pinneadas — Node `24.x` en el workflow; Go desde
  `client/go.mod` (`1.27.0` exacto, no `1.27.x`). Diferencia aceptada con el texto
  del ticket: atar la versión al módulo evita que el workflow y el proyecto
  declaren versiones distintas. Caches configurados.
- ⚠️ AC5: la corrida roja/verde **no se ejecutó** — gap explícito hasta el primer
  push; el ticket no se marca `done` por eso.
- ✅ AC6: el workflow no necesita secretos.

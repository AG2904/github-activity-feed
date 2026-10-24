# Ticket 01 - CI runs both suites on every push

## Resumen

El repositorio estrena CI en GitHub Actions con dos jobs independientes: `relay`
(`npm ci`, build, lint, suite e2e) y `client` (`go build`, `go vet`, gate de `gofmt`,
suite e2e con `-count=1`). Node queda pinneado en 24 y Go se toma del módulo
(`go-version-file`), así que la versión del workflow no puede divergir de la que
compila el cliente. Los comandos se verificaron localmente uno por uno, y el AC de
rojo/verde se cerró con dos corridas reales: una verde en `main` y una roja en un PR
descartable con el gate de formato fallando.

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

**Corridas reales en GitHub Actions** (el AC que no se puede ejecutar localmente):

- **Verde** — run `40050526347`, evento `push` a `main`, commit `7108150`:
  `success` en 1 m 31 s. `Relay (NestJS)` 22 s y `Client (Go)` 87 s, con todos sus
  pasos en verde (install, build, lint, test; setup, cache, build, vet, formato,
  test).
- **Rojo** — run `40051037419`, evento `pull_request`, rama `ci-probe-red` (PR #8,
  draft, cerrado sin merge): `failure`. `Relay (NestJS)` en verde y `Client (Go)`
  fallando **exactamente** en `Check formatting`, que es el gate de `gofmt`; el
  archivo de prueba se borró con la rama.

**No verificado**: el comportamiento de los caches en un runner (no se inspeccionó
hit/miss en estas dos corridas), y el camino de una suite roja —el rojo se forzó por
formato, que es el gate más barato de probar, no por un test fallando.

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
- ✅ AC5: la corrida roja/verde se ejecutó y quedó registrada arriba — verde en
  `main` (`40050526347`) y rojo en el PR descartable (`40051037419`, falla en
  `Check formatting`).
- ✅ AC6: el workflow no necesita secretos.

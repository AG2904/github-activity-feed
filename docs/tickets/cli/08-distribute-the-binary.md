# Ticket 08 - El binario se distribuye

## Resumen

La Fase 2 cierra con lo que le faltaba para ser entregable: las entradas de build
(`cli:build`, `cli:dist`, `cli:test`, `cli:lint`), los cuatro binarios estáticos con
la versión inyectada, la mitad de la CLI en el runbook manual, y el árbol y el plan
de fases del documento de arquitectura diciendo que el cliente llegó.

Dos commits en `main`: `96e6504 feat: distribute the binary and its build entry
points (ticket 08)` y el de este registro. El primero lleva los scripts, el build,
el runbook y el documento de arquitectura, porque en este ticket la documentación es
el entregable, no el registro de un entregable.

## Cambios Realizados

### package.json

```json
"cli:build": "cd client && node tools/build.mjs",
"cli:dist": "cd client && node tools/build.mjs --dist",
"cli:test": "cd client && go test -count=1 ./...",
"cli:lint": "cd client && go vet ./..."
```

Los cuatro entran ellos mismos en `client/`, como pide el ticket. `cli:test` usa
`-count=1` para que la suite no se sirva del caché: cada corrida compila los
binarios de prueba desde cero.

### client/tools/build.mjs (nuevo)

- **`cli:build`** construye el target del host; **`cli:dist`** los cuatro
  distribuidos: windows/amd64, linux/amd64, darwin/amd64 y darwin/arm64. El host se
  deriva de `process.platform`/`process.arch`, y si no está entre los cuatro, el
  script sale 1 diciéndolo.
- **Siempre `CGO_ENABLED=0` y `-trimpath`**, en los dos modos, así lo que corre el
  desarrollador es lo que se descarga su compañero y ningún binario lleva rutas de
  un disco adentro.
- **La versión** sale del `package.json` más `git describe --always --dirty
  --abbrev=7`, y se inyecta con `-X main.version` — el mismo mecanismo que la suite
  usa para inyectar la suya. Sin git (un tarball) el build igual sale, con la versión
  del paquete y nada más.
- **Salida**: `client/dist/feed-<os>-<arch>[.exe]`, un nombre por plataforma.
- Es un script de Node y no una línea de shell porque cada target necesita variables
  de entorno, y `CGO_ENABLED=0 GOOS=linux go build` no es un comando que entienda
  ninguna shell de Windows. Node ya está: npm es lo que lo invoca.

### docs/manual-testing.md

- Se agregó **la mitad de la CLI**: construir el binario, apuntarlo al relay, ver un
  evento real llegar con el `watch` abierto, el backfill de la primera conexión, los
  filtros, los casos de tubería (`| tee`, `--json | jq`) y la consulta retrospectiva
  con `--group-by-repo`.
- **Paso 7, la tabla**: qué probás / cómo / qué esperar, con el ticket de cada
  comportamiento, igual que la tabla del relay. Los números de la suite (46 casos de
  la CLI, 54 del relay) y la forma de la estampa de versión son los de estas
  corridas; el `<commit>` es el del árbol desde el que construiste, así que cambia
  con cada build y por eso va como plantilla y no como un hash literal.
- La introducción aclara que el documento tiene dos mitades: el relay primero, el
  binario después.

### ARCHITECTURE.md

- **§5 Estructura**: el árbol de `client/` completo (comando, los cuatro paquetes
  internos, `tools/`, `test/`, `go.mod`), `docs/tickets/cli/` y `.scratch/cli/` entre
  los directorios, y el comentario de `client/` sin el "(pendiente)".
- **§7 Plan de Fases**: la Fase 2 pasa a *completa*, con los cuatro items marcados y
  el ticket de cada uno. Dice también que con esto la Fase 2 queda cerrada.
- **Encabezado y §1**: el estado dice que el Relay y la CLI están implementados y que
  la Fase 3 queda pendiente, y dos frases que hablaban del CLI en futuro ("será
  desarrollado", "el futuro comando `feed --user`") pasan a presente. Es más de lo
  que pide el AC, pero es el mismo hecho: el cliente llegó.

## Comportamiento resultante

```text
$ npm run cli:build
feed: dist/feed-windows-amd64.exe  windows/amd64  1.0.0+96e6504

$ npm run cli:dist
feed: dist/feed-windows-amd64.exe  windows/amd64  1.0.0+96e6504
feed: dist/feed-linux-amd64  linux/amd64  1.0.0+96e6504
feed: dist/feed-darwin-amd64  darwin/amd64  1.0.0+96e6504
feed: dist/feed-darwin-arm64  darwin/arm64  1.0.0+96e6504

$ client/dist/feed-windows-amd64.exe --version
1.0.0+96e6504
```

`-dirty` aparece cuando el árbol tenía cambios sin commitear al construir: la versión
dice de dónde salió el binario, y eso incluye si salió de un árbol sucio.

## Verificación

```text
$ go version -m client/dist/feed-linux-amd64        # y los otros tres
	build	-trimpath=true
	build	CGO_ENABLED=0
	build	GOARCH=amd64
	build	GOOS=linux
```

Los cuatro binarios, uno por uno: `windows/amd64`, `linux/amd64`, `darwin/amd64` y
`darwin/arm64`, todos con `-trimpath=true` y `CGO_ENABLED=0`. Además se buscó la
estampa de la versión dentro de los cuatro archivos y está en los cuatro (True), y
el de windows se ejecutó y la imprimió: la inyección no es una promesa del script,
está en el binario.

```text
$ npm run cli:lint
> cd client && go vet ./...
(exit 0, sin salida)

$ npm run cli:test
> cd client && go test -count=1 ./...
?   	github.com/AG2904/github-activity-feed/client/cmd/feed	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/activity	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/cli	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/render	[no test files]
?   	github.com/AG2904/github-activity-feed/client/internal/sse	[no test files]
ok  	github.com/AG2904/github-activity-feed/client/test	23.053s

$ npm test
Test Files  1 passed (1)
Tests       54 passed (54)
```

`gofmt -l .` sigue limpio (se verificó a mano; el AC sólo nombra `go vet` para
`cli:lint`, así que no se cableó).

## Acceptance Criteria

- ✅ AC1: `npm run cli:build` construye el binario del host y `cli:test`/`cli:lint`
  corren la suite y `go vet`, cada uno entrando en `client/` por su cuenta.
- ✅ AC2: `npm run cli:dist` produce los cuatro estáticos, con `CGO_ENABLED=0` y
  `-trimpath`, que es lo que muestra `go version -m` en cada archivo.
- ✅ AC3: cada binario lleva la versión inyectada en el build (la estampa está en los
  cuatro archivos, y el de windows la reporta al ejecutarse).
- ✅ AC4: `docs/manual-testing.md` tiene la sección de la CLI: cómo correr el binario
  contra el relay local y ver un evento real llegar, incluido el caso pipeado.
- ✅ AC5: el árbol del repositorio y el plan de fases del documento de arquitectura
  reflejan la llegada del cliente.

## Decisiones y límites conocidos

- **Script de Node, no una línea de shell.** El ticket y el spec piden scripts npm, y
  lo son: `cli:build` y `cli:dist` son las entradas. Adentro hay un `.mjs` porque los
  cuatro targets necesitan `GOOS`/`GOARCH`/`CGO_ENABLED` por build, y no existe una
  forma de escribir eso que funcione en cmd, en PowerShell y en sh a la vez. La
  alternativa era `cross-env` como dependencia, que es exactamente el costo que el
  comentario del ticket rechaza para `goreleaser`.
- **La versión es paquete + commit**, no un número a mano ni un tag inventado. El
  repo todavía no tiene tags, así que `git describe --always` da el commit corto, que
  es literalmente "de dónde vino". Sin git el build no se cae: sale la versión del
  paquete y punto. El runbook lo muestra como plantilla (`1.0.0+<commit>`) porque el
  hash cambia con cada commit: un literal ahí estaría viejo en el commit siguiente.
- **Los binarios viven en `client/dist/`**, que ya estaba ignorado por la regla
  `dist/` del `.gitignore` (la que usa el build de Nest). No hizo falta tocarlo, y
  nada construido se commitea.
- **`cli:lint` es `go vet`, y sólo eso.** El AC lo nombra así; `gofmt -l` se sigue
  verificando a mano y aparece en los registros de cada ticket. Si algún día molesta,
  el lugar es el script de npm, no el build.
- **Este ticket no sumó casos a la suite.** No es una omisión: su comportamiento no
  está del lado del proceso que la suite prueba —la seam acordada—, y la parte que sí
  lo está (que la versión inyectada por `-ldflags` llegue a `--version`) ya estaba
  pinneada por `TestVersionIsInjectedAtBuildTime`, que compila el binario con las
  mismas banderas. Lo que queda sin prueba automática es que `cli:dist` produzca los
  cuatro archivos con las banderas correctas, y eso se verifica ejecutando los
  comandos de arriba y leyendo `go version -m`, que es evidencia, no una promesa.
- **La tabla del runbook referencia los tickets** (01, 02, 05, 06, 07, 08, 10, 16, 17,
  21), mezcla números de ticket con números de user story, igual que la tabla del
  relay que ya estaba. Es la convención del documento.
- **Deuda que queda para la Fase 3:** CI en GitHub Actions, publicación de los
  binarios y el README del repositorio, que sigue sin existir.

## Revisión

Dos ejes, y esta vez encontraron sobre todo documentación que mentía.

**Lo que se arregló.**

1. **Un comando inventado en el runbook.** `feed watch --json | jq '.[0].assignee'`
   no funciona: `watch --json` escribe un objeto por línea, no un array, así que
   `.[0]` es un error de jq. El ejemplo ahora es `.assignee` por línea, con una línea
   explicando la diferencia con `feed history --json`, que sí es un array solo.
2. **La estampa de versión del runbook era de otro commit** (`1.0.0+88bb654`, del
   ticket 07) y encima sin `-dirty` en un árbol que estaba sucio, lo que contradecía
   la regla que el propio documento explica. Ahora es plantilla (`1.0.0+<commit>`)
   con la explicación: el hash cambia con cada build, así que un literal está viejo en
   el commit siguiente. El registro sí cita la corrida real.
3. **El conteo del relay decía 48** en un archivo que este ticket tocó; son 54 desde
   los tickets 08 y 09 del relay.
4. **El árbol citaba mal los ADR.** Le colgaba ADR-0008 a `cmd/feed/main.go` por la
   inyección de versión y el Ctrl-C, que ADR-0008 no dice; y ADR-0009 a
   `internal/cli/`, cuando ADR-0009 es sólo el módulo anidado. Ahora 0009 está en
   `client/` y 0008 en `internal/cli/`, que es donde el ticket 02 dice que va.
5. **`feed --user` no es un comando** en tres lugares del documento; es
   `feed history --user`, como dice el glosario.
6. **`cli:build` prometía más de lo que hace**: dice "tu plataforma, si está entre
   las cuatro", y el documento aclara qué pasa con un host que no lo esté (una línea
   y salida 1, no otro binario).
7. **En `build.mjs`:** el `const { version }` de adentro tapaba al `version()` de
   afuera, y el mapa de arquitecturas traía `ia32: '386'` para un target que no
   existe entre los cuatro. El mensaje de "no hay target" ahora nombra la plataforma
   del host y las cuatro que sí existen.
8. **§9 del documento de arquitectura** no mencionaba la Fase 2 mientras el
   encabezado la daba por hecha: ahora tiene su item.

**Lo que se mantuvo.** La racionalidad de "npm al frente, Node adentro" aparece en el
script, en el ticket y en el registro; es el mismo motivo contado para tres lectores
distintos, y en el runbook se recortó a una línea. Y las dos frases de §1 que pasaron
de futuro a presente siguen ahí: es más de lo que pide el AC5, pero es el mismo hecho
—el cliente llegó— y el registro lo dice.

# Ticket 03 - A tag releases the four binaries

## Resumen

Un push de tag `v*` compila los cuatro binarios estáticos y crea el GitHub Release
con los cuatro assets. La versión la manda el tag, no `package.json`: el workflow
exporta `FEED_VERSION` y el script de build gana ese valor; el build local sigue
calculando `versión+commit` como siempre. La verificación se hizo de punta a punta
con un tag de prueba (`v0.0.0-test`): la corrida salió verde, el Release se publicó
con los cuatro assets y el binario lleva la versión del tag adentro. Después se
borraron el Release y el tag de prueba.

## Cambios Realizados

### `.github/workflows/release.yml`

- Dispara en `push` de tags `v*`, con `permissions: contents: write` (lo mínimo para
  crear el Release) y un `concurrency` por ref para que dos corridas no compitan por
  el mismo Release.
- Pasos: checkout, Node 24 con cache npm, Go desde `client/go.mod` (necesario: el
  script de build invoca `go build`; el ticket no lo mencionaba), `npm ci`, build con
  `FEED_VERSION = github.ref_name` y publicación con `gh` y `GITHUB_TOKEN`.
- El nombre del tag no se interpola en el shell: viaja por `env` y el script usa
  `"$REF_NAME"`, porque un tag es entrada no confiable.
- `--verify-tag` evita que `gh` cree un tag que nunca se pusheó.
- El Release se crea como **draft** con los cuatro assets y se publica con
  `gh release edit --draft=false` recién cuando la subida terminó: un upload fallido
  no deja un Release parcial publicado. Si el build falla, el paso de publicación no
  corre.

### `client/tools/build.mjs`

- `FEED_VERSION` con `trim()`: si trae un valor, gana y se usa tal cual; si no, se
  conserva el camino anterior (`package.json` + `git describe`). Un override vacío o
  solo espacios no puede ganarle a la versión del paquete.

## Comportamiento resultante

Tag de prueba `v0.0.0-test` → run `40052648012` (`Release`): **success**, con todos
los pasos verdes (checkout, Node, Go, `npm ci`, build, publish). Release publicado
(`draft=false`) con:

```text
feed-windows-amd64.exe   10.125.312 bytes
feed-linux-amd64          9.988.009 bytes
feed-darwin-amd64        10.216.112 bytes
feed-darwin-arm64         9.508.002 bytes
```

El asset de Windows contiene la cadena `v0.0.0-test` y **no** la caída
`1.0.0+<commit>`, y un build local del mismo commit con
`FEED_VERSION=v0.0.0-test` corre y responde `v0.0.0-test`. El Release y el tag de
prueba se borraron después (204/204) para no dejar ruido en el repositorio.

## Verificación

```text
python yaml.safe_load(release.yml)   OK, job release
build FEED_VERSION=v9.9.9-test       cuatro binarios, --version = v9.9.9-test
build FEED_VERSION="  v9.9.9-trim  " v9.9.9-trim (trim aplicado)
build sin FEED_VERSION               1.0.0+<commit> (camino anterior intacto)
tag v0.0.0-test -> run 40052648012   success, Release con 4 assets, draft=false
```

Revisión de dos ejes: se corrigieron la inyección del nombre de tag en el `run`
(ahora por `env`), la publicación parcial (`--draft` y luego `--draft=false`),
`--verify-tag` y el `trim()` de `FEED_VERSION`. Judgement calls aceptados: el setup
duplicado respecto de `ci.yml` (un reusable workflow para cuatro pasos sería
sobre-ingeniería), la ausencia de cache de Go en release (las corridas son raras) y
la no-idempotencia de re-correr sobre un Release existente (la inmutabilidad es
deliberada).

**No verificado**: ejecutar el asset descargado en esta máquina. Windows lo bloquea
con "Access is denied" incluso sin `Zone.Identifier` y con atributos `Archive`
solamente; el archivo es un PE válido (`MZ`) y su cadena de versión está adentro, y
el build local del mismo commit con el mismo `FEED_VERSION` corre y responde el tag.
La cláusula "runs on its platform" queda evidenciada por ese build, no por el archivo
descargado, y esa diferencia se registra en vez de disimularse.

## Acceptance Criteria

- ✅ AC1: `.github/workflows/release.yml` dispara en tags `v*`; el Release se corta
  pusheando un tag, nunca subiendo archivos a mano.
- ✅ AC2: checkout, Node 24 pinneado, `npm ci`, build con `cli:dist` y el tag como
  versión; `package.json` no se edita en el diff.
- ✅ AC3: los cuatro artefactos (`windows/amd64`, `linux/amd64`, `darwin/amd64`,
  `darwin/arm64`) quedan adjuntos al Release, con `gh` y `GITHUB_TOKEN`.
- ✅ AC4: un build fallido corta antes de publicar; además, el Release se crea como
  draft y se publica después de subir los assets, así que un Release parcial no puede
  quedar publicado.
- ✅ AC5 (con salvedad explícita): el tag de prueba produjo un Release con los cuatro
  assets descargables y el binario reporta el tag (cadena verificada dentro del asset
  y build local del mismo commit ejecutado); la ejecución del archivo descargado en
  esta máquina quedó bloqueada por el entorno, como se detalla arriba.

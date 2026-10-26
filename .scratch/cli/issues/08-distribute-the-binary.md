# 08: The binary is distributed

**What to build:** The build entry points and the cross-platform binaries, plus the runbook that proves the CLI against the real relay.

**Blocked by:** 07

**Status:** done

- [x] `npm run cli:build` builds the binary for the host platform, and `cli:test` and `cli:lint` run the Go suite and `go vet` — each changing directory into `client/` itself.
- [x] `npm run cli:dist` produces static binaries for windows/amd64, linux/amd64, darwin/amd64, and darwin/arm64, with `CGO_ENABLED=0` and `-trimpath`.
- [x] Each produced binary reports the version injected at build time.
- [x] `docs/manual-testing.md` gains a CLI section: how to run the binary against a locally running relay and watch a real event arrive, including the piped case.
- [x] The architecture document's repository tree and phase plan reflect the client's arrival.

## Comments

- 2026-10-17: npm scripts rather than a Makefile because the repository already speaks npm and `make` is not installed on the machine this is developed on. `goreleaser` is deliberately left to Phase 3: today it would be an extra dependency to build four binaries.
- 2026-10-18: Completed (46 process-seam cases sin cambios, `go vet`/`gofmt` limpios, los cuatro binarios verificados con `go version -m`, relay suite 54 verde). Dos commits en `main`: `96e6504 feat: distribute the binary and its build entry points (ticket 08)` y el del registro. El registro de evidencia está en `docs/tickets/cli/08-distribute-the-binary.md`.

  **Los scripts son npm; el build es un `.mjs`.** `cli:build` y `cli:dist` son las entradas y entran ellas mismas en `client/`, pero adentro hay un script de Node (`client/tools/build.mjs`) porque cada target necesita `GOOS`/`GOARCH`/`CGO_ENABLED` por build y no hay una línea de shell que funcione en cmd, PowerShell y sh a la vez. La alternativa era `cross-env`, que es el costo que este mismo ticket rechaza para `goreleaser`.

  **La versión dice de dónde vino el binario.** Sale del `package.json` más `git describe --always --dirty`, inyectada por `-ldflags` con el mismo mecanismo que la suite usa para la suya (`TestVersionIsInjectedAtBuildTime`). Con el árbol sucio aparece `-dirty`; sin git el build igual sale, con la versión del paquete.

  **La verificación es archivo por archivo.** `go version -m` sobre los cuatro confirma `-trimpath=true`, `CGO_ENABLED=0` y el `GOOS`/`GOARCH` de cada uno; los cuatro llevan la estampa de la versión adentro; y el de windows se ejecutó y la imprimió. El único binario que no se puede correr aquí es el de linux y los de macOS, y por eso la evidencia de esos tres es el contenido del archivo y no una ejecución.

  **Este ticket no sumó casos a la suite, y está dicho por qué.** Su comportamiento no está del lado del límite del proceso —la seam acordada—, y la parte que sí (la inyección de la versión) ya estaba pinneada. Queda para la Fase 3 lo que falta de verdad: CI, publicación de los binarios y el README, que sigue sin existir.

  **La revisión encontró documentación que mentía**, que es el riesgo propio de un ticket así. Un ejemplo de tubería con `jq '.[0].assignee'` que no funciona (`watch --json` escribe un objeto por línea, no un array: `.[0]` es un error de jq), una estampa de versión de otro commit presentada como la actual y sin el `-dirty` que el propio documento explica, el conteo del relay desactualizado (48, son 54) en un archivo que este ticket tocó, dos ADR mal citados en el árbol (0008 colgado del comando y 0009 de `internal/cli`, cuando van al revés), `feed --user` en vez de `feed history --user` en tres lugares, y en el script un `const { version }` que tapaba al `version()` de afuera y un `ia32: '386'` para un target que no existe. Todo corregido, y la estampa quedó como plantilla en el runbook con el motivo escrito: un hash literal está viejo en el commit siguiente.

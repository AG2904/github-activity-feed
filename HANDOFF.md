# Handoff — dejar el repo listo para público

Documento **temporal**: existe para continuar el trabajo desde otra máquina. Se
borra cuando el repositorio esté público y la checklist esté cerrada.

Fecha: 8 de noviembre de 2026. Rama `main` sincronizada con `origin/main`; el último
commit de trabajo es `23fdfb4`. Árbol limpio salvo un `odd/` local (ver abajo).

## Estado

- **Release `v1.0.0` publicada** (no draft): cuatro binarios + `SHA256SUMS`.
  <https://github.com/AG2904/github-activity-feed/releases/tag/v1.0.0>
  Verificada: el `SHA256SUMS` publicado coincide con el `.exe` descargado y
  `feed-windows-amd64.exe --version` responde `v1.0.0`.
- **El repositorio sigue privado**, por decisión del 31 de octubre: se publica más
  adelante. Descripción, 7 topics y licencia MIT ya están configurados en GitHub.
- **Cerrado el 31 de octubre:**
  - `SECURITY.md` en la raíz (alcance, versiones soportadas, reporte privado) y
    `.github/PULL_REQUEST_TEMPLATE.md`. El README enlaza los dos en
    *Seguridad y contribución*.
  - Fase 3 marcada como *completa* en `README.md` y `ARCHITECTURE.md` §7 —CI, release,
    camino dockerizado y demo—, con el despliegue interno como opcional sin marcar y
    una entrada nueva en §9. La fecha del encabezado del ARCHITECTURE pasó al 31.
  - Ramas viejas borradas: `feat/cli-go-quality` y `prototype/palette-options`. En
    `origin` queda solo `main`.
  - `npm audit` verificado con el registry respondiendo: `--omit=dev` da **0**
    vulnerabilidades. `@nestjs/platform-express` subió a 12.3.2, que trae multer
    2.5.0 y cierra GHSA-q7qp-rjg7-q26m (alta, DoS remoto). Quedan 2 moderadas
    dev-only por `minimatch` vía `@nestjs/cli`: el único fix que ofrece baja el CLI a
    11, así que no se aplicó; no llega a la imagen del relay.
- **Los badges siguen comentados** en `README.md`, con las dos líneas ya escritas
  listas para descomentar: contra un repo privado muestran "repo not found".
- **Toolchain al día (8 de noviembre):** `client/go.mod` pide `go 1.27.3`, el último
  estable en `go.dev/dl` (un patch de Go es un arreglo de la stdlib que el binario
  lleva adentro). CI y release lo instalan desde el módulo con `go-version-file`, el
  `Dockerfile` ya fijaba `1.27.3` y el binario construido lo reporta en
  `go version -m`. Golangci-lint subió a `v2.15` en el mismo movimiento: el `v2.13`
  está compilado con Go 1.26 y no puede typechequear la stdlib de 1.27 (la corrida
  roja fue `export data version 5 is greater than maximum supported version 4`).

## Lo que falta

1. **Pasar el repo a público** (Settings → General → Danger Zone → Change visibility),
   descomentar los dos badges del README y verificar: los badges renderizan, los
   assets del release se descargan sin autenticación y el *community profile* no marca
   faltantes.
2. **Borrar este documento** cuando la checklist esté cerrada.
3. Opcional, para el portafolio: `homepage` en el repo, social preview, reporte
   privado de vulnerabilidades habilitado (el `SECURITY.md` apunta ahí) y fijar el
   repo en el perfil.

## Verificación del 31 de octubre y del 8 de noviembre, 2026

- `npm run build`, `npm run lint` y `npm test`: 67 casos en verde.
- `npm run cli:lint` y `npm run cli:test`: los cinco paquetes del cliente en verde.
  Con `go 1.27.3`, además: `go build`, `go vet`, `gofmt -l` y golangci-lint `v2.15`
  sin hallazgos.
- `npm run check:docs`: todos los markdown trackeados son UTF-8 plano.
  `npm run diagrams:prune -- --check`: un solo directorio por diagrama en el repo (el
  run sin trackear que quedó en `.archify/` es local y está ignorado).
- CI verde en `2d3af29` y en `23fdfb4`: Relay, Client, Docs e Images.
- Higiene: el barrido de rutas absolutas de usuario y de prefijos de token no
  encuentra nada en los archivos trackeados, y no hay `.env`, `.pem` ni `.key` en el
  índice.

## Notas de entorno

- No hay `gh` instalado: la API de GitHub se consultó con la credencial que ya usa
  `git credential fill` para `github.com` (scopes `gist, repo, workflow`).
- Con `GOTOOLCHAIN=auto` (el default), un `go` local anterior a 1.27.3 descarga el
  toolchain que pide `client/go.mod` la primera vez que se corre un comando dentro de
  `client/`: eso necesita red, y después queda cacheado.
- PowerShell 5.1 reporta el stderr de `git` y `npm` como error aunque el comando
  termine bien. Confirmar con `git status -sb` en lugar de confiar en el exit code.
- `npm run check:docs` recorre `git ls-files`: falla en una copia sin `.git`, y no ve
  un archivo nuevo hasta que está en el índice.
- El release toma la versión del tag (`FEED_VERSION`), no de `package.json`; un tag
  nuevo publica un release nuevo sin tocar el `package.json`.
- `docs/demo/issues-closed.json` es un payload **derivado** del real de la asignación,
  no una entrega capturada. El README del demo lo aclara; si se toca la demo, mantener
  esa distinción.
- `odd/tasks/cli-go-quality.md` quedó sin trackear de una sesión anterior; su trabajo
  ya está mergeado, así que no entra al repo.

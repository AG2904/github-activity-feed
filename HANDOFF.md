# Handoff — dejar el repo listo para público

Documento **temporal**: existe para continuar el trabajo desde otra máquina. Se
borra cuando el repositorio esté público y la checklist esté cerrada.

Fecha: 31 de octubre de 2026. Rama `main` en `cf566f7`, árbol limpio y sincronizado
con `origin/main`.

## Estado

- **Release `v1.0.0` publicada** (no draft): cuatro binarios + `SHA256SUMS`.
  <https://github.com/AG2904/github-activity-feed/releases/tag/v1.0.0>
  Workflow: <https://github.com/AG2904/github-activity-feed/actions/runs/41204029458>
  Verificada: el `SHA256SUMS` publicado coincide con el `.exe` descargado y
  `feed-windows-amd64.exe --version` responde `v1.0.0`.
- **El repositorio sigue privado**. Descripción, 7 topics y licencia MIT ya están
  configurados en GitHub.
- Lo que ya quedó hecho: eventos de cierre (`issue.closed` / `issue.dismissed`) en
  relay y CLI, colores propios de cada cierre, README con la tabla de los cuatro
  tipos, demo regrabado con prompt neutro y un cierre en vivo.

## Lo que falta

1. **Re-habilitar los badges del README.** `README.md:7-9` tiene un comentario que
   explica por qué están apagados (un repo privado hace que `shields` y el endpoint
   de Actions respondan "repo not found"). Al pasar a público, reemplazar el
   comentario por el badge de CI y el de release.
2. **Archivos de comunidad.** Agregar `SECURITY.md` (cómo reportar una
   vulnerabilidad) y `.github/PULL_REQUEST_TEMPLATE.md`. Opcional:
   `.github/ISSUE_TEMPLATE/`. GitHub los lista en el *community profile* del repo.
3. **Borrar ramas viejas.** En `origin` quedan `feat/cli-go-quality` (ya mergeada) y
   `prototype/palette-options` (prototipo descartable):

   ```powershell
   git push origin --delete feat/cli-go-quality prototype/palette-options
   git branch -D prototype/palette-options
   ```

4. **`npm audit` con acceso al registry.** En esta máquina falla con `ECONNRESET`
   (proxy corporativo), así que las vulnerabilidades de dependencias **no están
   verificadas**. Correr `npm audit --omit=dev` y `npm audit` donde el registry
   responda, y decidir si alguna advisory bloquea la publicación.
5. **Actualizar el estado de las fases.** `README.md:239` y `ARCHITECTURE.md:202`
   todavía describen la Fase 3 como "en progreso" e incluyen CI, publicación de
   binarios, docker y demo como pendientes. Ya están los cuatro. Ajustar esas líneas
   y, si corresponde, marcar el cierre de la fase.
6. **Verificación final antes de cambiar la visibilidad** (todo debe quedar verde):

   ```powershell
   npm run build; npm run lint; npm test
   npm run cli:lint; npm run cli:test
   npm run check:docs
   npm run diagrams:prune -- --check
   ```

   Y el barrido de higiene:

   ```powershell
   git grep -n -I -e "C:\\Users" -e "BEGIN PRIVATE" -e "ghp_" -e "github_pat_" -- .
   git ls-files | Select-String -Pattern '\.env$|\.pem$|\.key$'
   ```

7. **Pasar el repo a público** (Settings → General → Danger Zone → Change
   visibility). Después verificar: los badges renderizan, los assets del release se
   descargan sin autenticación, y el *community profile* ya no marca faltantes.
8. **Opcional, para el portafolio**: agregar `homepage` al repo, subir una social
   preview, habilitar el reporte privado de vulnerabilidades y fijar el repo en el
   perfil.

## Notas de entorno (máquina donde se hizo el trabajo)

- No hay `gh` instalado: la PR y la verificación del release se hicieron contra la
  API de GitHub con la credencial que ya usa `git credential fill` para `github.com`.
- PowerShell 5.1 reporta el stderr de `git` y `npm` como error aunque el comando
  termine bien. Confirmar con `git status -sb` en lugar de confiar en el exit code.
- `npm run check:docs` recorre `git ls-files`: falla en una copia sin `.git`.
- WSL está instalado pero sin distribución, y Docker no está disponible: el camino
  dockerizado no se puede probar en esta máquina.
- El release toma la versión del tag (`FEED_VERSION`), no de `package.json`; un tag
  nuevo publica un release nuevo sin tocar el `package.json`.
- `docs/demo/issues-closed.json` es un payload **derivado** del real de la
  asignación, no una entrega capturada. El README del demo lo aclara; si se toca la
  demo, mantener esa distinción.

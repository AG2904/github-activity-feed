# Payloads reales del demo

Dos entregas reales de `github.com`, capturadas el 24 de octubre de 2026 desde el
repositorio descartable `AG2904/github-activity-feed-demo`, con un webhook registrado
en `https://decades-squad-personnel-lit.trycloudflare.com/` (la raíz del túnel, con
barra final: es el `Payload URL` que documentan `docs/end-to-end-testing.md` y
`docs/exposing-the-relay.md`).

- `pull-request-closed.json` — entrega `3850136753461526528`, evento `pull_request`,
  acción `closed` (PR #5, mergeado a `main` desde la rama `demo/merge-3`). 25.108
  bytes.
- `issues-assigned.json` — entrega `3850136758098337792`, evento `issues`, acción
  `assigned` (issue #6, con label `bug`, asignado a `AG2904`). 12.005 bytes.

Cómo se obtuvieron: de la API de deliveries del webhook
(`GET /repos/{owner}/{repo}/hooks/{hook_id}/deliveries/{delivery_id}`), que devuelve el
payload como objeto JSON. Se serializó de nuevo como JSON compacto; la API no expone
los bytes crudos de GitHub, así que estos archivos son el **contenido** real, no una
copia byte a byte del cable. La firma HMAC del replay se calcula sobre estos bytes, y
el relay los aceptó con `200`.

Se revisaron: no contienen secretos, tokens, contraseñas ni direcciones de correo.
Referencian únicamente el repo de demo y sus PR/issue, que se borraron después de la
captura. No son fixtures de test: el ticket 04 prohíbe congelarlos en la suite, porque
eso re-crearía el gap que la demo existe para cerrar.

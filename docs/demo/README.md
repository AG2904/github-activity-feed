# Payloads reales del demo

Dos entregas reales de `github.com`, capturadas el 24 de octubre de 2026 desde el
repositorio descartable `AG2904/github-activity-feed-demo`, con un webhook registrado
en `https://decades-squad-personnel-lit.trycloudflare.com/` (la raíz del túnel, con
barra final: es el `Payload URL` que documentan `docs/end-to-end-testing.md` y
`docs/exposing-the-relay.md`), más un cierre derivado de una de ellas para que el demo
muestre el evento nuevo.

- `pull-request-closed.json` — entrega `3850136753461526528`, evento `pull_request`,
  acción `closed` (PR #5, mergeado a `main` desde la rama `demo/merge-3`). 25.108
  bytes.
- `issues-assigned.json` — entrega `3850136758098337792`, evento `issues`, acción
  `assigned` (issue #6, con label `bug`, asignado a `AG2904`). 12.005 bytes.
- `issues-closed.json` — 12.070 bytes. **No es una entrega capturada**: es el mismo
  issue #6, con la forma exacta de GitHub, con `action: closed`, `state: closed` y
  `state_reason: "completed"`. El repositorio de demo se borró después de la captura,
  así que un cierre real del mismo issue ya no se puede capturar; derivarlo del
  payload real mantiene el contenido verdadero (issue, título, label, asignado) y deja
  explícito qué parte es reconstruida.

Cómo se obtuvieron los dos primeros: de la API de deliveries del webhook
(`GET /repos/{owner}/{repo}/hooks/{hook_id}/deliveries/{delivery_id}`), que devuelve el
payload como objeto JSON. Se serializó de nuevo como JSON compacto; la API no expone
los bytes crudos de GitHub, así que estos archivos son el **contenido** real, no una
copia byte a byte del cable. La firma HMAC del replay se calcula sobre estos bytes, y
el relay los aceptó con `200`.

Se revisaron: no contienen secretos, tokens, contraseñas ni direcciones de correo.
Referencian únicamente el repo de demo y sus PR/issue, que se borraron después de la
captura. No son fixtures de test: el ticket 04 prohíbe congelarlos en la suite, porque
eso re-crearía el gap que la demo existe para cerrar.

## La grabación del README

`demo.cast` y `demo.gif` son una sesión real, no una ilustración:

- `demo.cast` — asciicast v2 (una línea JSON por evento, con tiempos absolutos) de la
  sesión, con los tiempos tal como ocurrieron. Es el artefacto fuente: legible,
  diffeable y versionado. No lleva las secuencias OSC de título de ventana, que
  arrastraban la ruta de la máquina donde se grabó.
- `demo.gif` — un render de ese cast, el que muestra el README.

Qué corre en la grabación: el relay levantado con una base temporal en el puerto
`3010`, el binario de Windows recién compilado, y los payloads de arriba reenviados
con su firma HMAC. Primero `feed history --group-by-repo` con el merge y la asignación
ya almacenados; después `feed watch --backfill 0`, que anuncia el stream y muestra el
cierre en vivo cuando llega. El prompt de la sesión es `github-activity-feed>`: el
directorio de trabajo no se muestra, así que el cast no filtra la ruta ni el nombre de
usuario de la máquina.

La sesión termina en la tarjeta del cierre. Un `watch` no sale por sí solo, y el
pseudo-console de Windows no entrega el interrupt de consola que lo cerraría de forma
ordenada; la grabación corta después de la tarjeta en lugar de inventar un final.

Para regenerar la imagen no hace falta el relay: el cast ya contiene la sesión. En
una máquina con Windows, Linux o macOS, `vhs` o cualquier renderizador de asciicast
producen el GIF desde `demo.cast`:

```bash
npx svg-term-cli --in demo.cast --out demo.svg
agg demo.cast demo.gif          # asciinema/agg
```

El script que produjo el cast no se versiona a propósito: ataría el repositorio a
una herramienta de captura de una sola plataforma para un asset de documentación. Lo
que se conserva es el resultado — el `.cast` — y no el andamio.

Fidelidad de la transcripción: el único carácter que la consola de captura no
devolvió igual es el separador `·` de las líneas de contexto, que llegó como carácter
de reemplazo y se restauró al valor que el binario escribe. Es el único byte no ASCII
de la sesión completa.

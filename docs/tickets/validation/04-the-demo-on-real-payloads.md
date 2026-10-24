# Ticket 04 - The demo runs on real GitHub payloads

## Resumen

La instancia quedó confirmada como **github.com** y el demo se hizo con un repositorio
descartable (`AG2904/github-activity-feed-demo`) para no ensuciar la historia de este
proyecto. Se registró un webhook real por quick tunnel, se generaron un merge y una
asignación, y el relay recibió las dos entregas por la red —con `200`—, las normalizó
y las sirvió. Después se capturaron esos payloads reales, se guardaron como evidencia
y se replayaron con la firma HMAC correcta; el CLI mostró las dos tarjetas, incluida la
rama del merge. El "known gap" del spec del CLI —un fake relay con formato congelado no
puede notar la deriva del cable— queda cerrado por esta corrida, no por más tests.

## Cambios Realizados

### `docs/demo/`

- `pull-request-closed.json` (25.108 bytes) y `issues-assigned.json` (12.005 bytes):
  los payloads reales de las entregas `3850136753461526528` (`pull_request`/`closed`) y
  `3850136758098337792` (`issues`/`assigned`), obtenidos de la API de deliveries del
  webhook. La API entrega el payload como objeto; se serializó de nuevo en JSON
  compacto y la firma del replay se calcula sobre esos bytes.
- `README.md`: qué es cada archivo, de dónde salió, por qué no son fixtures de test y
  la revisión de datos sensibles (sin secretos, tokens ni correos).

### Sin cambios de código

- No se agregó ningún test para los payloads capturados: el ticket lo prohíbe porque
  congelarlos en la suite re-crearía el problema que el demo cierra.

## Comportamiento resultante

- El webhook real apunta a la **raíz** del túnel. La primera configuración apuntaba a
  `/webhook` y GitHub registró las entregas como `Invalid HTTP Response: 404`; se
  corrigió al path que documentan los runbooks y las entregas siguientes devolvieron
  `200`.
- El relay almacenó las dos entregas reales:

```text
[{"type":"issue.assigned","repository":"AG2904/github-activity-feed-demo","branch":null,"actor":"AG2904","subject":"Third demo assignment for the relay","assignee":"AG2904","labels":["bug"],"url":"https://github.com/AG2904/github-activity-feed-demo/issues/6","when":"2026-10-24T16:33:56.000Z"},
 {"type":"pr.merged","repository":"AG2904/github-activity-feed-demo","branch":"demo/merge-3","actor":"AG2904","subject":"Third demo merge for the relay","assignee":null,"labels":[],"url":"https://github.com/AG2904/github-activity-feed-demo/pull/5","when":"2026-10-24T16:33:55.000Z"}]
```

- Replay firmado contra el relay local: `pull_request -> HTTP 200` y
  `issues -> HTTP 200`; `/history` pasó a cuatro eventos (las dos reales más las dos
  replayadas, cada una con su `X-GitHub-Delivery` propio).
- `feed history --json` entregó el array del relay **verbatim**; el reporte agrupado y
  el camino en vivo muestran las tarjetas:

```text
AG2904 assigned to AG2904
  [bug] Third demo assignment for the relay
  AG2904/github-activity-feed-demo · 1m ago
  https://github.com/AG2904/github-activity-feed-demo/issues/6

AG2904 merged demo/merge-3 into main
  Third demo merge for the relay
  AG2904/github-activity-feed-demo · 1m ago
  https://github.com/AG2904/github-activity-feed-demo/pull/5
```

- `watch --backfill 0` conectado al relay imprimió la tarjeta de la asignación en vivo
  al recibir el replay, con el banner y `connected` en stderr.

## Verificación

Comandos y resultados reales:

```text
git delivery status (entregas nuevas)   OK (200) para pull_request/closed e issues/assigned
replay HMAC (fixtures guardados)        pull_request -> 200, issues -> 200
/history                                4 eventos, con branch demo/merge-3 y labels [bug]
feed history --json                     array del relay verbatim
feed history --group-by-repo            las dos tarjetas, agrupadas por repositorio
feed watch --backfill 0 --no-color      tarjeta en vivo + banner/connected en stderr
```

La revisión de datos sensibles de los payloads (secret/token/password/email) dio cero
coincidencias.

**No verificado**: los bytes crudos exactos del cable de GitHub —la API de deliveries
expone el payload parseado, así que los archivos son su contenido re-serializado; la
firma del replay se calcula sobre esos bytes y el relay los aceptó—. Tampoco se
ejercitaron plataformas distintas de Windows para el CLI, ni el deploy opcional
(ticket 05). El quick tunnel no tiene garantía de uptime, y el repositorio de demo y su
webhook se borraron después de la captura, así que las URLs de los fixtures ya no
existen.

## Acceptance Criteria

- ✅ AC1: la instancia quedó registrada como `github.com` en el ticket antes de capturar
  (comentario del 2026-10-24) y los payloads salieron de esa instancia.
- ✅ AC2: se capturaron dos entregas reales —un merge a `main` y una asignación con
  label `bug`—, revisadas y guardadas en `docs/demo/` con su nota de origen.
- ✅ AC3: se replayaron con `X-Hub-Signature-256` correcto contra el relay real; el
  relay las aceptó, las almacenó y las sirvió por `/history` y por el stream.
- ✅ AC4: el CLI renderizó los payloads reales: `merged <rama> into main` con su rama y
  `assigned to …` con `[bug]` antes del título; `--json` entregó los bytes del relay
  verbatim.
- ✅ AC5: además del replay, el webhook real por túnel funcionó: las entregas de
  github.com devolvieron `200` y el relay las procesó.
- ✅ AC6: el registro de evidencia cita comandos y salidas reales, nombra el "known
  gap" del spec del CLI como cerrado y dice qué no se verificó.
- ✅ AC7: no se agregó ningún test para los payloads capturados.

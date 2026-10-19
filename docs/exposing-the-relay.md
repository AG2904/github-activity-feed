# Exponer el relay: túneles para webhooks de GitHub

GitHub necesita una URL pública para entregar un webhook, y el relay corre en tu
máquina. Un túnel resuelve esa distancia **sin abrir puertos y sin desplegar nada**.
Esta guía explica qué es, cuánto cuesta y cómo usarlo para probar el sistema completo
con un repositorio real; el recorrido de punta a punta está en
[`docs/end-to-end-testing.md`](end-to-end-testing.md#paso-12--github-de-verdad-opcional).

## Camino corto

1. Levantá el relay como siempre (`GITHUB_WEBHOOK_SECRET` + `npm.cmd start`).
2. En otra terminal: `cloudflared tunnel --url http://localhost:3000`. No hace falta
   cuenta ni permisos de administrador: se baja el `.exe` del release oficial y se
   ejecuta.
3. Copiá la URL `https://…trycloudflare.com` y usala como **Payload URL** del webhook,
   con la barra final.
4. Hacé un merge a `main` en el repo de prueba y mirá tu `feed watch`, que sigue
   apuntando a `http://localhost:3000` y no a la URL del túnel: ver
   [el límite conocido](#un-límite-conocido-el-stream-en-vivo).
5. Cerrá el túnel (`Ctrl-C`) cuando termines.

## El problema que resuelve

GitHub vive en internet. Tu relay vive en `http://localhost:3000`, detrás del NAT del
router y del firewall. Para GitHub esa dirección no existe: `127.0.0.1` en sus
servidores es *su* propia máquina.

Las salidas clásicas son tres:

| Salida | Costo | Por qué duele |
| --- | --- | --- |
| IP pública fija + abrir el puerto + DNS + TLS | proveedor de internet + tiempo | no lo hacés en una laptop de desarrollo |
| Desplegar el backend | servidor | ya no estás probando el proyecto local |
| **Un túnel** | normalmente cero | el relay sigue siendo tu proceso, en tu máquina |

## Cómo funciona

```
GitHub ──HTTPS──▶ https://algo-random.trycloudflare.com   (edge del proveedor, TLS)
                                │
                                │  conexión persistente, iniciada desde adentro
                                ▼
                     cloudflared (tu máquina)  ──▶  http://localhost:3000
```

1. Corrés un cliente (`cloudflared`, `ngrok`) en tu máquina.
2. Ese cliente abre una conexión **saliente** al proveedor y la deja viva. Como sale,
   el firewall no la bloquea y no hace falta abrir ningún puerto.
3. El proveedor te da un hostname público y termina el TLS por vos.
4. Cuando GitHub hace `POST` a ese hostname, el proveedor mete el request por la
   conexión abierta, tu cliente lo reenvía a `localhost:3000`, y la respuesta vuelve
   por el mismo camino.

Dos aclaraciones que evitan confusiones:

- **No es una VPN.** Una VPN te da acceso a una red; un túnel publica *un puerto* en
  una URL.
- **No es un deploy.** El relay sigue siendo tu proceso local, con tu SQLite local, y
  lo apagás cuando querés.

Los proxies cortan conexiones ociosas, y por eso el relay escribe un comentario SSE cada
15 segundos (ticket 08): ésa es la liveness que cualquier proxy necesita, y sin ella el
stream moriría en el primer minuto. Con los Quick Tunnels de Cloudflare, además, hay un
límite propio que no tiene que ver con eso.

## Un límite conocido: el stream en vivo

Medido en dos máquinas y dos redes distintas en octubre de 2026 —una de ellas doméstica,
sin firewall corporativo ni proxy de por medio—, contra un Quick Tunnel (`cloudflared`
2026.10.1, `protocol=quic`, sin cuenta y sin administrador):

| Ruta por la URL pública | Resultado |
| --- | --- |
| `POST /` firmado | `200 {"received":true}` en ~0,3 s y la fila aparece en `/history` |
| `GET /history` | `200` con el body correcto, en ~0,7 s |
| `GET /stream` | cabeceras en ~0,4 s y después **cero bytes, indefinidamente** |

La fila del `POST` no es sólo de `curl`: un webhook **real** de GitHub apuntando a la URL
del túnel entregó su `ping` inicial y después un `issues` con `200 {"received":true}` en
*Recent Deliveries*, y la fila apareció en `/history` local y público.

Se reprodujo con `curl` y con el binario del CLI, con una ráfaga de 40 eventos (unos
12 KB) y con `STREAM_HEARTBEAT_MS=1000` para descartar un problema de silencio inicial.
En local, el mismo `/stream` manda cabeceras en 0,02 s y entrega el evento y los
heartbeats (306 bytes en 45 s).

Que se reproduzca en una red doméstica importa: **no es un firewall corporativo, ni un
proxy de la empresa, ni la máquina**. En esa segunda red el webhook público y `/history`
funcionan igual de bien; lo único que no atraviesa es el stream.

Y los bytes no quedan retenidos: matando el relay con el stream abierto desde la URL
pública, el cliente recibe `0` bytes igual, no un volcado final. El edge descarta, no
bufferiza para liberar después.

No es una limitación genérica de SSE en Cloudflare: un servidor Node que imita las
cabeceras del relay y su formato de frame (`\n` inicial incluido) **sí** streamea por el
mismo túnel, con el primer byte a 1,4 s.

### Qué se descartó

| Hipótesis | Cómo se probó | Resultado |
| --- | --- | --- |
| el primer chunk de 1 byte que escribe Nest | servidor Node que escribe `\n` solo y después frames | streamea |
| las cabeceras del relay (`no-store`, `Pragma`, `X-Accel`, `x-powered-by`) | servidor Node que las copia exactas | streamea |
| QUIC | `cloudflared --protocol http2` | 0 bytes en 42 s |
| el writer de Nest | proxy Node que lee `/stream` del relay y reescribe cada chunk | 0 bytes; su log confirma que sí recibe los chunks |
| un error de cloudflared | `--loglevel debug` | el request se proxea a `localhost:4000/stream`, sin error de body |
| el protocolo del cliente | el mismo `/stream` por HTTP/2 (`ALPN=h2`, `200 text/event-stream`) | 0 bytes en 25 s |
| el túnel puntual | un segundo Quick Tunnel simultáneo, otro hostname | mismo `200` y 0 bytes |
| buffering que se libera al cerrar | matar el origin con el stream abierto desde la URL pública | 0 bytes; no hay volcado final |

El proxy es el dato que más cierra: si el writer fuera la causa, reescribir los mismos
bytes habría funcionado. **La causa raíz queda sin identificar.** Lo único que el patrón
sugiere es el edge: los dos Quick Tunnels de la prueba resolvieron al mismo colo
(`CF-Ray` con sufijo `MIA`), así que un edge distinto no está descartado.

### Cómo se arregla (o no)

| Opción | Costo | Confianza | Comentario |
| --- | --- | --- | --- |
| CLI contra `localhost` | cero | **verificado** | el túnel existe sólo para GitHub → relay |
| desplegar el relay (VPS, Fly, Render) | un servidor | alta, sin verificar acá | bypassa cloudflared por completo |
| ngrok | cuenta gratis | media, sin verificar | otra data plane; puede comportarse distinto |
| reescribir el SSE del relay | tiempo | **descartada** | el proxy Node con los mismos bytes falla igual |

Para desarrollo, la primera. Si necesitás el CLI remoto, la segunda: un deploy chico es
más predecible que perseguir un bug de quick tunnels, que además no tienen garantía de
nada.

Dos medidas que conviene tener presentes: la URL tarda en ser alcanzable después de
aparecer —de 8 s a 64 s según la corrida, y si disparás antes falla en silencio—, y un
túnel de cinco ni siquiera resolvió DNS. Los quick tunnels no tienen garantía de
disponibilidad, y eso está en sus términos.

## Efímero o con nombre

| Tipo | URL | Cuenta | Para qué |
| --- | --- | --- | --- |
| **Efímero / quick** | random (`*.trycloudflare.com`, `*.ngrok-free.app`) | no | probar hoy, una sesión |
| **Con nombre / persistente** | fija (`relay.tudominio.com`) | sí, token y a veces dominio | dejarlo días o compartirlo con el equipo |

La diferencia práctica es una sola: con URL efímera tenés que **actualizar el webhook
en GitHub cada vez que reiniciás el túnel**. Con hostname fijo lo configurás una vez.

## Costos

Consultado en octubre de 2026. Los precios de ngrok cambian seguido y las fuentes no
siempre coinciden: verificá `ngrok.com/pricing` antes de decidir.

| Opción | Costo | Límites que importan |
| --- | --- | --- |
| **Cloudflare Quick Tunnel** (`cloudflared tunnel --url`) | **US$0**, sin cuenta ni dominio | URL random que cambia en cada corrida; el servicio cae si cerrás el proceso |
| **Cloudflare Named Tunnel** | túnel **gratis**; requiere un dominio propio dado de alta en Cloudflare (~US$10–15 **al año** según el TLD) | hostname fijo, sin costo mensual extra en el plan free |
| **ngrok Free** | **US$0** | ~1 GB/mes, ~20.000 requests/mes, 3 endpoints online, URL random; algunas fuentes de 2026 reportan un tope de sesión de ~2 h |
| **ngrok Personal / Pro** | ~US$8–10 / ~US$20 por mes | sin las cuotas del free, dominio fijo, más conexiones |
| **localtunnel / localhost.run** | US$0 | random y menos confiables; sirven para una prueba corta |
| **VPS propio** (frp, WireGuard, Caddy) | ~US$4–6/mes + dominio | control total; ya es infraestructura, no una prueba |

**Para este proyecto el costo es cero.** El volumen es mínimo: un webhook son
kilobytes y el stream es *una* conexión larga, no miles de requests. El límite que te
va a molestar no es el ancho de banda sino la URL efímera.

## Seguridad: qué queda expuesto

Mientras el túnel está abierto, tu relay es público:

| Ruta | ¿Protegida? |
| --- | --- |
| `POST /` | sí: sin `X-Hub-Signature-256` válida responde `401` |
| `GET /history` | **no**: cualquiera con la URL lee todo el historial |
| `GET /stream` | **no**: cualquiera con la URL recibe todos los eventos nuevos |

Eso es por diseño (ADR-0001: broadcast, el filtro es del cliente). Las consecuencias
operativas: usá el túnel sólo lo que dure la prueba, cerralo al terminar, y no lo
levantes en una red que no controlás. El HMAC protege el ingreso, no la lectura.

## Configurar el webhook

En el repo de prueba: **Settings → Webhooks → Add webhook**.

| Campo | Valor |
| --- | --- |
| Payload URL | la URL del túnel **con barra final** (`https://…/`) |
| Content type | `application/json` |
| Secret | el mismo `GITHUB_WEBHOOK_SECRET` del relay |
| SSL verification | activa (los dos proveedores dan certificados válidos) |
| Events | **Pull requests** e **Issues** |

Para varios repositorios, o repetís el webhook en cada uno, o usás uno a nivel
organización (**Org Settings → Webhooks**, con *All* o *Selected repositories*). Ojo:
el relay **no filtra por repositorio al ingerir**; guarda todo lo que llega y el filtro
por repo es de lectura (`/history?repository=`, `feed watch --repo`).

## Checklist

- [ ] El relay responde en `http://localhost:3000/history`.
- [ ] El túnel está arriba y la URL abre en el navegador.
- [ ] El webhook tiene la barra final, `application/json` y el secret correcto.
- [ ] *Recent Deliveries* muestra `200` con `{"received":true}`.
- [ ] Un merge a `main` produce una fila en `/history` y una línea en `feed watch`.
- [ ] Al terminar, el túnel está cerrado.

## Problemas frecuentes

| Síntoma en GitHub / CLI | Causa |
| --- | --- |
| entrega en rojo, "Connection refused" o timeout | el túnel o el relay están caídos; *Redeliver* cuando vuelvan |
| `401` | el secret del webhook no es el `GITHUB_WEBHOOK_SECRET` |
| `200` pero no hay fila | el evento no era crítico: rama distinta de `main`, sin label `bug`/`feedback`, o acción que no asigna |
| `200` y fila, pero el CLI no imprime | el CLI se conectó tarde (el stream no repite) o el filtro local lo descartó |
| el CLI apunta a la URL del túnel y no muestra nada | el stream no atraviesa los Quick Tunnels (ver [el límite conocido](#un-límite-conocido-el-stream-en-vivo)); apuntá el CLI a `localhost:3000` |
| el CLI muestra reconexiones cada tanto | un proxy intermedio cerró el stream ocioso; el heartbeat de 15 s del relay está pensado para evitarlo |
| la entrega nunca llega desde una instancia corporativa | el hostname del túnel no es alcanzable desde esa red; con github.com funciona siempre |

## Alternativas

- **Desplegar el relay** (VPS, Fly, Render): útil para que quede prendido para el
  equipo, innecesario para probar. Las free tiers duermen y agregan su propio misterio.
- **Servicios de reenvío de webhooks** (`smee.io`, extensiones de `gh`): mismo concepto,
  pensado específicamente para desarrollo; también gratis y también con URL efímera.
- **Doble del relay**: para probar la *lógica* no hace falta túnel ni GitHub; la suite
  del CLI ya habla con un relay falso (`client/test/harness_test.go`).

## Qué se verificó acá

**Verificado en dos máquinas y dos redes** (una doméstica, sin firewall corporativo): el
relay local, el webhook firmado, el stream y el CLI, documentados en
[`docs/manual-testing.md`](manual-testing.md) y
[`docs/end-to-end-testing.md`](end-to-end-testing.md). Con `cloudflared` 2026.10.1
bajado al temporal y sin permisos de administrador: creación del Quick Tunnel sin
cuenta, `protocol=quic`, `POST /` firmado por la URL pública con `200` y su fila en
`/history`, y `GET /history` público. En la segunda red se repitió lo mismo, más el
stream por HTTP/2 y un segundo túnel simultáneo. El límite del stream de la sección
anterior también se midió acá, en las dos redes, no se copió de ningún lado.

**Con un webhook de GitHub de verdad**: eventos `Issues` y `Pull requests`, secret
compartido con el relay. Un issue con label `bug` y assignee llegó como `200
{"received":true}`, quedó en `/history` y el `feed watch` lo pintó desde el stream
local. Ojo al probar: crear un issue **ya asignado** produce dos entregas críticas
(`opened` y `assigned`) y por lo tanto dos filas y dos líneas, porque el relay las
recibe como deliveries distintos.

**No verificado acá**: `ngrok` (no está instalado y su free tier pide cuenta), los
túneles con nombre, el deploy del relay como alternativa de arreglo, y el
comportamiento desde una instancia corporativa. Para esas filas, el documento repite lo
que publica cada proveedor, con fecha.

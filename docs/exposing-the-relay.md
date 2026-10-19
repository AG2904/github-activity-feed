# Exponer el relay: túneles para webhooks de GitHub

GitHub necesita una URL pública para entregar un webhook, y el relay corre en tu
máquina. Un túnel resuelve esa distancia **sin abrir puertos y sin desplegar nada**.
Esta guía explica qué es, cuánto cuesta y cómo usarlo para probar el sistema completo
con un repositorio real; el recorrido de punta a punta está en
[`docs/end-to-end-testing.md`](end-to-end-testing.md#paso-12--github-de-verdad-opcional).

## Camino corto

1. Levantá el relay como siempre (`GITHUB_WEBHOOK_SECRET` + `npm.cmd start`).
2. En otra terminal: `cloudflared tunnel --url http://localhost:3000`.
3. Copiá la URL `https://…trycloudflare.com` y usala como **Payload URL** del webhook,
   con la barra final.
4. Hacé un merge a `main` en el repo de prueba y mirá tu `feed watch`.
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

Y una consecuencia del diseño del relay: los túneles y los proxies cortan conexiones
ociosas. El stream aguanta porque el relay escribe un comentario SSE cada 15 segundos
(ticket 08 del relay). Sin ese heartbeat, el túnel cerraría el stream cada pocos
minutos y verías reconexiones sin motivo aparente.

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
| el CLI muestra reconexiones cada tanto | el proxy cerró el stream ocioso; el heartbeat de 15 s debería evitarlo, revisá que el relay esté actualizado |
| la entrega nunca llega desde una instancia corporativa | el hostname del túnel no es alcanzable desde esa red; con github.com funciona siempre |

## Alternativas

- **Desplegar el relay** (VPS, Fly, Render): útil para que quede prendido para el
  equipo, innecesario para probar. Las free tiers duermen y agregan su propio misterio.
- **Servicios de reenvío de webhooks** (`smee.io`, extensiones de `gh`): mismo concepto,
  pensado específicamente para desarrollo; también gratis y también con URL efímera.
- **Doble del relay**: para probar la *lógica* no hace falta túnel ni GitHub; la suite
  del CLI ya habla con un relay falso (`client/test/harness_test.go`).

## Qué se verificó acá

El relay local, el webhook firmado, el stream y el CLI están verificados en esta
máquina y documentados en [`docs/manual-testing.md`](manual-testing.md) y
[`docs/end-to-end-testing.md`](end-to-end-testing.md).

Los pasos con túnel **no** se ejecutaron acá: ni `cloudflared` ni `ngrok` están
instalados. El comando y el comportamiento son los documentados por cada proveedor, y
el resto del recorrido —lo que pasa una vez que el request entra— sí está probado.

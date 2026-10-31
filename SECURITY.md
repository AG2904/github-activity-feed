# Política de seguridad

Este es un proyecto personal: no hay un equipo de guardia ni un SLA, pero un reporte
bien hecho se lee y se responde. El relay recibe webhooks firmados y el CLI habla con
él por HTTP, así que los dos lados del contrato están en alcance.

## Versiones soportadas

| Versión | Soporte |
| --- | --- |
| `1.0.x` (último release) | recibe arreglos de seguridad |
| `main` | línea viva, sin garantía de estabilidad |

Los binarios se cortan por tag y `package.json` no se bumpea por release: la versión
que vale es la del tag, que es la que reporta `feed --version`.

## Cómo reportar

No se abre un issue público con los detalles. El camino es el reporte privado de
vulnerabilidades de GitHub: **Security** → **Report a vulnerability**
(<https://github.com/AG2904/github-activity-feed/security/advisories/new>). El reporte
queda privado entre quien lo escribe y el mantenedor hasta que se publique el aviso.

Un reporte útil incluye:

- la versión afectada (`feed --version`, o el tag de la imagen);
- la plataforma y cómo se levantó el relay (local, docker, detrás de un túnel);
- los pasos mínimos para reproducirlo, con el request o el payload cuando aplique;
- el impacto observado, no el supuesto.

Sin secretos reales: alcanza con un secreto de prueba y un payload sintético.

## Qué esperar

- Un acuse de recibo con la primera lectura del reporte, y una respuesta con la
  decisión —arreglo, mitigación documentada o descarte razonado— cuando esté tomada.
- No hay recompensas económicas ni un programa de bug bounty.
- Si el arreglo cambia el contrato de `ActivityEvent`, se escribe como ADR en
  [`docs/adr/`](docs/adr/) antes o junto al código, como cualquier otro cambio de
  diseño.

## Fuera de alcance

- **Las lecturas no tienen autenticación, por diseño** (ADR-0001): cualquiera que
  alcance el puerto del relay ve el stream y el historial. Exponerlo a internet sin un
  proxy que autentique es una decisión de despliegue, no una vulnerabilidad; los
  límites medidos están en [`docs/exposing-the-relay.md`](docs/exposing-the-relay.md).
- **Un advisory de dependencia sin camino de explotación** en este código se sigue por
  Dependabot y el workflow de CI, no por el reporte privado.
- **El contenido de `.scratch/` y `docs/demo/`**: tickets y payloads de demostración,
  no configuración desplegada.

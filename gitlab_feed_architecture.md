# Documento de Arquitectura y Diseño: GitHub Activity Feed CLI & Relay

> **Estado:** Diseño actualizado tras la migración de plataforma a GitHub (§2 y §9). El walking skeleton del Relay está implementado (ticket 01).
> **Fecha original:** 11 de Octubre, 2026. **Actualizado:** 15 de Octubre, 2026.

---

## 1. Resumen Ejecutivo y Alcance

Sistema de notificaciones en tiempo real para un equipo de desarrollo de 3 personas sobre **GitHub (github.com)**. Notifica en la terminal de los desarrolladores eventos clave:

1. **Pull Requests:** Cuando un PR es fusionado a `main`.
2. **Issues:** Cuando un issue etiquetado con `bug` o `feedback` es asignado a alguien.

El proyecto es principalmente un **portafolio técnico** — demuestra arquitectura, disciplina de testing y documentación —; el uso como herramienta interna del equipo es un objetivo secundario y opcional (ver §9).

### Restricciones Técnicas Reales y Decisiones de Arquitectura

- **Cero Slack / Teams como destino final:** Evaluados y descartados (§2.3). Las notificaciones llegan a la terminal, sin ruido de chat.
- **Cero Docker:** El relay es un proceso **NestJS (TypeScript)** nativo con **SQLite** embebido — sin contenedores ni servidores de base de datos. Trivial de correr en una demo; admisible por TI si algún día se despliega en la empresa.
- **Autonomía del Cliente:** El cliente CLI será desarrollado 100% en **Go** y distribuido como un **único binario estático** por plataforma (sin dependencias en la máquina del dev).
- **Privacidad y Auditoría:** Sin payloads hacia servicios de terceros; el relay es dueño del dato: **SQLite** para persistencia y soporte del futuro comando analítico `feed --user`.
- **Autenticación:** webhooks de GitHub firmados con **HMAC SHA-256** (`X-Hub-Signature-256`) y verificación en tiempo constante. El secreto vive solo en variables de entorno, nunca en código commiteado.

---

## 2. Justificación Técnica: ¿Por qué este sistema en lugar de las notificaciones nativas de GitHub o Bots?

GitHub cuenta con notificaciones nativas (inbox web, correo, móvil), GitHub Actions, una app oficial de Slack y GitHub Apps. La investigación contra fuentes primarias ([docs/research/github-notifications-vs-relay.md](docs/research/github-notifications-vs-relay.md)) concluye que ninguna de esas opciones cubre el sistema completo:

### 2.1. Mitigación de la fatiga de notificaciones y el cambio de contexto (*Context Switching*)

- **El problema de las herramientas nativas:** Las alertas tradicionales llegan por correo o exigen abrir la interfaz web, rompiendo la concentración. Estudios de ingeniería humana demuestran que un desarrollador tarda en promedio más de 20 minutos en recuperar el estado de flujo tras una interrupción externa.
- **La solución del CLI:** Lleva la información hiperenfocada **directamente a la terminal**, el entorno de trabajo natural del desarrollador, preservando el foco de desarrollo (*Mark et al., 2008*).

### 2.2. Filtrado inteligente y reducción de ruido

- **El problema de las herramientas nativas:** La suscripción nativa de GitHub llega como máximo a "tipo de evento por repositorio" (issues, pull requests, releases...). No existe filtro de entrega por branch ni por label, y el filtrado del inbox (`reason:assign`, etc.) ocurre a la hora de **triar**, no de **entregar**. La retención del inbox es de 3 meses y es personal — no un historial compartido del equipo.
- **La solución del Relay:** Aplica lógica de negocio para entregar únicamente eventos de valor crítico operacional: integración de código en `main` y asignación de tareas urgentes (`bug`/`feedback`), con historial indefinido y compartido.

### 2.3. ¿Por qué no GitHub Actions + Slack/Teams?

Ambos eventos **se pueden expresar** en Actions (`pull_request: types: [closed]` con `merged == true` y `branches: [main]`; `issues: types: [assigned]` con `if:` sobre labels), y una acción puede publicar a Slack/Teams/webhook. Se descarta porque:

- Requiere aprobar **egress SaaS** y entrega la información a un canal de chat — se pierde la terminal como destino.
- No produce **historial de auditoría** persistente ni consultable (`feed --user`); el "registro" queda vivir en el historial efímero del chat.
- GitHub Apps tampoco escapa al problema: su código "necesita correr en algún servidor" con un webhook propio — exactamente la misma pieza que estamos construyendo.

### 2.4. ¿Por qué un CLI/Relay propio en lugar de un Bot de Chat?

- **Soberanía y Cero Dependencias de Terceros:** Un bot de chat depende de APIs externas y de la aprobación de red de la empresa. Un relay propio en NestJS y un CLI en Go garantizan autonomía y privacidad de los metadatos (*NIST SP 800-122*).
- **Ergonomía (*Non-Intrusive DX*):** Las apps de chat generan ruido social y pop-ups disruptivos. La terminal es un canal privado, asíncrono y enfocado.
- **Programabilidad Local:** Un mensaje de chat es texto estático. El evento en el cliente CLI es programable: automatizaciones locales desde la consola (atajos para interactuar con Git, etc.).
- **Historial Estructurado y Auditoría:** El relay almacena los eventos en **SQLite** — de forma indefinida y compartida por el equipo — habilitando consultas retroactivas como `feed --user <nombre>`.

---

## 3. Matriz Comparativa de Arquitectura

| Dimensión | GitHub Nativo (Inbox/Correo/Mobile) | Actions + Slack/Teams | CLI & Relay Propio (Go + NestJS) |
| :---- | :---- | :---- | :---- |
| **Lugar de llegada** | Navegador / Correo / Push móvil | App de chat corporativa | Terminal (CLI) |
| **Filtrado de entrega** | Por tipo de evento; sin branch ni label | Exacto (Actions) pero con ruido del canal | Quirúrgico: `main` y `bug`/`feedback` |
| **Carga Cognitiva** | Alta (rompe flujo de código) | Media-Alta (mezcla chat y alertas) | Baja (in-situ en consola) |
| **Dependencias** | Nube de GitHub | Egress SaaS + APIs de terceros | Relay propio + binario estático |
| **Capacidad de Automatización** | Ninguna (solo lectura web) | Media (workflows ajenos al CLI) | Alta (acciones locales en consola) |
| **Auditoría / Historial** | 3 meses, personal, por hilo | Limitada al historial del chat | SQLite propia, indefinida, compartida |

---

## 4. Arquitectura del Sistema (Flujo de Datos)

```text
[ GitHub (github.com) ]

       │  (Webhook HTTP POST — firma HMAC: X-Hub-Signature-256)

       ▼

[ Relay Backend: NestJS ] ──(guarda en)──> [ SQLite (Historial & Auditoría) ]

       │  (SSE — stream en tiempo real)

       ▼

[ CLI en Go (Cliente) ] ──> (pinta la notificación en la terminal del dev)
```

**Nota de alcance:** los webhooks de GitHub son *push-only* (GitHub llama a tu endpoint; no existe modo pull oficial). Para las demos y el desarrollo se usan payloads de prueba/fixtures y, opcionalmente, un túnel temporal; un despliegue con webhook real requiere un endpoint alcanzable desde internet.

---

## 5. Estructura Actual del Repositorio (monorepo: el Relay hoy, el cliente Go en Fase 2)

```text
gitlab-notifier/
├── src/                       # Relay (NestJS + SQLite)
│   ├── app.module.ts          # wiring + fail-fast del secreto de entorno
│   ├── main.ts
│   └── relay/                 # seam HTTP única
│       ├── domain/                 # modelo de dominio: sin TypeORM, sin HTTP, sin payloads
│       │   ├── activity-event.ts   # el shape normalizado (§6)
│       │   └── critical.ts         # la regla: qué hace crítico a un evento
│       ├── github/                 # la frontera del payload: sólo acá se conoce GitHub
│       │   ├── payload.ts          # lectura defensiva de payloads
│       │   ├── to-activity-event.ts    # única entrada: payload -> ActivityEvent | undefined
│       │   ├── merge-to-main.ts        # filtro + normalización del merge
│       │   └── issue-assignment.ts     # filtro + normalización de la asignación
│       ├── activity-event.entity.ts    # la fila persistida (ADR-0004)
│       ├── activity-events.service.ts  # store/list
│       ├── activity-stream.ts      # el broadcast: entrega en vivo, sin historia
│       ├── ingress.ts              # el parser único: bytes crudos, firma antes de parsear (ADR-0002)
│       ├── webhook.controller.ts   # POST / — validación HMAC (ADR-0002)
│       ├── history.controller.ts   # GET /history — historial de auditoría
│       ├── stream.controller.ts    # GET /stream — canal SSE en vivo (ADR-0001)
│       └── relay.module.ts
├── test/                      # e2e sobre la seam HTTP (vitest + supertest)
├── docs/
│   ├── adr/                   # registro de decisiones (ADR)
│   ├── agents/                # convenciones del tracker de issues
│   ├── research/              # feasibility research (GitHub vs relay)
│   └── tickets/relay/         # registros de evidencia por ticket completado
├── .scratch/relay/            # spec + tickets
├── GLOSSARY.md                # lenguaje de dominio compartido
└── client/                    # CLI en Go — Fase 2 (pendiente)
```

---

## 6. Contrato de Datos Común (`ActivityEvent`)

```json
{
  "type": "pr.merged",
  "actor": "maria.perez",
  "subject": "Fix memory leak in parser component",
  "assignee": null,
  "labels": ["bug"],
  "url": "https://github.com/AG2904/Gitlab-notifier/pull/42",
  "when": "2026-10-11T13:00:00.000Z"
}
```

Siete campos, ni uno más. `type` es un conjunto cerrado de literales (`pr.merged`, `issue.assigned`); `assignee` es `null` cuando el evento no tiene ninguno, nunca cadena vacía; `when` es UTC en ISO-8601 con milisegundos (`toISOString()`), forma canónica fijada por test. El mismo shape sirve el stream y el historial, para que el cliente parsee una sola cosa.

El identificador de entrega (`X-GitHub-Delivery`) **no** es un campo de este contrato: es una columna de la fila, junto al id de storage, y ninguno de los dos llega al cable (ADR-0005).

---

## 7. Plan de Implementación por Fases

1. **Fase 1: El Relay (NestJS + SQLite)** — *en curso*
   - [x] Walking skeleton: app boota contra SQLite temporal, seam HTTP con autenticación, historial vacío, suite e2e (ticket 01).
   - [x] Migrar la autenticación de `X-Gitlab-Token` a **HMAC `X-Hub-Signature-256`** sobre el cuerpo del request.
   - [x] Reescribir el spec y los tickets de filtrado (02–07) con la semántica de eventos de GitHub (`pull_request`, `issues`).
   - [x] Un merge a `main` se convierte en un evento almacenado y legible por HTTP (ticket 02).
   - [x] Una asignación con label `bug` o `feedback` se convierte en un evento almacenado (ticket 03).
   - [x] El barrido negativo del filtro: todo webhook no crítico se acusa y se descarta. Incluyó corregir el ingress, que rechazaba el cuerpo antes de verificar la firma (ticket 04).
   - [x] Idempotencia por identificador de entrega (`X-GitHub-Delivery`): la entrega se guarda una sola vez, con el identificador como columna única de la fila (ticket 05).
   - [x] Canal SSE que sostiene la conexión y emite cada evento almacenado a todos los clientes conectados (ticket 06).
   - [x] Historial filtrable por usuario (ticket 07). Con esto la Fase 1 queda cerrada.

2. **Fase 2: El Cliente CLI (Go)**
   - Estructurar el binario con manejo de flags (conexión al relay, filtrado local).
   - Cliente de recepción en tiempo real (SSE) desde el Relay.
   - Interfaz visual en terminal (formato legible, timestamps relativos).
   - Compilar binarios estáticos multiplataforma (`go build`).

3. **Fase 3: Validación y Demo**
   - CI con **GitHub Actions** (typecheck, lint, suite e2e en cada PR).
   - Demostración con payloads reales de GitHub (fixtures capturados) y, opcionalmente, un webhook real vía túnel.
   - Publicación de binarios de la CLI para la demo.
   - *Opcional:* despliegue del relay en servidor interno y prueba con el equipo de 3 devs.

---

## 8. Referencias Académicas y de la Industria

- **Investigación propia con fuentes primarias:** [docs/research/github-notifications-vs-relay.md](docs/research/github-notifications-vs-relay.md) — notificaciones nativas de GitHub, Actions/Slack/GitHub Apps y modelo de entrega de webhooks, citado a docs.github.com.
- **Mark, G., Gudith, D., & Klocke, U. (2008).** *"The cost of interrupted work: more speed and stress"*. Proceedings of the SIGCHI Conference on Human Factors in Computing Systems.
- **Fowler, M. (2014).** *"Event-Driven Architecture"*. martinfowler.com.
- **NIST Special Publication 800-122.** *"Guide to Protecting the Confidentiality of Personally Identifiable Information (PII)"*.
- **Pike, R. (2012).** *"Go at Google: Language Design in the Service of Software Engineering"*. Google Inc.

---

## 9. Estado de la Migración a GitHub (16 de Octubre, 2026)

- [x] Plataforma objetivo cambiada de GitLab self-managed a **GitHub** — decisión fundamentada en §2.
- [x] Investigación de factibilidad con fuentes primarias publicada en `docs/research/`.
- [x] Relay walking skeleton implementado y mergeado en `main` (PR #1).
- [x] Spec y tickets en `.scratch/relay/` reescritos para eventos de GitHub.
- [x] Autenticación migrada a HMAC `X-Hub-Signature-256` (PR #3).
- [x] `GLOSSARY.md` actualizado a la terminología de GitHub. Los ADRs también: ADR-0002 y ADR-0005 reescritos, ADR-0003 corregido.
- [x] Los tres tickets del filtro están en `main`: merge a `main` (ticket 02, PR #4), asignación crítica (ticket 03) y el barrido negativo con el fix del ingress (ticket 04). El 03 y el 04 aterrizaron directo en `main`, sin rama de feature: es un proyecto personal y el PR no aportaba.
- [ ] README del repositorio — **no existe todavía**. Este item lo daba por pendiente de actualizar, pero nunca hubo un README que actualizar; o se escribe uno, o el §6 de este documento sigue siendo la referencia pública del contrato.
- [ ] CI (GitHub Actions) configurado.

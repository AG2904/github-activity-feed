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
│       ├── webhook.controller.ts   # POST / — validación HMAC (en migración)
│       ├── history.controller.ts   # GET /history — historial de auditoría
│       └── relay.module.ts
├── test/                      # e2e sobre la seam HTTP (vitest + supertest)
├── docs/
│   ├── adr/                   # registro de decisiones (ADR)
│   ├── agents/                # convenciones del tracker de issues
│   └── research/              # feasibility research (GitHub vs relay)
├── .scratch/relay/            # spec + tickets (pendientes de migrar a eventos GitHub)
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
  "assignee": "",
  "labels": ["bug"],
  "url": "https://github.com/AG2904/Gitlab-notifier/pull/42",
  "when": "2026-10-11T13:00:00Z"
}
```

---

## 7. Plan de Implementación por Fases

1. **Fase 1: El Relay (NestJS + SQLite)** — *en curso*
   - [x] Walking skeleton: app boota contra SQLite temporal, seam HTTP con autenticación, historial vacío, suite e2e (ticket 01).
   - [ ] Migrar la autenticación de `X-Gitlab-Token` a **HMAC `X-Hub-Signature-256`** sobre el cuerpo del request.
   - [ ] Reescribir los tickets de filtrado (02–07) con la semántica de eventos de GitHub (`pull_request`, `issues`).
   - [ ] Implementar normalización a `ActivityEvent`, idempotencia por identificador de evento y el canal SSE.

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

## 9. Estado de la Migración a GitHub (15 de Octubre, 2026)

- [x] Plataforma objetivo cambiada de GitLab self-managed a **GitHub** — decisión fundamentada en §2.
- [x] Investigación de factibilidad con fuentes primarias publicada en `docs/research/`.
- [x] Relay walking skeleton implementado y mergeado en `main` (PR #1).
- [ ] Spec y tickets en `.scratch/relay/` reescritos para eventos de GitHub.
- [ ] Autenticación migrada a HMAC `X-Hub-Signature-256` (ticket 01 aún valida `X-Gitlab-Token`).
- [ ] `GLOSSARY.md` y README actualizados a la terminología de GitHub.
- [ ] CI (GitHub Actions) configurado.

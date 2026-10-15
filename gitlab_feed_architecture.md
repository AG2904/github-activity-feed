# Documento de Arquitectura y Diseño: GitLab Activity CLI & Relay

> **Estado:** Diseño aprobado y consensuado. Listo para implementación.  
> **Fecha:** 11 de Octubre, 2026\.

---

## 1\. Resumen Ejecutivo y Alcance

Sistema de notificaciones en tiempo real para un equipo de desarrollo de 3 personas en un entorno corporativo GitLab (self-managed). Notifica en la terminal de los desarrolladores eventos clave:

1. **Merge Requests:** Cuando una MR es fusionada a `main`.  
2. **Issues:** Cuando un issue etiquetado con `bug` o `feedback` es asignado a alguien.

### Restricciones Técnicas Reales y Decisiones de Arquitectura

* **Cero Slack / Teams descartado:** Notificaciones directas en la terminal (UX moderna, sin ruidos de chat).  
* **Cero Docker en producción (TI):** El relay de backend no usará Go ni contenedores, sino **NestJS (TypeScript)** como proceso nativo aceptado por TI.  
* **Autonomía del Cliente:** El cliente CLI será desarrollado 100% en **Go** y distribuido como un **único archivo binario estático** (sin dependencias en la máquina del dev).  
* **Privacidad y Auditoría:** Se evita enviar payloads a servicios de terceros (`ntfy`); el relay es propio dentro de la infraestructura interna e incluye **SQLite** para persistencia y soporte del futuro comando analítico `feed --user`.

---

## 2\. Justificación Técnica Profunda: ¿Por qué este sistema en lugar de las notificaciones nativas de GitLab o Bots de Chat?

Aunque GitLab ya cuenta con notificaciones nativas (correo, campanita web) y permite crear Bots de chat (Teams/Telegram/Slack), este sistema aporta ventajas críticas de arquitectura, privacidad y ergonomía:

### 2.1. Mitigación de la fatiga de notificaciones y el cambio de contexto (*Context Switching*)

* **El problema de las herramientas nativas:** Las alertas tradicionales llegan por correo o exigen abrir la interfaz web, rompiendo la concentración. Estudios de ingeniería humana demuestran que un desarrollador tarda en promedio más de 20 minutos en recuperar el estado de flujo tras una interrupción externa.  
* **La solución del CLI:** Lleva la información hiperenfocada **directamente a la terminal**, el entorno de trabajo natural del desarrollador, preservando el foco de desarrollo (*Mark et al., 2008*).

### 2.2. Filtrado inteligente y reducción de ruido corporativo

* **El problema de las herramientas nativas:** GitLab notifica de todo por defecto (comentarios masivos, pushes menores), lo que genera fatiga de alertas (*notification fatigue*) y lleva a los devs a silenciar las bandejas.  
* **La solución del Relay:** Aplica lógica de negocio para filtrar y procesar únicamente eventos de valor crítico operacional: integración de código en `main` y asignación de tareas urgentes (`bug`/`feedback`).

### 2.3. ¿Por qué un CLI/Relay propio en lugar de un Bot de Chat (Teams/Telegram/Discord)?

* **Soberanía y Cero Dependencias de Terceros:** Un bot de chat depende de APIs externas. Si la empresa cambia de plataforma o la red corporativa bloquea el tráfico saliente, el sistema falla. Un relay interno en NestJS y un CLI en Go garantizan autonomía y privacidad de los metadatos corporativos (*NIST SP 800-122*).  
* **Ergonomía (*Non-Intrusive DX*):** Las apps de chat generan ruido social y pop-ups disruptivos. La terminal es un canal privado, asíncrono y enfocado.  
* **Programabilidad Local:** Un mensaje de chat es solo texto estático. Al recibir el evento en un cliente CLI en Go, el evento es programable y abre la puerta a automatizaciones locales desde la consola (ej. atajos para interactuar con Git).  
* **Historial Estructurado y Auditoría:** El relay almacena los eventos en **SQLite**, habilitando consultas analíticas retroactivas desde la consola mediante comandos como `gitlab-feed --user <nombre>`.

---

## 3\. Matriz Comparativa de Arquitectura

| Dimensión | GitLab Nativo (Web/Correo) | Bot de Chat (Teams/Telegram) | CLI & Relay Propio (Go \+ NestJS) |
| :---- | :---- | :---- | :---- |
| **Lugar de llegada** | Navegador / Correo | App de chat corporativa | Terminal (CLI) |
| **Carga Cognitiva** | Alta (Rompe flujo de código) | Media-Alta (Mezcla chat y alertas) | Baja (In-situ en consola) |
| **Nivel de Ruido** | Alto (Todo evento sin filtrar) | Medio (Notificaciones masivas) | Mínimo (Quirúrgico a `main` y bugs) |
| **Dependencias** | Servidores de GitLab | APIs de terceros / Red externa | Servidor interno \+ Binario estático |
| **Capacidad de Automatización** | Ninguna (Solo lectura web) | Ninguna (Texto estático) | Alta (Acciones locales en consola) |
| **Persistencial Local / Auditoría** | No disponible | Limitada al historial del chat | Sí (SQLite integrada para `feed --user`) |

---

## 4\. Arquitectura del Sistema (Flujo de Datos)

\[ GitLab Self-Managed \] 

       │  (Webhook HTTP POST)

       ▼

\[ Relay Backend: NestJS \] ──(Guarda en)──\> \[ SQLite (Historial & Auditoría) \]

       │  (WebSocket / SSE / Stream en tiempo real)

       ▼

\[ CLI en Go (Cliente) \] ──\> (Pinta notificación estilizada en la Terminal del Dev)

---

## 5\. Estructura Propuesta del Proyecto (Monorepo o Repositorios Separados)

gitlab-activity-feed/

├── README.md                  \# Documentación, justificación de arquitectura y guías

├── relay/                     \# Backend (NestJS \+ SQLite)

│   ├── package.json

│   ├── tsconfig.json

│   └── src/

│       ├── app.module.ts

│       ├── webhook/           \# Recepción y validación de webhooks de GitLab

│       ├── events/            \# Gestión de conexiones en tiempo real (SSE/WebSockets)

│       └── database/          \# Configuración de SQLite y modelos de historial

└── client/                    \# CLI (Go)

    ├── go.mod

    ├── main.go                \# Punto de entrada de la CLI (Comandos / Config)

    └── internal/

        ├── domain/            \# Modelos compartidos (ActivityEvent)

        ├── client/            \# Lógica de conexión con el Relay (SSE/WebSockets)

        └── ui/                \# Formato y renderizado en terminal (colores, iconos)

---

## 6\. Contrato de Datos Común (`ActivityEvent`)

{

  "type": "mr.merged", 

  "actor": "maria.perez",

  "subject": "Fix memory leak in parser component",

  "assignee": "", 

  "labels": \["bug"\], 

  "url": "https\://gitlab.internal.corp/proyecto/repo/-/merge\_requests/42",

  "when": "2026-10-11T13:00:00Z"

}

---

## 7\. Plan de Implementación por Fases

1. **Fase 1: El Relay (NestJS \+ SQLite)**  
     
   * Configurar servidor NestJS con endpoint para recibir webhooks de GitLab.  
   * Parsear eventos de Merge Requests (`merge`) y Issues (`assignee` con labels `bug`/`feedback`).  
   * Almacenar eventos en SQLite para auditoría y consulta histórica.  
   * Implementar canal de salida en tiempo real (Server-Sent Events o WebSockets).

   

2. **Fase 2: El Cliente CLI (Go)**  
     
   * Estructurar el binario en Go con manejo de flags (conexión al relay, filtrado local).  
   * Implementar cliente de recepción en tiempo real desde el Relay.  
   * Diseñar la interfaz visual en la terminal (formato legible, timestamps relativos).  
   * Compilar binarios estáticos multiplataforma (`go build`).

   

3. **Fase 3: Pruebas con el equipo (3 devs)**  
     
   * Desplegar el relay en el servidor interno aprobado por TI.  
   * Distribuir el binario compilado de Go a los compañeros de equipo.  
   * Validación en vivo de notificaciones de merges e issues.

---

## 8\. Referencias Académicas y de la Industria

* **Mark, G., Gudith, D., & Klocke, U. (2008).** *"The cost of interrupted work: more speed and stress"*. Proceedings of the SIGCHI Conference on Human Factors in Computing Systems.  
* **Fowler, M. (2014).** *"Event-Driven Architecture"*. martinfowler.com.  
* **NIST Special Publication 800-122.** *"Guide to Protecting the Confidentiality of Personally Identifiable Information (PII)"*.  
* **Pike, R. (2012).** *"Go at Google: Language Design in the Service of Software Engineering"*. Google Inc.
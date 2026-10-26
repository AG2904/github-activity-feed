# Cómo contribuir

Este repositorio es un proyecto personal, pero está construido como si tuviera
un equipo detrás: las reglas de abajo son las que mantienen esa ilusión.

## Antes de escribir código

1. Leer [`ARCHITECTURE.md`](ARCHITECTURE.md) — sobre todo el §6, el contrato de
   datos, y el §7, el plan por fases.
2. Consultar [`GLOSSARY.md`](GLOSSARY.md): el proyecto distingue términos que el
   uso común mezcla (*relay* contra *cliente*, *cliente* contra *CLI*), y esa
   distinción no es decorativa.
3. Si el cambio altera una decisión de diseño, la decisión se escribe como ADR en
   [`docs/adr/`](docs/adr/) antes o junto al código, no después.

## Idioma

- La documentación, los tickets y este archivo se escriben en español.
- Los comentarios del código, los identificadores y los mensajes de error se
  escriben en inglés.

## El recorrido de un cambio

Cada unidad de trabajo nace como ticket, con criterios de aceptación, en
`.scratch/<feature>/issues/`, y termina como registro de evidencia en
`docs/tickets/<feature>/`. La forma de ambos documentos está en
[`docs/agents/`](docs/agents/).

## Verificación

Ninguna rama se propone para integrar con una suite en rojo. Los dos conjuntos
son independientes y se corren por separado:

```powershell
npm.cmd run build; npm.cmd run lint; npm.cmd test   # relay
npm.cmd run cli:lint; npm.cmd run cli:test          # CLI
```

Los comandos de arranque y de uso están en el [README](README.md); el recorrido
manual, paso a paso, en [`docs/manual-testing.md`](docs/manual-testing.md).

## Commits

Mensajes convencionales (`feat(relay): …`, `docs: …`, `fix(release): …`), en
imperativo y describiendo el porqué cuando no es obvio. Sin atribución de
herramientas ni firmas automáticas: el commit dice qué cambió, no con qué editor.

## Reglas que no se negocian

- El contrato de `ActivityEvent` no se toca sin ADR: nueve campos, ni uno más.
- El secreto del webhook no se commitea nunca, ni siquiera como ejemplo con un
  valor real.
- Los payloads de [`docs/demo/`](docs/demo/) son contenido real de github.com:
  no se editan a mano ni se congelan como fixtures de test.

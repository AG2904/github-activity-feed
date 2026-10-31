<!-- El qué se lee en el diff; acá va el porqué. -->

## Qué cambia

## Por qué

<!-- El ticket que lo origina: .scratch/<feature>/issues/... -->

## Verificación

- [ ] Relay: `npm run build; npm run lint; npm test`
- [ ] CLI: `npm run cli:lint; npm run cli:test`
- [ ] Documentación: `npm run check:docs; npm run diagrams:prune -- --check`

## Checklist

- [ ] Los documentos van en español; identificadores, comentarios y mensajes de error, en inglés.
- [ ] Si el cambio toca una decisión de diseño, el ADR está en `docs/adr/` y viaja en este PR.
- [ ] El contrato de `ActivityEvent` sigue en nueve campos, o hay un ADR que explica por qué no.
- [ ] No quedó ningún secreto, ruta absoluta de la máquina ni payload privado en el diff.

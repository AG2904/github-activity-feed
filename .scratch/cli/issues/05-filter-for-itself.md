# 05: The CLI filters for itself

**What to build:** Local filtering, so each developer narrows the relay's broadcast to what concerns them — the client-side half of the broadcast model.

**Blocked by:** 02

**Status:** done

- [x] `--user <login>` passes an event where the user is the actor, passes an event where the user is the assignee, and drops everything else.
- [x] `--repo <owner/name>` passes only events whose repository matches, and drops everything else.
- [x] The repository comes from the event's `repository` field, which the relay reports (ADR-0010); there is no derivation from the `url` anywhere.
- [x] `--user` and `--repo` combine as a conjunction.
- [x] Filtering is local: the CLI still receives every event the relay broadcasts, and a filtered-out event produces no output at all.

## Comments

- 2026-10-17: `--user` matches actor or assignee because that is the rule the relay applies to `/history?user=`, and the two must not disagree about what a user's events are. The repository derivation is the accepted cost of not extending the shape (ADR-0006); it lives here because this is its first use, and ticket 07 reuses it.
- 2026-10-18: Completed (25 process-seam cases, `go vet`/`gofmt` clean, linux/darwin builds clean, relay suite still 54 green). Two commits on `main`: `df9c4ac feat: filter the broadcast locally (ticket 05)` and `589301e refactor: name the filtered event correctly (ticket 05)`, el segundo con las correcciones de la revisión.

  **El filtro vive en el dominio.** `activity.Filter{User, Repository}.Match` es un predicado puro sobre `Event`, sin flags ni salida. `--user` hace match si el login es el actor **o** el assignee (Involvement, la misma regla que `/history?user=`), y `--repo` lee `event.Repository`; no se deriva nada del `url`. Un `repository: null` no matchea ningún filtro de repo (ADR-0010). Los dos flags se combinan con AND.

  **Es local de verdad.** El request sigue siendo `GET /stream` sin query; el relay manda todo y el cliente descarta. En `--json` el filtro corre antes de escribir el payload crudo, así que un evento descartado no deja ni una línea. Un evento válido que el filtro descarta igual resetea el backoff: llegó, aunque no se muestre.

  **Case-sensitive, y ahora pinneado.** El relay aplica match exacto (su ticket 07 lo deja así a propósito), así que el CLI hace lo mismo. Hay un test que manda `Maria.Perez` con `--user maria.perez` y exige que no salga: la decisión queda explícita en vez de depender de una frase en un comentario.

  **Revisión.** Se renombró `sawEvent`/`seen` a `eventSeen`/`valid` porque el valor ahora significa "evento válido", no "evento escrito", y se corrigió el comentario de `stream`. También se actualizó el registro del ticket 04, que decía que el reset ocurría solo con un evento emitido. El helper de fixtures pasó a `eventFrame`, para que los casos de filtro no dupliquen el JSON de ocho campos.

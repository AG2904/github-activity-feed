# 02: Live events are painted in the terminal

**What to build:** An SSE client that parses the relay's frames by hand, and a renderer that turns each ActivityEvent into the two-line terminal form.

**Blocked by:** 01

**Status:** done

- [x] `feed watch` holds a connection to `GET /stream` open and prints an event when the relay writes one.
- [x] An event is printed as two lines: the type badge, the assignee when there is one, and the subject; then the repository, the actor, and the time relative to now.
- [x] On an `issue.assigned` event the assignee is rendered in bold immediately after the badge; on a `pr.merged` event, where `assignee` is `null`, that slot disappears without leaving a gap.
- [x] The subject carries an OSC 8 hyperlink pointing at the event's `url`.
- [x] Escape sequences are used only when stdout is a terminal. With the output piped, with `NO_COLOR` set, or with `--no-color`, the output carries no escape sequences and the `url` is printed raw on the second line.
- [x] `id:`, `event:`, and `retry:` lines and comment frames are ignored; only `data:` payloads become events.
- [x] A frame that does not parse - truncated JSON, a body that is not JSON, an `id:`-only frame - is discarded without ending the process.

## Comments

- 2026-10-17: The parser is hand-rolled for a specific reason (ADR-0007): the relay puts `id:` on every frame and that counter restarts at zero per connection, so a generic SSE client would read it as a resume position that does not exist and would impose its own reconnect policy.
- 2026-10-18: Completed (10 process-seam cases, `go vet` clean, linux/darwin cross-builds clean, relay suite still 54 green). Two commits on `main`: `ba2be89 feat: paint live events in the terminal (ticket 02)` and `98f9fe6 fix: align the feed with the event contract and the console (ticket 02)`, the second from the two-axis review.

  **Interior split.** `internal/sse` hand-rolls the parser (ADR-0007): it yields only `data:` payloads, joins multi-line data, and skips `id:`, `event:`, `retry:`, and comments. `internal/activity` owns the eight-field shape and rejects JSON that is missing a field the contract always carries. `internal/render` owns the two-line form and knows nothing about flags. `internal/cli` remains the flag shell, as ADR-0008 asks.

  **Rendering decisions the ticket left open.** The assignee is the bare login in bold, with no `@` decoration. A `repository: null` (a row stored before the column existed, ADR-0010) drops the segment instead of inventing a name. Relative time stays relative at every age (`3d ago`), rather than switching to an absolute date.

  **The colour path is code-only evidence.** The spec forbids tests from importing the renderer, and a process-seam test cannot allocate a pty. Piped, `NO_COLOR`, and `--no-color` are all covered; the bold assignee and the OSC 8 hyperlink were verified by running the renderer against a sample event and inspecting the bytes. On Windows the check is `GetConsoleMode` plus enabling virtual terminal processing, so a conhost does not print literal escapes.

  **Review outcomes.** Fixed: the invented repository name, the unasked `@`, and the absolute-date fallback. Deferred by their own tickets: the architecture tree and the npm scripts (ticket 08), and reconnection/liveness (ticket 04).

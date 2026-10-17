# 02: Live events are painted in the terminal

**What to build:** An SSE client that parses the relay's frames by hand, and a renderer that turns each ActivityEvent into the two-line terminal form.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] `feed watch` holds a connection to `GET /stream` open and prints an event when the relay writes one.
- [ ] An event is printed as two lines: the type badge, the assignee when there is one, and the subject; then the repository, the actor, and the time relative to now.
- [ ] On an `issue.assigned` event the assignee is rendered in bold immediately after the badge; on a `pr.merged` event, where `assignee` is `null`, that slot disappears without leaving a gap.
- [ ] The subject carries an OSC 8 hyperlink pointing at the event's `url`.
- [ ] Escape sequences are used only when stdout is a terminal. With the output piped, with `NO_COLOR` set, or with `--no-color`, the output carries no escape sequences and the `url` is printed raw on the second line.
- [ ] `id:`, `event:`, and `retry:` lines and comment frames are ignored; only `data:` payloads become events.
- [ ] A frame that does not parse — truncated JSON, a body that is not JSON, an `id:`-only frame — is discarded without ending the process.

## Comments

- 2026-10-17: The parser is hand-rolled for a specific reason (ADR-0007): the relay puts `id:` on every frame and that counter restarts at zero per connection, so a generic SSE client would read it as a resume position that does not exist and would impose its own reconnect policy.

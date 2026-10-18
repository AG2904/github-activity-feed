# 06: Events stream to connected clients

**What to build:** A client can hold a connection open and receive every stored event as it happens, over plain HTTP, without polling.

**Blocked by:** 02

**Status:** done

- [x] A stream endpoint holds the connection open and emits events as they are stored.
- [x] An event posted while a client is connected arrives on that connection.
- [x] Every connected client receives every event.
- [x] The stream travels over plain HTTP and needs no handshake beyond the request itself.

## Comments

- 2026-10-16: Completed (43 e2e cases, 3 new). One commit on `main`: `feat: stream every stored event to connected clients (ticket 06)`. `GET /stream` is an `@Sse` route backed by `ActivityStream`, a subject with no history; `store()` now answers whether the row was written so a redelivery cannot reach the stream twice.

  Two findings are worth carrying forward. First, the frame: an `ActivityEvent` carries its own `type`, and an SSE message reads `message.type` as the **event name**, so passing the event as the message produced `event: pr.merged` with no `data:` line and the payload never reached the wire. A raw-socket probe showed `bytes="\n: connected\n\nevent: pr.merged\nid: 0\n\n"`; the event now travels as the message's `data`. The tests found it, not the code review of the source.

  Second, a correction to my own reasoning. I first added a comment-frame greeting on connect, on the stated premise that Nest defers the SSE headers until the first message — inferred from reading `router-response-controller.js` only to line 139. That premise is false: twenty lines further down it calls `stream.commitHeaders()` in a `setTimeout(…, 0)` right after subscribing, with a comment saying headers are sent "immediately rather than waiting for the first Observable emission". The two-axis review caught it, the greeting is gone, and the tests pass identically — which is the proof. The connection barrier is now the response headers themselves, which is deterministic: Nest subscribes the handler before committing them.

  The three stream cases open a real connection on an ephemeral port, because a held-open stream is not something supertest can observe. Status `ready-for-human` per the canonical triage labels. The evidence record is at `docs/tickets/relay/06-stream-events-to-clients.md`.

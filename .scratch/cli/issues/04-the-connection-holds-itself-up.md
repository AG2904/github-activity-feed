# 04: The connection holds itself up

**What to build:** The reconnect and liveness behaviour that lets the CLI be left running in a terminal for days.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] A dropped connection is reopened without intervention, backing off exponentially from one second to a thirty-second ceiling, with jitter.
- [ ] The backoff resets as soon as an event arrives.
- [ ] A stream that goes silent past five minutes is treated as dead and reopened.
- [ ] Every reconnect attempt writes exactly one line to stderr; stdout carries events and nothing else.
- [ ] A relay that is unreachable at startup is retried indefinitely rather than being fatal.
- [ ] A reconnect never replays: no event already printed is printed again.
- [ ] Ctrl-C exits 0 after flushing, with no stack trace.

## Comments

- 2026-10-17: The relay sent no heartbeat when this ticket was written, so a half-open connection — the corporate proxy that drops a stream in silence — looked alive forever, and the client could only assume death after five minutes and reopen. A false positive is cheap: reconnecting replays nothing, so nothing is lost or duplicated. Relay ticket 08 has since landed a heartbeat, so the timeout is now a backstop rather than the primary signal: it covers a connection that dies inside a heartbeat interval, and a relay that stops writing without closing the socket. The rule does not change — the client still assumes death after five minutes of silence, because it cannot see the relay's timer and a quiet relay still looks like a gone one between beats.

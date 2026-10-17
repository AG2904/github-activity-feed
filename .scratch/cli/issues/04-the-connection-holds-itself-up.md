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

- 2026-10-17: The relay sends no heartbeat, so a half-open connection — the corporate proxy that drops a stream in silence — looks alive forever. The client cannot fix that, so it assumes death after five minutes and reopens. A false positive is cheap: reconnecting replays nothing, so nothing is lost or duplicated. The heartbeat itself is filed as a relay follow-up so this ticket does not depend on a Phase 1 change.

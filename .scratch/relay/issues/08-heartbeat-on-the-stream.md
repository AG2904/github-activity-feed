# 08: The stream announces that it is alive

**What to build:** A heartbeat on `GET /stream`, so a client can tell a quiet relay from a dead connection instead of guessing.

**Status:** done

- [x] The stream writes a heartbeat at a fixed interval whenever no event has been sent in that interval.
- [x] The heartbeat does not reach a client as an ActivityEvent: it must not appear on the stream as data a client would parse.
- [x] The interval is short enough to defeat a corporate proxy's idle timeout, and the chosen value is justified from evidence rather than guessed.
- [x] The existing stream cases still pass, and a new case asserts the heartbeat on a held-open connection.

## Comments

- 2026-10-17: Opened by the Phase 2 grilling, and deliberately not part of it. The CLI works without this: it treats five minutes without a byte as death and reopens. What is missing without a heartbeat is the ability to distinguish a quiet relay from a gone one — the client can only assume the worst after a timeout — and a reconnect that a heartbeat would have avoided. Filed so that Phase 2 does not depend on a Phase 1 change to be correct.
- 2026-10-17: Completed (49 e2e cases, 1 new). `stream-heartbeat.ts` composes the liveness half of the stream — a `: heartbeat` comment frame every 15 s whenever the connection would otherwise be idle — and the controller merges it beside the events. The frame is a comment and not data, so no client can parse it as an ActivityEvent and the seven-field shape is untouched; it is also the frame the ticket 06 harness already skipped, so no existing case needed touching. The interval is four times under the tightest documented proxy default (60 s: AWS's load balancer idle timeout, and nginx's `proxy_read_timeout`), and is overridable per deployment.

  Two things are worth carrying forward. The two-axis review found a real bug I had written: `Number(process.env.STREAM_HEARTBEAT_MS ?? 15_000)` does not catch an empty string, so `STREAM_HEARTBEAT_MS=""` reached `timer(0, 0)` and would have written frames as fast as the socket accepted them. And the change invalidated a premise written elsewhere — ADR-0007 and the CLI's liveness decision both argued from "the relay sends no heartbeat" — which is now corrected in both, because the client's timeout is a backstop rather than the primary signal. The evidence record is at `docs/tickets/relay/08-heartbeat-on-the-stream.md`.

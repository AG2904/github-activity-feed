# 08: The stream announces that it is alive

**What to build:** A heartbeat on `GET /stream`, so a client can tell a quiet relay from a dead connection instead of guessing.

**Status:** needs-triage

- [ ] The stream writes a heartbeat at a fixed interval whenever no event has been sent in that interval.
- [ ] The heartbeat does not reach a client as an ActivityEvent: it must not appear on the stream as data a client would parse.
- [ ] The interval is short enough to defeat a corporate proxy's idle timeout, and the chosen value is justified from evidence rather than guessed.
- [ ] The existing stream cases still pass, and a new case asserts the heartbeat on a held-open connection.

## Comments

- 2026-10-17: Opened by the Phase 2 grilling, and deliberately not part of it. The CLI works without this: it treats five minutes without a byte as death and reopens. What is missing without a heartbeat is the ability to distinguish a quiet relay from a gone one — the client can only assume the worst after a timeout — and a reconnect that a heartbeat would have avoided. Filed so that Phase 2 does not depend on a Phase 1 change to be correct.

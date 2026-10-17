# The SSE client is hand-rolled, not a third-party library

The CLI parses `text/event-stream` itself, in roughly forty lines over `bufio`: split on a blank line, collect the `data:` lines, ignore everything else, discard a frame that does not parse.

A library would be the obvious choice and is the wrong one here, because the relay's frames are not the SSE the libraries assume. Nest's stream writer puts an `id:` line on **every** message, and that value is a counter that starts at `0` on each connection — it identifies nothing. A generic client reads it as `Last-Event-ID`, offers it back on reconnect as a resume position, and brings its own reconnect policy, which is exactly the behaviour this client must control: the relay holds no per-connection history to resume from, the backoff is ours to tune, and the liveness rule is ours because the relay sends no heartbeat. Adopting a library would mean fighting it on every one of those points.

The relay's own ingress makes the same move in the other direction, and for the same reason: it keys on what the payload carries rather than on a header it would have to trust.

## Considered Options

- **`r3labs/sse` and similar clients**: they handle reconnection, `Last-Event-ID`, and typed events. Rejected — every one of those features is either wrong here (resume from a counter that restarts), or already decided differently (the reconnect and liveness policy), and adopting one would put the only piece of protocol logic the project actually needs behind an abstraction that does not match the wire.
- **Reading the whole body and parsing with `encoding/json`**: impossible on a held-open stream, which by definition never ends.

## Consequences

The parser is the CLI's only protocol code, and the only place a change to the relay's frame format can break it. It is covered through the process seam with frames captured from the real relay, and the gap that leaves — a frozen fixture format that cannot notice the wire drifting — is closed by the manual runbook and the Phase 3 demo, not by more tests.

The `id:` counter is ignored rather than used, and no cursor is persisted between runs. A reconnect therefore cannot recover what happened while the CLI was down. `feed history` is the answer to that, and the spec says so instead of pretending the live view is continuous.

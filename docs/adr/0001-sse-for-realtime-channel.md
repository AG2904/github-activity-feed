# SSE for the CLI realtime channel

Traffic on the realtime channel flows one way: the relay emits ActivityEvents, the CLI displays them, and the CLI never sends anything back upstream. We use Server-Sent Events (`text/event-stream`) instead of WebSockets, because SSE is plain HTTP that survives corporate proxies, `EventSource` reconnects automatically with no client-side reconnect logic, and the Go client needs no WebSocket library. NestJS exposes SSE natively through its `@Sse` decorator.

## Considered Options

- **WebSocket**: bidirectional, but we have no upstream traffic to justify it, and it would cost a WebSocket library in Go, hand-rolled reconnection, and connection state on the relay.

## Consequences

The channel is an unauthenticated broadcast: every connected CLI receives every ActivityEvent and filters locally. Routing events per developer would require identity and authentication on the CLI, and is deliberately deferred. Revisit only if the team grows past a handful of developers.

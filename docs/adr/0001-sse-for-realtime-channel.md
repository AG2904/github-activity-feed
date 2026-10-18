# SSE for the CLI realtime channel

Traffic on the realtime channel flows one way: the relay emits ActivityEvents, the CLI displays them, and the CLI never sends anything back upstream. We use Server-Sent Events (`text/event-stream`) instead of WebSockets, because SSE is plain HTTP that survives corporate proxies, `EventSource` reconnects automatically with no client-side reconnect logic, and the Go client needs no WebSocket library. NestJS exposes SSE natively through its `@Sse` decorator.

## Considered Options

- **WebSocket**: bidirectional, but we have no upstream traffic to justify it, and it would cost a WebSocket library in Go, hand-rolled reconnection, and connection state on the relay.

## Consequences

The channel is an unauthenticated broadcast: every connected client receives every ActivityEvent and filters locally. Routing events per developer would require identity and authentication on the client, and is deliberately deferred.

The broadcast holds while the relay watches one team's repositories. It stops being affordable when either of these becomes true, and this is the trigger to revisit — not a headcount:

- the relay is pointed at repositories beyond a single team's, so most events reaching a developer concern someone else's work; or
- every client is receiving more than roughly 50 critical events a day, at which point the feed is a firehose rather than "what needs my attention".

Both are checkable from the audit history, and both are deliberately concrete: "past a handful of developers" was not. The 50 is a chosen threshold, not a measured one — revise it if experience disagrees.

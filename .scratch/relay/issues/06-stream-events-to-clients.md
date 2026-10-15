# 06: Events stream to connected clients

**What to build:** A client can hold a connection open and receive every stored event as it happens, over plain HTTP, without polling.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] A stream endpoint holds the connection open and emits events as they are stored.
- [ ] An event posted while a client is connected arrives on that connection.
- [ ] Every connected client receives every event.
- [ ] The stream travels over plain HTTP and needs no handshake beyond the request itself.

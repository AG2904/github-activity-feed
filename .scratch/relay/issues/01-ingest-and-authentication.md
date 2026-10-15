# 01: Ingest and authentication (walking skeleton)

**What to build:** The relay starts and accepts a GitLab webhook over HTTP, refusing any request that does not carry the correct shared secret and acknowledging those that do. No event survives yet — nothing is filtered, normalized, or stored. This ticket exists to stand the walking skeleton up: the app, the persistence wiring, the HTTP seam, and the test harness, so every later ticket has somewhere to land.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] The app boots against a temporary SQLite file with no external services.
- [ ] A single endpoint accepts GitLab webhooks over HTTP POST.
- [ ] A request with a missing or wrong token is rejected with `401`.
- [ ] A request with the correct token is acknowledged with a success status.
- [ ] The secret is read from the environment, never from committed code.
- [ ] Token comparison does not short-circuit on the first differing byte.
- [ ] A history endpoint exists and returns an empty collection before anything is stored.
- [ ] An integration test drives the endpoint over HTTP, not by calling a handler directly.

# 01: Ingest and authentication (walking skeleton)

**What to build:** The relay starts and accepts a GitHub webhook over HTTP, verifying the `X-Hub-Signature-256` HMAC of the raw request body and refusing any request whose signature is missing or does not match the shared secret. No event survives yet — nothing is filtered, normalized, or stored. This ticket stands the walking skeleton up: the app, the persistence wiring, the HTTP seam, and the test harness, so every later ticket has somewhere to land.

**Blocked by:** None (can start immediately)

**Status:** done

- [x] The app boots against a temporary SQLite file with no external services.
- [x] A single HTTP POST endpoint stands at the seam where GitHub webhooks arrive.
- [x] A request without a signature, with a malformed signature, or signed with the wrong secret is rejected with `401`.
- [x] A request whose body does not match its signature (tampered body) is rejected with `401`.
- [x] A request carrying a valid HMAC-SHA256 signature over the raw body is acknowledged with a success status.
- [x] The secret is read from the environment, never from committed code.
- [x] The signature comparison does not short-circuit on the first differing byte.
- [x] A history endpoint exists and returns an empty collection before anything is stored.
- [x] An integration test drives the endpoint over HTTP, not by calling a handler directly.

## Comments

- 2026-10-15: Rewritten for the GitHub migration (see `docs/research/github-notifications-vs-relay.md` and §9 of `ARCHITECTURE.md`). Authentication changes from GitLab's plaintext `X-Gitlab-Token` header to GitHub's `X-Hub-Signature-256`: HMAC-SHA256 of the raw request body, verified in constant time. The walking skeleton merged in PR #1 still validates the old header; the remaining work on this ticket is that migration. Boot, seam, history, and the HTTP-only test harness criteria remain satisfied from PR #1.
- 2026-10-15: HMAC migration completed (`rawBody` capture + constant-time verification, 8 e2e cases covering wrong secret, tampered body, malformed digest, and raw-byte signing). Env secret renamed to `GITHUB_WEBHOOK_SECRET`. Status `ready-for-human` per the canonical triage labels — implementation done, human review/merge pending.

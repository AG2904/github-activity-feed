# Webhook authentication via an HMAC signature

The relay's webhook endpoint must reject forged events, because everything it accepts is persisted into the audit history. We verify GitHub's `X-Hub-Signature-256` header — HMAC-SHA256 of the raw request body, keyed by a secret read from the environment — and return `401` on a missing, malformed, or mismatched signature.

The body is verified as the exact bytes GitHub signed, never as a re-serialization of the parsed object; that is why the app bootstraps with `rawBody: true`. The comparison is constant-time over the fixed-length hex digests, so it neither short-circuits on the first differing byte nor leaks the secret's length. Every incomplete input fails closed: absent body, absent header, wrong prefix, empty secret. The secret lives only in the environment, and the app refuses to boot without it.

Verification also happens **before** the body is parsed. The app is created with `bodyParser: false` and registers a single raw-body parser (`src/relay/ingress.ts`), so no framework parser can answer for a body that the signature check has not yet seen. Nest's JSON parser would answer `400` for unparseable input, and its `strict` mode rejects even a perfectly valid JSON scalar — in both cases reporting a failed delivery to GitHub for a webhook the relay is supposed to acknowledge and discard. The controller reads the bytes, verifies them, and only then parses.

## Considered Options

- **A plaintext shared-secret header** (`X-Gitlab-Token`): what the relay used before the GitHub migration, and the reason this ADR was rewritten. The header carried the raw secret, so it was only safe over TLS and a leaked header was a leaked secret — GitLab shipped no signature (signing tokens only arrive in 19.0), which made that the ceiling rather than a choice. GitHub offers HMAC-SHA256 directly, which binds the secret to the body, so the stronger option was available and taken.
- **Keeping Nest's JSON parser and catching its parse errors downstream** (`entity.parse.failed` → acknowledge): rejected. It leaves the failure mode in the framework's hands, keys the relay on an express error shape, and makes what the relay answers depend on which parser ran. Reading the bytes itself keeps one ingress path — verify, then parse — and keeps the handler's own tolerance for unreadable input reachable.

## Consequences

The two schemes are not interchangeable, and the environment variable is named for GitHub (`GITHUB_WEBHOOK_SECRET`), so serving the GitLab-era deployment again would mean re-introducing the plaintext path. The migration between them is narrated in the ticket 01 record.

Because the framework's parsers are gone, `req.body` is a `Buffer` on this endpoint and the request payload is never a pre-parsed object; the controller is the only place that turns bytes into a payload. The options that produce this wiring are exported as `INGRESS_OPTIONS`/`configureIngress` and shared with the e2e harness, so the two cannot drift.

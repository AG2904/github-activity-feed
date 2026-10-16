# Webhook authentication via an HMAC signature

The relay's webhook endpoint must reject forged events, because everything it accepts is persisted into the audit history. We verify GitHub's `X-Hub-Signature-256` header — HMAC-SHA256 of the raw request body, keyed by a secret read from the environment — and return `401` on a missing, malformed, or mismatched signature.

The body is verified as the exact bytes GitHub signed, never as a re-serialization of the parsed object; that is why the app bootstraps with `rawBody: true`. The comparison is constant-time over the fixed-length hex digests, so it neither short-circuits on the first differing byte nor leaks the secret's length. Every incomplete input fails closed: absent body, absent header, wrong prefix, empty secret. The secret lives only in the environment, and the app refuses to boot without it.

## Considered Options

- **A plaintext shared-secret header** (`X-Gitlab-Token`): what the relay used before the GitHub migration, and the reason this ADR was rewritten. The header carried the raw secret, so it was only safe over TLS and a leaked header was a leaked secret — GitLab shipped no signature (signing tokens only arrive in 19.0), which made that the ceiling rather than a choice. GitHub offers HMAC-SHA256 directly, which binds the secret to the body, so the stronger option was available and taken.

## Consequences

The two schemes are not interchangeable, and the environment variable is named for GitHub (`GITHUB_WEBHOOK_SECRET`), so serving the GitLab-era deployment again would mean re-introducing the plaintext path. The migration between them is narrated in the ticket 01 record.

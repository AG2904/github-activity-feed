# Webhook authentication via the GitLab secret token

The relay's webhook endpoint must reject forged events, because everything it accepts is persisted into the audit history. We validate the `X-Gitlab-Token` header against a shared secret read from the environment, and return `401` on a missing or mismatched value.

This ceiling is GitLab's, not ours: GitLab 16–18 ships no HMAC signature for webhooks (signing tokens only arrive in 19.0). The header carries the raw secret in plaintext, so it is only safe over TLS, and the endpoint must not be reachable from outside the internal network. Validation is a constant-time comparison.

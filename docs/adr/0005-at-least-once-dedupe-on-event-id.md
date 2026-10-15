# At-least-once webhook delivery: dedupe on the GitLab event id

GitLab webhook delivery is at-least-once. A timeout or a manual resend re-delivers the same event, reusing the same `Idempotency-Key` (GitLab 17.4+) or `X-Gitlab-Event-UUID` on older instances. The asymmetry that makes this easy to get wrong: GitLab does *not* auto-retry failures — the webhook worker rescues HTTP errors — so the duplicates that arrive are timeouts, not retries.

We persist the GitLab event identifier on each ActivityEvent row under a unique index, and drop any incoming event whose identifier already exists.

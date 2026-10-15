# 02: A merge into `main` becomes an event

**What to build:** A pull request that GitHub reports as closed-and-merged into `main` is stored as an `ActivityEvent` and can be read back over HTTP. This is the first real path through the relay: filter, normalize, persist, read.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] A PR reported as closed and merged with base branch `main` is stored as an event.
- [ ] Merge detection keys on the merge state GitHub reports (`action: closed` plus `pull_request.merged`), never on intermediate payload detail.
- [ ] The stored event carries type, actor, subject, assignee, labels, URL, and timestamp.
- [ ] Labels are stored as plain strings, not as GitHub's label objects.
- [ ] The event is readable from the history endpoint in the same shape it is stored.
- [ ] Timestamps are stored in one canonical form (UTC ISO-8601) regardless of the sender's timezone.

## Comments

- 2026-10-15: Rewritten for the GitHub migration (see `docs/research/github-notifications-vs-relay.md` and §9 of `gitlab_feed_architecture.md`). The filter moves from GitLab's merge action + target branch to GitHub's `pull_request` payload: `action == "closed"`, `pull_request.merged == true`, `pull_request.base.ref == "main"`. Actor is `sender.login`. Ticket 01 (HMAC auth) is merged; storage (entity + persistence) is the new work here.

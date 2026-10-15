# 02: A merge into `main` becomes an event

**What to build:** A merge request that GitLab reports as merged into `main` becomes a stored event and can be read back over HTTP. This is the first real path through the relay: filter, normalize, persist, read.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] A merge request merged into `main` is stored as an event.
- [ ] Merge detection keys on the action GitLab reports, not on the `changes` object.
- [ ] The stored event carries type, actor, subject, assignee, labels, URL, and timestamp.
- [ ] Labels are stored as plain strings, not as GitLab's label objects.
- [ ] The event is readable from the history endpoint in the same shape it is stored.
- [ ] Timestamps are stored in one canonical form regardless of the sender's timezone.

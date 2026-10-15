# 05: A resent event is stored once

**What to build:** GitLab delivery is at-least-once, so the same event can arrive twice. The relay stores it exactly once.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] The GitLab event identifier is stored with each event under a unique constraint.
- [ ] Posting the same event twice stores a single row.
- [ ] The duplicate is still acknowledged with a success status.
- [ ] Two different events posted back to back both survive.

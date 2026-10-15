# 03: An issue assignment with a critical label becomes an event

**What to build:** An issue that gains an assignee while carrying a `bug` or `feedback` label becomes a stored event — whether the assignee is added later or set at the moment of creation.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] An issue update that adds an assignee while carrying `bug` is stored.
- [ ] An issue created with an assignee and carrying `feedback` is stored.
- [ ] The stored event names both the actor (who assigned) and the assignee (who received).
- [ ] Assignment is detected from the change in the assignee set, not from the action alone.

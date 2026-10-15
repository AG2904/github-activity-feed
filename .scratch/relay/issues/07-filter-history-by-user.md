# 07: The history is filtered by user

**What to build:** A team member can ask the history endpoint for the events involving a given user, rather than reading the whole record.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] The history endpoint accepts a user filter.
- [ ] With events from two different actors stored, filtering by one returns only that one's.
- [ ] The filtered response uses the same event shape as the unfiltered one and the stream.
- [ ] An unmatched user returns an empty collection, not an error.

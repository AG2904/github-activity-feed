# 05: The CLI filters for itself

**What to build:** Local filtering, so each developer narrows the relay's broadcast to what concerns them — the client-side half of the broadcast model.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] `--user <login>` passes an event where the user is the actor, passes an event where the user is the assignee, and drops everything else.
- [ ] `--repo <owner/name>` passes only events whose repository matches, and drops everything else.
- [ ] The repository is derived from the event's `url` — its first two path segments — in one place, so that a field on the relay could replace the derivation without touching the parser, the renderer, or the filters.
- [ ] `--user` and `--repo` combine as a conjunction.
- [ ] Filtering is local: the CLI still receives every event the relay broadcasts, and a filtered-out event produces no output at all.

## Comments

- 2026-10-17: `--user` matches actor or assignee because that is the rule the relay applies to `/history?user=`, and the two must not disagree about what a user's events are. The repository derivation is the accepted cost of not extending the shape (ADR-0006); it lives here because this is its first use, and ticket 07 reuses it.

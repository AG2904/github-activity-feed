# 10: An assignment at creation is stored once

**What to build:** An issue created already assigned, carrying a `bug` or `feedback` label, becomes exactly one stored event - read from the `assigned` delivery GitHub sends for it, never from the `opened` delivery that reports the same creation.

**Blocked by:** 03

**Status:** done

- [x] An `opened` delivery for an issue that arrives assigned with a critical label is not stored.
- [x] The `assigned` delivery GitHub sends for that same creation is stored.
- [x] The stored event keeps the actor (who assigned), the assignee, the labels, and the URL.
- [x] A real webhook run stores the creation once, from the three deliveries GitHub sends.

## Comments

- 2026-10-19: Found during a real webhook run against a Quick Tunnel. Creating an issue with `--label bug --assignee` produced three deliveries: `labeled`, `assigned`, `opened`. The relay stored `assigned` and `opened`, so the issue showed twice. The old rule - `opened` qualifies "when the issue arrives already assigned" - was written before the delivery sequence had been observed; the real one shows `assigned` covers the creation on its own. ADR-0012 records the decision. Re-ran the same scenario after the fix: the three deliveries still arrive, and `/history` and `feed watch` each show the issue once. Evidence: `docs/tickets/relay/10-an-assignment-at-creation-counts-once.md`.

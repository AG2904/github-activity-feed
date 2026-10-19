# An issue creation is not an assignment

GitHub reports an issue created already assigned as three separate deliveries: `labeled`, `assigned`, and `opened`. The `assigned` delivery is the assignment. The `opened` delivery reports the creation, and it carries the same assignee set, because the issue was created with someone already on it.

The relay used to treat both `assigned` and `opened` as assignment triggers, reading `opened` as qualifying only when the issue arrives already assigned. That does make a creation count the same as a later assignment - but it also stores the same happening twice, because they are two deliveries and the dedupe key is the delivery identifier (ADR-0005). A real run confirmed it: an issue created with a `bug` label and an assignee appeared twice in `feed watch` and twice in `/history`, with `updated_at` one second apart.

So `opened` is no longer a trigger. An assignment is the `assigned` action, and nothing else. The creation is not lost: GitHub sends `assigned` for it, which is the delivery reporting the assignee set going from empty to someone.

## Considered Options

- **`assigned`, and `opened` when the issue arrives assigned** (the previous rule): stores one happening twice. Rejected.
- **`assigned` only** (chosen): one trigger, one event, no new state. The creation is covered by its own `assigned` delivery, verified against a real webhook on 2026-10-19.
- **Dedupe on the normalized event within a time window** (same type, URL, and assignee within N seconds): keeps both triggers and collapses the pair. Rejected for the reason ADR-0005 already rejected the event hash - it cannot tell this duplicate from a legitimate re-assignment of the same issue to the same person moments later, and silently dropping a real event is worse than storing a duplicate. A window only shrinks the odds; it does not answer the question.

## Consequences

The rule now depends on GitHub emitting `assigned` for a creation that arrives assigned. It does, and one delivery is enough; if that ever changed, the creation would be missed rather than duplicated. The failure mode moved from noise to silence, which is why the ticket that changed this carries a real captured payload as its evidence.

`toAssignmentEvent` no longer reads `created_at`: every assignment it sees is an `assigned` delivery, timestamped by the issue's `updated_at`.

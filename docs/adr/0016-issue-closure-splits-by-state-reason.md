# An issue closure is two events, not one more field

A closed issue carrying `bug` or `feedback` now becomes an ActivityEvent, and the
reason GitHub reports for the closure splits it in two: `issue.closed` for
`completed` (or no reason at all), `issue.dismissed` for `not_planned`. The
product decision behind it is that "we fixed it" and "we decided not to" are
different news, and a feed that folds them together is telling the team less than
GitHub already knows.

The shape does not grow a tenth field. `state_reason` is real information, but
`ActivityEvent`'s nine fields are a contract with three consumers already —
storage, the stream, the CLI — and ADR-0005 deliberately pins the shape with
deep-equality assertions so that adding a field is a decision, not a reflex.
Growing it here would also mean deciding what the field means for a merge or an
assignment (`null`? absent?), storing it on every row, and teaching every client
to read a field only one type uses. `type` is already the closed set of literals
that says *what happened*, and a dismissal is a different happening from a
resolution, not the same happening with an attribute.

## Considered Options

- **One `issue.closed` type plus a tenth `stateReason` field**: rejected. It is
  the conceptually purest model — one happening, one attribute — but it pays for
  that purity with the shape's defining invariant, a migration-shaped change to
  the stored rows, and a field every other event type has to answer for. The
  distinction is a difference in what happened, and the type already exists to
  carry exactly that.
- **One `issue.closed` type with the reason folded into `labels`**: rejected.
  Labels come from GitHub verbatim and the CLI prints them as the reason the
  event exists. Inventing one would make the feed lie about the issue's labels.
- **Only `completed` closures**: rejected by the product decision. A dismissed
  bug is exactly the moment the team stops needing to think about it, and hiding
  it leaves a watched issue looking open forever.
- **A single `issue.closed` for both, the card saying nothing about the reason**:
  rejected for the same reason: the relay knows the difference and would be
  throwing it away.
- **Keep ignoring closures**: rejected. The feed reported the beginning of the
  work and never its end, which is half an audit trail.

## Consequences

The merge adapter already keys on `action: "closed"`, but on a `pull_request`
object; an `issues` delivery carries none, so the two cannot collide. The closure
adapter is a third step in the one-pass composition, and the `malformed` rule is
terminal for it as it is for the other two (ADR-0015).

The closure carries the issue's assignee, read from the issue rather than from
the delivery's top-level `assignee`. That makes the event reachable through
`--user` for the person the issue was on, which is the person the closure is news
to; it does not change what `assignee` means elsewhere.

`when` is `issue.updated_at`, because GitHub stamps the closure on the issue and
the payload carries no closer timestamp. The label gate reads `issue.labels` at
that moment, exactly as the assignment gate does: closing a routine task produces
nothing.

The wire accepts the two new literals without any version negotiation. A client
built before this change already degrades an unknown type to its raw value rather
than dropping the card, so an old binary shows `issue.dismissed` in place of the
sentence — readable, if not styled — and the new binary shows the sentence. The
stored history is untouched: previous rows keep their two literals, no column is
added, and the dedupe index is not involved.

The CLI's session account grows two counters, `closed` and `dismissed`, because
it exists to answer "what did I see" and a type it silently omits would make the
line wrong.

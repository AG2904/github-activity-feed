# 01: A closed critical issue becomes an event, split by why it closed

**What to build:** GitHub reports the closure of an issue as `action: "closed"`
on an `issues` delivery, with `issue.state_reason` naming the outcome. Today the
relay reads an `issues` payload only for `assigned`, so a bug that gets closed —
the moment the work stops needing attention — leaves no trace in the feed or the
history. Add the closure as a third adapter beside the merge and the assignment,
gated on the same `bug` / `feedback` labels, and keep the two outcomes apart in
the wire contract: `issue.closed` for a closure reported as `completed` (or with
no reason at all), `issue.dismissed` for one reported as `not_planned`.

The nine-field shape does not move (ADR-0005). The reason of the closure lives in
`type`, which is already a closed set of literals, rather than in a tenth field
that would break every deep-equality assertion and the stored rows. No
migration: the entity already stores `type` as text.

**Status:** done

- [x] A delivery with `action: "closed"` whose issue carries `bug` or `feedback`
      stores an event; one without a critical label stores nothing.
- [x] `issue.state_reason: "completed"`, `null` or absent yields `issue.closed`;
      `"not_planned"` yields `issue.dismissed`; an unrecognised value yields
      `issue.closed`.
- [x] The event carries repository, `branch: null`, actor from `sender.login`,
      subject, the issue's assignee (or `null`), the flattened labels, the issue
      URL, and `issue.updated_at` as its canonical instant.
- [x] A closure that passes the filter but cannot be normalized is `malformed`,
      and the merge-then-assignment-then-closure composition stops there.
- [x] `ActivityEventType` names the four literals; no field is added and no
      stored row or served history changes for the existing payloads.
- [x] The end-to-end suite covers: closed + `bug` (completed), closed + `feedback`
      (not planned), closed without a critical label, closed with a critical
      label but nobody to attribute it to, and the existing merge and assignment
      cases still green with no edit.
- [x] `npm test`, `npm run build` and `npm run lint` are green.
- [x] `GLOSSARY.md`, `docs/relay-flow.md`, `ARCHITECTURE.md` and a new ADR
      describe the two types and why the outcome is in `type`.

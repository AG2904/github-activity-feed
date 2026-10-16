# 07: The history is filtered by user

**What to build:** A team member can ask the history endpoint for the events involving a given user, rather than reading the whole record.

**Blocked by:** 02

**Status:** ready-for-human

- [x] The history endpoint accepts a user filter.
- [x] With events from two different actors stored, filtering by one returns only that one's.
- [x] The filtered response uses the same event shape as the unfiltered one and the stream.
- [x] An unmatched user returns an empty collection, not an error.

## Comments

- 2026-10-16: Completed (48 e2e cases, 5 new). One commit on `main`: `feat: filter the audit history by the user involved (ticket 07)`.

  **"Involving" resolved to actor OR assignee.** The ticket's question — what makes a user involved in an event — is answered by story 15: *"reconstruct what a teammate merged or was assigned"*, plus the glossary, which defines **Actor** as "who merged the pull request, or who assigned the issue" and **Assignee** as "the person an assignment event is about". So the filter is `where: [{ actor: user }, { assignee: user }]`. This is deliberately **wider than story 15's sentence**, which does not name the person who *assigns*: filtering by `lucia.gomez` returns the assignment she made. Pinned by test, so it is a decision rather than an accident.

  **The match is exact and case-sensitive, and that is left unfixed on purpose.** GitHub treats logins as case-insensitive, so `?user=Maria.Perez` finds nothing where `maria.perez` has events — indistinguishable from "this user has no events". No spec line requires or forbids case-insensitive matching (`Normalization` only says usernames are used), so it is an open product decision rather than a ticket deliverable: no test freezes the behaviour in either direction. Making it case-insensitive is a small change to the query plus its test.

  Two things this ticket also corrected, both recorded in the evidence document: the "empty filter means no filter" claim in the controller's JSDoc is now backed by a test (the same class of unbacked comment the ticket 06 review caught), and the test vocabulary consolidated onto one `postDelivery` helper — reversing the ticket 05 decision to keep two spellings apart, which the third variant made stop paying. The order the history is served in was untested until now, and this change touches the very query that carries it, so the filtered case pins it too.

  Status `ready-for-human` per the canonical triage labels. The evidence record is at `docs/tickets/relay/07-filter-history-by-user.md`.

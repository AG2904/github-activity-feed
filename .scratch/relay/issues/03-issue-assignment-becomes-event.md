# 03: An issue assignment with a critical label becomes an event

**What to build:** An issue that gains an assignee while carrying a `bug` or `feedback` label becomes a stored event — whether the assignee is added later or set at the moment of creation.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] An issue update that adds an assignee while carrying `bug` is stored.
- [ ] An issue created with an assignee and carrying `feedback` is stored.
- [ ] The stored event names both the actor (who assigned) and the assignee (who received).
- [ ] Assignment is detected from the assignee set the payload reports, never from the action alone: an update that adds an assignee and a creation that already carries one both qualify.

## Comments

- 2026-10-15: The fourth criterion was rewritten. It previously read "detected from the change in the assignee set, not from the action alone" — which is GitLab-era language and is **unsatisfiable on GitHub**. GitHub's `issues` payload carries no `changes` object at all (confirmed against the webhook event reference and the canonical payloads for `assigned` and `opened`), so there is no previous-assignee set to diff against and no before/after to compare. What the payload does give is the current set in `issue.assignees`, plus the action. Both triggers the ticket asks for are therefore read the same way: the action that adds an assignee, and a creation reported with a non-empty assignee set. Note also that `opened` carries no top-level `assignee` — the assigned user has to be read from `issue.assignees`.
- 2026-10-15: This is the ticket that forces the event model to accommodate a second kind. The domain-modeling session settled the shape it has to fit (see the spec's normalization decision and ADR-0005): `type` becomes a closed set of literals, `assignee` becomes `null` instead of an empty string for an event that has none, and the GitHub payload stays inside the HTTP adapter rather than leaking into the domain.

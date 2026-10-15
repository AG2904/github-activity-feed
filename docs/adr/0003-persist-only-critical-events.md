# Persist only normalized critical events

The relay stores only the ActivityEvents it considers critical — an MR merged to `main`, or an issue assigned while carrying a `bug` or `feedback` label — and not the raw GitLab payload of every incoming webhook.

Raw payloads would retain assignee email addresses and other PII we have no reason to keep, and they would turn the audit history into noise. The architecture document commits to auditing only events "of critical operational value", and the `feed --user` query only ever asks for those. Events that fail the filter are dropped, not archived.

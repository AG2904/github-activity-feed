# Relay — Critical Activity Events

Status: ready-for-agent

## Problem Statement

A small development team works on a corporate GitHub instance. Two things regularly need their attention: code landing on `main`, and a `bug` or `feedback` issue being assigned to someone. Today they only find out by checking GitHub's web UI or reading email, both of which pull them out of the terminal and break focus. GitHub's native notifications cannot be narrowed to just those two events, so the alternative is muting them entirely.

## Solution

A relay that runs inside the corporate network, as a plain Node process on SQLite — no container, no external database. It accepts GitHub webhooks, keeps only the two critical events, normalizes them into a single event shape, persists them for audit, and streams them to connected clients over Server-Sent Events. It also serves the stored history, so a team member can reconstruct what happened after the fact.

## User Stories

1. As a developer, I want a merge into `main` to become a stored event, so that integrations into our main branch are auditable.
2. As a developer, I want a merge into any branch other than `main` to be ignored, so that routine branch merges don't pollute the feed.
3. As a developer, I want a pull request that is opened, closed, reopened, or edited to be ignored, so that only actual integrations appear.
4. As a developer, I want an issue assigned while carrying a `bug` or `feedback` label to become a stored event, so that urgent work reaches me.
5. As a developer, I want an issue assigned at the moment of creation to count the same as a later assignment, so that no assignment is silently missed.
6. As a developer, I want an issue assigned without a `bug` or `feedback` label to be ignored, so that routine task assignment doesn't interrupt me.
7. As a developer, I want the relay to reject a webhook whose signature doesn't match our secret, so that nobody can inject fabricated events into our history.
8. As a developer, I want the same event delivered twice to be stored only once, so that the audit history isn't padded with duplicates.
9. As a developer, I want the relay to acknowledge a webhook promptly, so that GitHub doesn't record the delivery as a failure.
10. As a developer, I want each stored event to carry its type, repository, source branch, actor, subject, assignee, labels, URL, and timestamp, so that I can tell what happened without opening GitHub.
11. As a developer, I want to hold open a stream connection and receive events as they happen, so that my client never polls.
12. As a developer, I want that stream to travel over plain HTTP, so that corporate proxies and TLS-inspecting middleboxes don't break it.
13. As a developer, I want a dropped stream connection to recover without intervention, so that I don't restart my client.
14. As a developer, I want every stored event broadcast to every connected client, so that the relay needs no per-developer identity or login.
15. As a developer, I want to query the stored history filtered by user, so that I can reconstruct what a teammate merged or was assigned.
16. As a developer, I want the history query to return the same event shape as the stream, so that my client parses only one thing.
17. As a team lead, I want the relay reachable only from inside the corporate network, so that our metadata never leaves the perimeter.
18. As a team lead, I want raw webhook payloads discarded after processing, so that assignee email addresses and similar personal data are never retained.
19. As a team lead, I want the webhook secret supplied through the environment, so that no secret is ever committed.
20. As a team lead, I want the relay to run as a native process with an embedded database file, so that it fits what TI is willing to approve.
21. As a developer, I want the history to survive a restart, so that the audit record outlives any redeploy.
22. As a developer, I want timestamps recorded in one canonical form, so that ordering is unambiguous regardless of who is in which timezone.
23. As a developer, I want each event to include a link to the pull request or issue it describes, so that I can jump from the terminal straight to GitHub.
24. As a developer, I want an unrecognized or malformed webhook to be ignored without error, so that turning on an unrelated GitHub integration can't break the relay.
25. As a developer, I want to query the stored history filtered by repository, so that I can read one project's record on its own.
26. As a developer, I want an issue closed while carrying a `bug` or `feedback` label to become a stored event, so that the end of urgent work is as visible as its beginning.
27. As a developer, I want a closure GitHub reports as `not_planned` to be a different event from one it reports as resolved, so that a discarded issue and a fixed one are never confused.

## Implementation Decisions

- **Ingress**: a single HTTP endpoint accepts every GitHub hook. GitHub names the event in the `X-GitHub-Event` header, but the relay keys on what the payload itself carries — a `pull_request` object, or an `issues` object — so a hook we don't understand is ignored rather than dispatched into a branch we didn't anticipate.
- **Authentication**: the `X-Hub-Signature-256` header carries an HMAC-SHA256 of the raw request body, keyed by a secret from the environment and compared in constant time; a missing, malformed, or mismatched signature returns `401`. The body is verified as the bytes GitHub signed, never as a re-serialization of the parsed payload (ADR-0002).
- **Filtering**: a merge is critical only when GitHub reports the pull request closed **and** merged and its base branch is `main`. Detection keys on the reported merge state (`action: closed` plus `pull_request.merged`), never on intermediate payload detail. An assignment is critical when GitHub reports the action that assigns — `assigned` — and the issue carries a `bug` or `feedback` label at that moment. A closure is critical when GitHub reports `closed` on an `issues` delivery and the issue carries one of those labels at that moment; its type is split by `issue.state_reason` — `completed` (or no reason) stores `issue.closed`, `not_planned` stores `issue.dismissed` — so a resolved issue and a discarded one are different news (ADR-0016). `opened` is deliberately not a trigger: GitHub delivers `assigned` and `opened` as two deliveries for an issue created already assigned, both reporting the same assignee set, so accepting `opened` stores one happening twice (ADR-0012). The creation is still covered by its `assigned` delivery, which is what makes an assignment at creation count the same as a later one. An action that merely touches an already-assigned issue — labeling it, editing it — is not an assignment and produces nothing, since reporting it would attribute to the labeler an assignment they never made. GitHub's payload carries no previous-assignee set, so a before/after diff is not available: the relay reads the current set from `issue.assignees`.
- **Normalization**: every surviving event is mapped to the project's single `ActivityEvent` shape: type, repository, branch, actor, subject, assignee, labels, URL, and timestamp. `type` is a closed set of literals, and `assignee` is `null` for an event that has none rather than an empty string. The repository is GitHub's `repository.full_name`, which every hook carries, so the relay reads it rather than recovering it from the URL (ADR-0010); it is `null` only for an event stored before that column existed, because inventing one from the URL is the derivation that was removed. The branch is the merge's source branch, GitHub's `pull_request.head.ref`, read from the payload rather than derived from the type (ADR-0013); it is `null` for an event stored before that column existed and for an event with no branch to name — an assignment, or a merge whose payload carries no usable head ref — and a missing or unusable head ref normalizes to `null` instead of dropping the event. GitHub's label objects are flattened to plain strings, and usernames are used for actor and assignee.
- **Idempotency**: GitHub's per-delivery identifier (`X-GitHub-Delivery`, stable across redelivery) is stored on the event's **row** under a unique constraint, and an incoming delivery whose identifier already exists is dropped as a duplicate. The identifier is a storage column and never a field of the `ActivityEvent` shape (ADR-0005).
- **Persistence**: events are stored through an ORM into an embedded SQLite file, one table, surviving restarts.
- **Realtime**: an SSE endpoint holds client connections open and broadcasts every stored event to all of them.
- **History**: a query endpoint returns stored events, filterable by a user's **Involvement** — the glossary's word for being an event's actor or its assignee — by repository, or by both, in the same normalized shape as the stream. The two are an AND rather than an OR: an array of clauses is an OR and the keys inside one clause are an AND, so a user's two roles get a clause each while the repository narrows every one of them.
- **Processing model**: handling is synchronous — validate, persist, respond — with no queue or broker. A local SQLite write is far below GitHub's 10-second delivery budget.
- **Response codes**: invalid signature returns `401`; a correctly authenticated but non-critical or unrecognized event returns a success code without persisting; a stored event returns a success code. A duplicate is indistinguishable from a fresh store in the response.

## Testing Decisions

- **One seam: the relay's HTTP boundary.** Tests boot the full application against a temporary SQLite file and interact with it exclusively over HTTP. This covers authentication, filtering, normalization, persistence, streaming, and history with a single seam.
- **What makes a good test here**: it posts a real GitHub payload and asserts on observable output — the HTTP status, the contents of the history query, or the bytes arriving on the stream connection. No test reaches into the adapter, the domain, or the ORM directly; those are implementation details and testing them would couple the tests to TypeORM.
- **Fixtures**: captured, real GitHub payloads, one per branch of the decision tree — a pull request merged into `main`; a pull request merged into another branch; a pull request opened or updated; an issue update that adds an assignee and carries `bug`; the `assigned` delivery GitHub sends for an issue created with an assignee and carrying `feedback`; the `opened` delivery for that same creation, which is ignored; an issue assigned without a critical label; a redelivery carrying an already-stored `X-GitHub-Delivery`; a payload with a bad signature.
- **Prior art**: none yet — this is the first code in the repository, so these tests establish the pattern the Go client's tests will later follow.
- **Streaming**: the stream is tested by opening the connection and asserting that a freshly posted event arrives on it, not by inspecting the broadcast mechanism.

## Out of Scope

- The Go client binary, its flags, and its terminal rendering (Phase 2).
- Deployment to the internal server and distributing the binary to the team (Phase 3).
- Per-developer routing or authentication on the stream — the channel is a broadcast by design.
- Retention or pruning of the audit history; events are kept indefinitely.
- Backfilling GitHub history that predates the webhook configuration.
- Choosing which GitHub repositories or organizations the relay listens to; that is configured on GitHub's side.
- Any acknowledgement or reply flowing from the client back to the relay.

## Further Notes

- Webhook delivery is at-least-once: a delivery that misses the 10-second response budget can be retried, and GitHub's own UI can redeliver one by hand. GitHub does not automatically redeliver failed deliveries, so a duplicate is not evidence of an automatic retry — and `X-GitHub-Delivery` stays the same across redelivery, which is exactly what makes it a sound dedupe key.
- A delivery that doesn't get a 2xx response within 10 seconds is recorded as a failure, and manual redelivery only covers the last three days. A slow or crashing endpoint therefore loses events for good, which is why the handler does nothing between validation and the SQLite write.
- The instance that will send the webhooks is **github.com**, confirmed on 24 October 2026 (validation ticket 04); no Enterprise Server is involved, so no corporate-network allow-listing applies.

## Comments

- 2026-10-15: Domain-modeling pass. GitHub terminology applied throughout the problem statement, solution, and implementation decisions. Two criteria were corrected against GitHub's actual payload rather than GitLab's:
  - Filtering no longer claims to read a `changes` object. GitHub's `issues` payload has no previous-assignee set at all, so "the assignee set changed" was rewritten as "the issue is reported with a non-empty assignee set", covering both a later assignment and an assignment at creation.
  - Idempotency now names `X-GitHub-Delivery` and states that the identifier is a row column, not a field of the `ActivityEvent` shape. The "stored per event" phrasing this replaces was ambiguous enough to invite putting it in the shape; ADR-0005 is the precise record.
  - Normalization now pins `assignee` to `null` when absent, replacing the empty string the first implementation used.
- 2026-10-19: `opened` stopped being an assignment trigger, after a real webhook run showed an issue created already assigned arriving as three deliveries (`labeled`, `assigned`, `opened`) and being stored twice. The creation is still covered, by its own `assigned` delivery. ADR-0012 records the decision and why the window-based dedupe was rejected.
- 2026-10-30: closures joined the critical set (stories 26–27, ADR-0016). An `issues` delivery with `action: closed` and a critical label now stores an event, split by `issue.state_reason` into `issue.closed` and `issue.dismissed` so that a resolution and a dismissal stay distinguishable. The nine-field shape is untouched: the reason travels in `type`, which was already the closed set of literals, so nothing is added to the stored rows or the served history. Fixtures added: a closure reported `completed`, one reported `not_planned` with nobody assigned, one with no reported reason, and one without a critical label.

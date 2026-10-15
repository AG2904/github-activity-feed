# Relay — Critical Activity Events

Status: ready-for-agent

## Problem Statement

Three developers work on a self-managed corporate GitLab. Two things regularly need their attention: code landing on `main`, and a `bug` or `feedback` issue being assigned to someone. Today they only find out by checking GitLab's web UI or reading email, both of which pull them out of the terminal and break focus. GitLab's native notifications cannot be narrowed to just those two events, so the alternative is muting them entirely.

## Solution

A relay that runs inside the corporate network, as a plain Node process on SQLite — no container, no external database. It accepts GitLab webhooks, keeps only the two critical events, normalizes them into a single event shape, persists them for audit, and streams them to connected clients over Server-Sent Events. It also serves the stored history, so a team member can reconstruct what happened after the fact.

## User Stories

1. As a developer, I want a merge into `main` to become a stored event, so that integrations into our main branch are auditable.
2. As a developer, I want a merge into any branch other than `main` to be ignored, so that routine branch merges don't pollute the feed.
3. As a developer, I want an MR that is opened, closed, reopened, or edited to be ignored, so that only actual integrations appear.
4. As a developer, I want an issue assigned while carrying a `bug` or `feedback` label to become a stored event, so that urgent work reaches me.
5. As a developer, I want an issue assigned at the moment of creation to count the same as a later assignment, so that no assignment is silently missed.
6. As a developer, I want an issue assigned without a `bug` or `feedback` label to be ignored, so that routine task assignment doesn't interrupt me.
7. As a developer, I want the relay to reject a webhook that doesn't carry the correct secret token, so that nobody can inject fabricated events into our history.
8. As a developer, I want the same event delivered twice to be stored only once, so that the audit history isn't padded with duplicates.
9. As a developer, I want the relay to acknowledge a webhook promptly, so that GitLab doesn't disable the integration after repeated timeouts.
10. As a developer, I want each stored event to carry its type, actor, subject, assignee, labels, URL, and timestamp, so that I can tell what happened without opening GitLab.
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
23. As a developer, I want each event to include a link to the MR or issue it describes, so that I can jump from the terminal straight to GitLab.
24. As a developer, I want an unrecognized or malformed webhook to be ignored without error, so that turning on an unrelated GitLab integration can't break the relay.

## Implementation Decisions

- **Ingress**: a single HTTP endpoint accepts every GitLab hook. Hooks are distinguished by the `X-Gitlab-Event` header, falling back to `object_kind` in the body.
- **Authentication**: the `X-Gitlab-Token` header is compared against a secret from the environment using a constant-time comparison. A missing or mismatched token returns `401`. GitLab 16–18 offers no HMAC signature, so a plaintext shared secret over TLS is the available ceiling.
- **Filtering**: a merge is critical only when GitLab reports the merge action and the target branch is `main`. Detection keys on the action field, never on the `changes` object, which GitLab may leave empty on merge events. An assignment is critical when the issue's assignee set changed — or was set at creation — and the issue carries a `bug` or `feedback` label at that moment.
- **Normalization**: every surviving event is mapped to the project's single `ActivityEvent` shape: type, actor, subject, assignee, labels, URL, and timestamp. GitLab's label objects are flattened to plain strings, and usernames are used for actor and assignee.
- **Idempotency**: the GitLab event identifier is stored per event under a unique constraint, and an incoming event whose identifier already exists is dropped as a duplicate.
- **Persistence**: events are stored through an ORM into an embedded SQLite file, one table, surviving restarts.
- **Realtime**: an SSE endpoint holds client connections open and broadcasts every stored event to all of them.
- **History**: a query endpoint returns stored events, filterable by user, in the same normalized shape as the stream.
- **Processing model**: handling is synchronous — validate, persist, respond — with no queue or broker. A local SQLite write is far below GitLab's delivery timeout.
- **Response codes**: invalid token returns `401`; a correctly authenticated but non-critical or unrecognized event returns a success code without persisting; a stored event returns a success code.

## Testing Decisions

- **One seam: the relay's HTTP boundary.** Tests boot the full application against a temporary SQLite file and interact with it exclusively over HTTP. This covers authentication, filtering, normalization, persistence, streaming, and history with a single seam.
- **What makes a good test here**: it posts a real GitLab payload and asserts on observable output — the HTTP status, the contents of the history query, or the bytes arriving on the stream connection. No test reaches into the parser, the filter, or the ORM directly; those are implementation details and testing them would couple the tests to TypeORM.
- **Fixtures**: captured, real GitLab payloads, one per branch of the decision tree — an MR merged into `main`; an MR merged into another branch; an MR opened or updated; an issue update that adds an assignee and carries `bug`; an issue created with an assignee and carrying `feedback`; an issue assigned without a critical label; a resend of an already-stored event carrying the same identifier; a payload with a bad token.
- **Prior art**: none yet — this is the first code in the repository, so these tests establish the pattern the Go client's tests will later follow.
- **Streaming**: the stream is tested by opening the connection and asserting that a freshly posted event arrives on it, not by inspecting the broadcast mechanism.

## Out of Scope

- The Go client binary, its flags, and its terminal rendering (Phase 2).
- Deployment to the internal server and distributing the binary to the team (Phase 3).
- Per-developer routing or authentication on the stream — the channel is a broadcast by design.
- Retention or pruning of the audit history; events are kept indefinitely.
- Backfilling GitLab history that predates the webhook configuration.
- Choosing which GitLab projects or groups the relay listens to; that is configured on GitLab's side.
- Any acknowledgement or reply flowing from the client back to the relay.

## Further Notes

- GitLab delivery is at-least-once: timeouts and manual resends produce duplicates. Counter-intuitively, GitLab does not retry failed webhooks on its own, so duplicates are not evidence of automatic retries.
- GitLab disables a webhook after four consecutive delivery failures, so a slow or crashing endpoint has a lasting cost.
- The instance that will send the webhooks is expected to be the corporate self-managed GitLab, but this must be confirmed before Phase 3.

# GitHub Activity Feed

The critical GitHub activity a development team sees in its terminal. This repo holds two contexts: the **relay** (a NestJS backend that receives GitHub webhooks) and the **CLI** (a Go binary developers run in their terminal).

## Language

**ActivityEvent**:
The single normalized shape the relay emits and the CLI renders, whatever GitHub hook it came from.
_Avoid_: notification, message, alert, payload

**Delivery**:
One HTTP POST from GitHub, carrying a payload. It is the unit GitHub retries and the unit the relay deduplicates on; it is not an ActivityEvent, because a delivery can be ignored and the same happening can arrive in two deliveries.
_Avoid_: request, event

**Relay**:
The internal backend that receives GitHub webhooks, normalizes them into ActivityEvents, persists them, and streams them to connected clients.
_Avoid_: server, backend, notifier, bot

**CLI**:
The Go binary a developer runs in their terminal to receive and display ActivityEvents.
_Avoid_: app, agent, daemon

**Client**:
A process that holds the stream open or reads the audit history. The CLI is one; `curl` is another, and for as long as the CLI did not exist it was the only one.
_Avoid_: consumer, subscriber, user

**Feed**:
What the CLI shows: the sequence of ActivityEvents a developer sees in their terminal. It is the same artifact whether the events arrived live or came from a query — the origin differs, not the thing.
_Note_: capitalized `Feed` is the concept; `feed` in code font is the binary.
_Avoid_: inbox, timeline, log, stream

**Critical event**:
A GitHub happening worth interrupting a developer for. Today that means a pull request merged into `main`, or an issue assigned while carrying a `bug` or `feedback` label.
_Avoid_: important event, alert, notification

**Merge to main**:
A pull request that GitHub reports as merged and whose base branch is `main`. Merges into any other branch are not critical events.
_Avoid_: merge, integration

**Assignment**:
An issue gaining an assignee, whether at creation or later. It is the trigger for the issue-side critical event, gated on the issue carrying a `bug` or `feedback` label at that moment.
_Note_: GitHub reports it as the `assigned` action in both cases. The `opened` delivery that accompanies a creation arriving already assigned reports the same happening, not a second one (ADR-0012).
_Avoid_: allocation, ownership

**Repository**:
The GitHub project an ActivityEvent happened in; the unit the CLI groups and filters by.
_Avoid_: project, codebase, origin

**Branch**:
The line of work a merge brought into `main`: the pull request's source branch, carried by the event when GitHub reports one. It is a location, not a person or a state, and it is supplementary — an event without one is still an event.
_Avoid_: ref, source, head

**Actor**:
The GitHub user who caused the event — who merged the pull request, or who assigned the issue.
_Avoid_: author, user, committer

**Assignee**:
The GitHub user an issue was assigned to; the person an assignment event is about.
_Avoid_: owner, responsible, target

**Involvement**:
A user's relation to an event, as its actor or its assignee. It is what "a user's events" means in every filter, and it introduces no third role: it is the union of the two the glossary already names.
_Avoid_: participation, ownership, membership

**Broadcast**:
The relay's delivery model. Every connected client receives every ActivityEvent, and each client filters for itself.
_Avoid_: push, fan-out, pub/sub, routing

**Audit history**:
The persisted record of ActivityEvents the relay keeps in SQLite and serves for retrospective queries such as `feed history --user`.
_Avoid_: log, archive, dump

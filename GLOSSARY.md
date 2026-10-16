# GitHub Activity Feed

Terminal notifications for a development team on GitHub. This repo holds two contexts: the **relay** (a NestJS backend that receives GitHub webhooks) and the **CLI** (a Go binary developers run in their terminal).

## Language

**ActivityEvent**:
The single normalized shape the relay emits and the CLI renders, whatever GitHub hook it came from.
_Avoid_: notification, message, alert, payload

**Relay**:
The internal backend that receives GitHub webhooks, normalizes them into ActivityEvents, persists them, and streams them to connected clients.
_Avoid_: server, backend, notifier, bot

**CLI**:
The Go binary a developer runs in their terminal to receive and display ActivityEvents.
_Avoid_: app, client, agent, daemon

**Critical event**:
A GitHub happening worth interrupting a developer for. Today that means a pull request merged into `main`, or an issue assigned while carrying a `bug` or `feedback` label.
_Avoid_: important event, alert, notification

**Merge to main**:
A pull request that GitHub reports as merged and whose base branch is `main`. Merges into any other branch are not critical events.
_Avoid_: merge, integration

**Assignment**:
An issue gaining an assignee, whether at creation or later. It is the trigger for the issue-side critical event, gated on the issue carrying a `bug` or `feedback` label at that moment.
_Avoid_: allocation, ownership

**Actor**:
The GitHub user who caused the event — who merged the pull request, or who assigned the issue.
_Avoid_: author, user, committer

**Assignee**:
The GitHub user an issue was assigned to; the person an assignment event is about.
_Avoid_: owner, responsible, target

**Broadcast**:
The relay's delivery model. Every connected CLI receives every ActivityEvent, and each CLI filters for itself.
_Avoid_: push, fan-out, pub/sub, routing

**Audit history**:
The persisted record of ActivityEvents the relay keeps in SQLite and serves for retrospective queries such as `feed --user`.
_Avoid_: log, archive, dump

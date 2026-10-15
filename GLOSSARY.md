# GitLab Activity Feed

Terminal notifications for a three-developer team on a self-managed GitLab. This repo holds two contexts: the **relay** (a NestJS backend that receives GitLab webhooks) and the **CLI** (a Go binary developers run in their terminal).

## Language

**ActivityEvent**:
The single normalized shape the relay emits and the CLI renders, whatever GitLab hook it came from.
_Avoid_: notification, message, alert, payload

**Relay**:
The internal backend that receives GitLab webhooks, normalizes them into ActivityEvents, persists them, and streams them to connected clients.
_Avoid_: server, backend, notifier, bot

**CLI**:
The Go binary a developer runs in their terminal to receive and display ActivityEvents.
_Avoid_: app, client, agent, daemon

**Critical event**:
A GitLab happening worth interrupting a developer for. Today that means an MR merged into `main`, or an issue assigned with a `bug` or `feedback` label.
_Avoid_: important event, alert, notification

**Merge to main**:
An MR that GitLab reports as merged and whose target branch is `main`. Merges into any other branch are not critical events.
_Avoid_: merge, integration

**Assignment**:
An issue gaining an assignee, whether at creation or later. It is the trigger for the issue-side critical event, gated on the issue carrying a `bug` or `feedback` label.
_Avoid_: allocation, ownership

**Actor**:
The GitLab user who caused the event — who merged the MR, or who assigned the issue.
_Avoid_: author, user, committer

**Assignee**:
The GitLab user an issue was assigned to; the person an assignment event is about.
_Avoid_: owner, responsible, target

**Broadcast**:
The relay's delivery model. Every connected CLI receives every ActivityEvent, and each CLI filters for itself.
_Avoid_: push, fan-out, pub/sub, routing

**Audit history**:
The persisted record of ActivityEvents the relay keeps in SQLite and serves for retrospective queries such as `feed --user`.
_Avoid_: log, archive, dump

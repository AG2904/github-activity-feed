# GitHub notifications vs. the relay — feasibility research

Question: if this team's workflow lived on GitHub instead of self-managed GitLab, could GitHub deliver exactly the two critical events — (1) an MR/PR merged into the default branch, and (2) an issue/PR assigned while carrying a `bug` or `feedback` label — and could that delivery work behind a firewall with no inbound ports?

Short answer: **native notifications cannot express the two critical events** (delivery-time filtering by branch, label, or merge state does not exist). **GitHub Actions can express both events exactly**, and the official Slack integration or an Actions webhook step can deliver them — but every push-style delivery either needs an internet-reachable endpoint or sends the metadata to a SaaS surface. **GitHub has no official pull mode for webhooks**; the pull alternatives (REST notifications API, REST events API, GraphQL polling) are outbound-only but per-user, lossy, or high-latency, and none provides a durable shared audit history.

All claims below are cited to docs.github.com or GitHub-owned first-party pages; see `## Sources`.

## Native notifications

### What GitHub native notifications can do

- Subscription granularity is: **one conversation** (issue/PR/gist thread), **all activity in a repository**, **CI activity**, or **per event type in a repository** — issues, pull requests, releases, security alerts, discussions ([about-notifications], [configuring-notifications]).
- The per-repository **"Custom" watch option** lets you tick event *types* ("Issues", "Pull Requests", ...). The docs' own example: selecting "Issues" subscribes you to "updates on every issue ... in the repository" ([configuring-notifications]).
- You are **auto-subscribed** by default when you are assigned to an issue or PR, open/comment on a thread, are @mentioned, or "changed the state of a thread, such as by closing an issue or merging a pull request" ([about-notifications]).
- Every notification carries a **`reason`** (e.g. `assign`, `mention`, `review_requested`, `state_change`, `subscribed`) and the inbox can filter by it, e.g. `reason:review-requested` ([about-notifications], [rest-notifications]).
- Delivery surfaces: web inbox, GitHub Mobile (which can push for "Assignments to issues or pull requests"), and email; email frequency/source options exist, and an org can restrict email to approved domains ([configuring-notifications]).

### Documented gaps vs. the two critical events

1. **No branch dimension anywhere.** Watch/custom settings select event *types*, not branches. There is no subscription option, and no `reason`, meaning "PR merged into the default branch". The closest native signal is `state_change`, documented as "You changed the thread state (for example, closing an issue or merging a pull request)" — i.e. it fires for *the actor who merged*, and it is thread-scoped, not repo- or branch-scoped ([rest-notifications]).
2. **No label gating of delivery.** There is no `label` reason in the `reason` table, and no option to receive assignment notifications only when the issue carries `bug`/`feedback` ([rest-notifications]). Being assigned auto-subscribes you to the thread — but so does every later comment on it, so you cannot get *only* the assignment ([about-notifications]).
3. **Filtering happens at triage time, not delivery time.** Custom inbox filters and `reason:` queries "focus" what you *see* in the inbox; delivery is still governed by watch/participation rules ([about-notifications]).
4. **Retention is short and personal.** Inbox notifications are kept 3 months unless marked Saved; they are per-user, not a shared team record ([about-notifications]).

**Verdict:** GitHub native notifications are a per-user, thread-centric inbox. Event 2 exists in dilated form (assignment notifies the assignee, and GitHub Mobile can push for it), but the `bug`/`feedback` gate is impossible; event 1 has no dedicated signal at all. Same shape of problem as the GitLab situation described in the relay spec — mute everything or accept the noise.

## Bots and integrations

### GitHub Actions workflows (the precise filter layer)

Actions triggers are webhook events with **activity types** and **branch filters**, plus payload conditionals:

- Event 1 exactly: `on: pull_request: types: [closed]` with `branches: [main]` and `if: github.event.pull_request.merged == true`. The docs give the `if_merged` conditional and the `branches` filter as first-party examples ([events-that-trigger-workflows]).
- Event 2 exactly: `on: issues: types: [assigned]` with an `if:` conditional inspecting `github.event.issue.labels`; the docs show event-payload conditionals (`github.event.*`) and that `issues` supports `assigned`, `labeled`, etc. ([events-that-trigger-workflows]). ("Assigned at creation" is visible in the payload's assignees/labels; verify the exact action sequence in a real payload before relying on it — this doc set does not spell it out.)
- Caveats: **not all webhook events trigger workflows**, and event-triggered workflows must exist on the default branch ([events-that-trigger-workflows]).

Delivery from a workflow is just outbound HTTP from the runner: `curl` to a Slack incoming webhook, a Teams/Power Automate webhook, or any internal endpoint. Constraints: a github.com-hosted runner can only reach things the internet can reach (so "post to our internal server" means an internet-exposed endpoint or self-hosted runners); every destination needs TI/corporate approval; secrets live in repo/environment secrets.

### Official GitHub integration for Slack (no code)

First-party README ([integrations-slack]):

- `/github subscribe org/repo` plus feature flags. `commits` = "New commits on the default branch (usually `main`)" — this is a real "code landed on main" notification; branch filters exist (`commits:main`, `commits:users/*`).
- `issues` covers "Opened or closed issues", `pulls` covers "New or merged pull requests". **Assignment is not a subscribable feature** — assignees only surface as *mentions* inside issue/PR notifications.
- **Label filters exist**: `+label:"bug"` sets a required label that filters issue/PR/comment/review events — but the README states the filter "does not apply" to Commit/Push or Branch events, and "Currently, we only support having one filter" (setting a new one replaces the old).
- Constraints: works on github.com and on GHES 3.8+ (a private GHES app is generated from the instance); the app requests GitHub permissions including "Write access to actions, issues, deployments, and pull requests"; for a GHES instance behind a proxy, the README documents **Slack Socket Mode**, i.e. the instance connects *outbound* to Slack — no inbound ports.

**Net:** the Slack app covers event 1 (default-branch commits, optionally merged-PR messages) out of the box, but **not** event 2's "assigned + labeled" conjunction — for that you need an Actions workflow posting to Slack, or a custom app.

### GitHub Apps (the "build your own bot" route)

- A GitHub App is an integration with **one webhook**, installed on orgs/accounts, whose **permissions determine which webhook events it can receive** ([creating-webhooks], [about-creating-github-apps]).
- The app's code "needs to run somewhere ... host your app on a server" ([about-creating-github-apps]) — i.e. you own a listening endpoint, exactly like the relay, with the same reachability problem.
- Admin friction: "By default, only organization owners can manage the settings of GitHub Apps in an organization" ([about-creating-github-apps]).

### Email

Native email notifications can reach you for participation/assignment, and orgs can restrict sending to approved domains ([configuring-notifications]). No branch or label gating; per-user, and it is the email surface the relay spec explicitly wants to avoid.

### Third-party marketplace bots

GitHub's own webhook docs describe the general pattern — webhooks "sending notifications about events on GitHub to collaboration platforms ... Discord or Slack" — and note GitHub Services (the old built-in hook catalog) is retired in favor of raw webhooks ([about-webhooks]). Any marketplace bot ultimately receives GitHub webhooks, so it inherits the same inbound-endpoint requirement unless it delivers to a SaaS you already use.

## Push vs pull: firewall implications

### How webhook delivery actually works (push — GitHub calls you)

- "When an event that your webhook is subscribed to occurs, **GitHub will send an HTTP request with data about the event to the URL that you specified**" ([about-webhooks]). That is server-push: your endpoint must be reachable **from GitHub's egress IPs** — inbound reachability is mandatory.
- GitHub tells you to serve HTTPS with SSL verification enabled, optionally IP-allowlisting GitHub's published ranges (`GET /meta`), and to **respond 2XX within 10 seconds** or the delivery is a failure ([best-practices-webhooks]).
- **GitHub does not automatically redeliver failed deliveries**; redelivery is manual and only covers the past 3 days. `X-GitHub-Delivery` gives a stable per-delivery ID (unchanged on redelivery) for replay/duplicate protection ([redelivering-webhooks], [best-practices-webhooks]).
- **There is no official pull mode for webhooks.** The docs frame it as an either/or: "Webhooks are used to receive data as it happens, as opposed to polling an API" ([about-webhooks]). Nothing in the docs offers a GitHub-hosted queue you can drain from behind the firewall.

### The pull alternatives (all outbound-only — ZERO inbound ports)

| Option | What it gives you | Documented limits |
| --- | --- | --- |
| **REST notifications API** — `GET /notifications` ([rest-notifications]) | Threads with `reason`, repo, subject; `since`/`before` params; `Last-Modified` + `304` and an `X-Poll-Interval` header (≥ 60 s) make polling cheap | **Classic PAT only** ("These endpoints only support authentication using a personal access token (classic)"); it is a *personal inbox* (per-user subscriptions), not a repo event feed; threads, not raw events — you must re-derive merge/label semantics client-side |
| **REST events API** — `GET /repos/{owner}/{repo}/events` ([rest-events]) | Raw `PullRequestEvent`, `IssuesEvent`, `PushEvent` objects with action, base ref, assignee, labels — enough to compute both critical events | Explicitly "not built to serve real-time use cases ... latency can be anywhere from 30s to 6h"; timeline capped at 300 events and 30 days; ETag/`304` + `X-Poll-Interval` |
| **GraphQL** ([graphql-activity]) | Full Issues/PRs objects on demand, so a poller can diff state | The schema exposes **no notifications feed** — the Activity reference contains only subscription *management* (`Subscribable`, `SubscriptionState`, `updateSubscription`). You poll and diff yourself; no realtime transport is documented |

**Firewall verdict:**

- **Zero inbound ports (outbound-only through a corporate proxy):** REST notifications polling, REST events polling, GraphQL polling, and any Actions workflow that posts to an approved SaaS (Slack/Teams) — plus GHES↔Slack via Slack Socket Mode, which the Slack README documents as an outbound connection for proxied instances ([integrations-slack]).
- **Requires inbound reachability (GitHub cloud → you):** repository/org webhooks, GitHub App webhooks, and any Actions `curl` aimed at an internal, non-internet-exposed endpoint. For a team whose relay must live inside the perimeter, this is the hard blocker: the only documented GitHub-native ways to get near-real-time events all require either opening a hole or sending the data back out to SaaS.
- Middle ground: GitHub-hosted Actions → SaaS is outbound-only everywhere, but "audit metadata stays inside the perimeter" is then false — the record lives in Slack/Teams/Actions logs.

## Reasons to still build the relay

**Facts that argue AGAINST building it (GitHub covers the need):**

1. Two Actions triggers with documented syntax reproduce both critical events *exactly*, including the default-branch gate and the label gate ([events-that-trigger-workflows]) — the relay's filtering logic becomes a ~20-line workflow.
2. If the team accepts Slack/Teams as the surface, the official Slack integration delivers default-branch commits with zero code ([integrations-slack]), and org email can reach people natively ([configuring-notifications]).
3. Delivery reliability is comparable, not better, on the relay side: GitHub also does not auto-retry webhooks ([redelivering-webhooks]), and both GitHub and the relay use a per-delivery ID for dedupe ([best-practices-webhooks]).
4. GitHub itself is the system of record; a merge/assignment happening is already durably recorded in Issues/PRs, so a second "audit" copy has diminishing returns if you accept GitHub's UI/API as the archive.

**Facts that argue FOR building it:**

1. **Exact filtering is not for sale natively.** No branch filter, no label-gated delivery, no merge-specific reason ([rest-notifications], [configuring-notifications]). "Use Actions" is real, but it means production notification semantics live in workflow YAML in each repo, with repo-admin permissions and the "workflow must exist on the default branch" constraint ([events-that-trigger-workflows]).
2. **Terminal-first UX.** Every GitHub-native surface is a web inbox, a mobile app, or email ([configuring-notifications]); Slack needs the Slack client. The spec's core complaint — being pulled out of the terminal — is untouched by all of these. Only the relay's SSE feed + CLI keeps the developer in the terminal (spec stories 11–13).
3. **A durable, shared, filterable audit history exists nowhere natively.** Notification inbox = 3 months per user ([about-notifications]); webhook redelivery history = 3 days ([redelivering-webhooks]); events API = 300 events / 30 days / 30 s–6 h latency ([rest-events]). The relay's SQLite history is team-wide, indefinite, and queryable with the same shape as the stream (spec stories 1, 15, 16, 21). Slack channel history is shared but is an unstructured chat log, not a queryable event store.
4. **Broadcast without identity.** Native notifications are per-user subscriptions; Slack mentions/threads are per-person too ([integrations-slack]). The relay's model — one event, every connected client, no logins (spec story 14) — has no GitHub-native equivalent.
5. **Data locality.** GitHub cloud webhooks and Actions-to-SaaS both move the metadata (assignee emails included) outside the perimeter; per the team-lead stories (17–20), only an in-network listener or outbound-only polling keeps it inside — and GitHub offers no official webhook-drain/pull API to make the latter real-time ([about-webhooks], [rest-notifications]).

**Honest verdict:** if this team were on GitHub *and* TI approved Slack/Teams *and* they dropped the terminal-first and durable-audit requirements, then **GitHub Actions + the Slack integration would cover both critical events with no custom code**, and building the relay for *delivery* would be redundant. What no GitHub option provides, in any combination, is: exact-event filtering *and* a terminal-native stream *and* an indefinite shared audit record *and* zero inbound ports with data staying in-network. That combination is the relay's actual product — and it is also why the same relay design ports to self-managed GitLab unchanged.

## Sources

All fetched successfully on 2026-10-15 unless noted.

1. [about-notifications] — About notifications — https://docs.github.com/en/account-and-profile/managing-subscriptions-and-notifications-on-github/setting-up-notifications/about-notifications
2. [configuring-notifications] — Configuring notifications — https://docs.github.com/en/account-and-profile/managing-subscriptions-and-notifications-on-github/setting-up-notifications/configuring-notifications
3. [rest-notifications] — REST API endpoints for notifications (reasons table, PAT-classic-only note, `X-Poll-Interval`) — https://docs.github.com/en/rest/activity/notifications?apiVersion=2022-11-28
4. [rest-events] — REST API endpoints for events (latency and retention notes) — https://docs.github.com/en/rest/activity/events?apiVersion=2022-11-28
5. [about-webhooks] — About webhooks — https://docs.github.com/en/webhooks/about-webhooks
6. [creating-webhooks] — Creating webhooks (repo/org/GitHub App webhooks, event selection) — https://docs.github.com/en/webhooks/using-webhooks/creating-webhooks
7. [best-practices-webhooks] — Best practices for using webhooks (10-second rule, IP allowlist, `X-GitHub-Delivery`) — https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks
8. [redelivering-webhooks] — Redelivering webhooks ("GitHub does not automatically redeliver failed deliveries", 3-day window) — https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks
9. [events-that-trigger-workflows] — Events that trigger workflows (`pull_request` closed + `merged == true`, `branches` filters, `issues` activity types) — https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows
10. [integrations-slack] — GitHub integration for Slack, official README (features, branch/label filters, GHES + Socket Mode) — https://github.com/integrations/slack
11. [about-creating-github-apps] — About creating GitHub Apps (one webhook, permissions gate events, hosting requirement) — https://docs.github.com/en/apps/creating-github-apps/about-creating-github-apps/about-creating-github-apps
12. [graphql-activity] — GraphQL Activity reference (no notifications feed; subscription state only) — https://docs.github.com/en/graphql/reference/activity

Fetch attempts that failed (404) and were replaced by the equivalents above:

- https://docs.github.com/en/webhooks/using-webhooks/about-webhooks (use `/en/webhooks/about-webhooks`)
- https://docs.github.com/en/apps/creating-github-apps/about-creating-github-apps/about-github-apps (use `.../about-creating-github-apps/about-creating-github-apps`)
- https://docs.github.com/en/apps/creating-github-apps/learn-about-github-apps/about-github-apps (URL no longer valid)

Note: the GraphQL *queries index* (https://docs.github.com/en/graphql/reference/queries) lists no notifications category; the Activity reference (source 12) was fetched to confirm the absence of any notification-read API in GraphQL.

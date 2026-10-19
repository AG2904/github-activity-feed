import { isoInstant, type ActivityEvent } from '../domain/activity-event.js';
import { carriesCriticalLabel } from '../domain/critical.js';
import { asRecord, labelNames, loginOf, repositoryName, type GitHubDelivery } from './payload.js';

interface GitHubUser {
  login: string;
}

export interface IssuePayload extends GitHubDelivery {
  issue: Record<string, unknown>;
  assignee?: GitHubUser | null;
}

/**
 * The users GitHub reports on the issue, most specific first: the issue's own
 * `assignee` (GitHub's pick of "the" assignee) ahead of the full list.
 */
function reportedAssignees(issue: Record<string, unknown>): string[] {
  const listed = Array.isArray(issue.assignees) ? issue.assignees : [];
  const logins = [loginOf(issue.assignee), ...listed.map(loginOf)];
  return [...new Set(logins.filter((login): login is string => login !== undefined))];
}

/**
 * A critical event exactly when GitHub reports an issue carrying a critical
 * label with someone on it, on the action that assigns.
 *
 * `opened` is deliberately not a trigger. GitHub delivers `assigned` and
 * `opened` as two deliveries for an issue created already assigned, and both
 * report the same assignee set, so accepting `opened` stores one happening
 * twice. The creation is not lost: it is read from the `assigned` delivery that
 * arrives with it.
 *
 * An action that merely touches an already-assigned issue — labeling it,
 * editing it — is not an assignment and produces nothing, because reporting it
 * would attribute to the labeler an assignment they never made.
 *
 * GitHub's `issues` payload carries no `changes` object and no
 * previous-assignee set, so there is nothing to diff: the reported state is the
 * only evidence available, and this is the closest honest reading of "the
 * assignee set changed".
 */
export function isCriticalAssignment(payload: unknown): payload is IssuePayload {
  const webhook = asRecord(payload);
  const issue = asRecord(webhook?.issue);
  if (!webhook || !issue) {
    return false;
  }
  if (!carriesCriticalLabel(labelNames(issue.labels))) {
    return false;
  }
  if (!(loginOf(webhook.assignee) ?? reportedAssignees(issue)[0])) {
    return false;
  }
  return webhook.action === 'assigned';
}

/**
 * Maps GitHub's issue payload to the ActivityEvent shape. Returns undefined
 * when the payload passes the filter but cannot be normalized, so it is ignored
 * without error. The actor is whoever performed the assignment; the assignee is
 * whoever received it.
 */
export function toAssignmentEvent(payload: IssuePayload): ActivityEvent | undefined {
  const issue = asRecord(payload?.issue) ?? {};
  const actor = loginOf(payload?.sender);
  // GitHub timestamps the assignment on the issue itself, not on the event: a
  // later assignment moves `updated_at`, and so does the one that lands at
  // creation, which is why `created_at` is no longer read here.
  const when = isoInstant(issue.updated_at);
  const subject = issue.title;
  const url = issue.html_url;
  if (!actor || !when || typeof subject !== 'string' || typeof url !== 'string') {
    return undefined;
  }
  return {
    type: 'issue.assigned',
    repository: repositoryName(payload?.repository),
    actor,
    subject,
    assignee: loginOf(payload.assignee) ?? reportedAssignees(issue)[0] ?? null,
    labels: labelNames(issue.labels),
    url,
    when,
  };
}

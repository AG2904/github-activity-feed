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
 * label with someone on it — either the update that adds an assignee, or a
 * creation that already arrives assigned.
 *
 * The action is half the trigger, never all of it. `opened` qualifies only when
 * the issue comes with someone already assigned, which is what makes an
 * assignment at creation count the same as a later one; and an action that
 * merely touches an already-assigned issue — labeling it, editing it — is not
 * an assignment and produces nothing, because reporting it would attribute to
 * the labeler an assignment they never made.
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
  return webhook.action === 'assigned' || webhook.action === 'opened';
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
  // later assignment moves `updated_at`, a creation carries `created_at`.
  const when = isoInstant(payload?.action === 'opened' ? issue.created_at : issue.updated_at);
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

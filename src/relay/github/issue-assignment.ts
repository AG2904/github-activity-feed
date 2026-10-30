import { isoInstant } from '../domain/activity-event.js';
import { carriesCriticalLabel } from '../domain/critical.js';
import type { Ingestion } from './ingestion.js';
import { asRecord, labelNames, loginOf, reportedAssignees, repositoryName } from './payload.js';

/**
 * The assignment worth interrupting someone for, or why nothing will be stored.
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
 * GitHub's `issues` payload carries no `changes` object and no previous-assignee
 * set, so there is nothing to diff: the reported state is the only evidence
 * available, and this is the closest honest reading of "the assignee set
 * changed".
 *
 * Filter and normalization are one pass (ADR-0015), which also means the
 * reported assignees are read once: the same walk decides whether there is
 * somebody to tell and who it is.
 */
export function criticalAssignment(payload: unknown): Ingestion {
  const delivery = asRecord(payload);
  if (delivery?.action !== 'assigned') {
    return { kind: 'ignored', reason: 'not-critical' };
  }
  const issue = asRecord(delivery.issue);
  if (!issue || !carriesCriticalLabel(labelNames(issue.labels))) {
    return { kind: 'ignored', reason: 'not-critical' };
  }
  const assignee = loginOf(delivery.assignee) ?? reportedAssignees(issue)[0];
  if (!assignee) {
    return { kind: 'ignored', reason: 'not-critical' };
  }

  const actor = loginOf(delivery.sender);
  // GitHub timestamps the assignment on the issue itself, not on the event: a
  // later assignment moves `updated_at`, and so does the one that lands at
  // creation, which is why `created_at` is no longer read here.
  const when = isoInstant(issue.updated_at);
  const subject = issue.title;
  const url = issue.html_url;
  if (!actor || !when || typeof subject !== 'string' || typeof url !== 'string') {
    return { kind: 'ignored', reason: 'malformed' };
  }

  return {
    kind: 'event',
    event: {
      type: 'issue.assigned',
      repository: repositoryName(delivery.repository),
      branch: null,
      actor,
      subject,
      assignee,
      labels: labelNames(issue.labels),
      url,
      when,
    },
  };
}

import type { ActivityEventType } from '../domain/activity-event.js';
import { isoInstant } from '../domain/activity-event.js';
import { carriesCriticalLabel } from '../domain/critical.js';
import type { Ingestion } from './ingestion.js';
import {
  asRecord,
  labelNames,
  loginOf,
  reportedAssignees,
  repositoryName,
} from './payload.js';

/**
 * The type a closure is stored as, from the reason GitHub reports for it.
 *
 * `not_planned` is the only reason that is not a resolution: the work was
 * decided against, not done. So it gets its own literal instead of being folded
 * into a closure, which is what keeps the feed truthful — "closed" and "closed
 * as not planned" are different news. Every other value — `completed`, the
 * `null` of a payload that predates the field, or a reason GitHub has not named
 * yet — reads as a closure: the action *was* `closed`, and not knowing why is
 * not the same as knowing the issue was discarded.
 */
function closureType(reason: unknown): ActivityEventType {
  return reason === 'not_planned' ? 'issue.dismissed' : 'issue.closed';
}

/**
 * The closure worth reporting, or why nothing will be stored.
 *
 * A closure is where the attention an assignment started ends, so it is gated on
 * the same `bug` / `feedback` labels read at that moment: closing a routine task
 * is not news, closing an urgent bug is. The payload is the issue's own state —
 * GitHub does not send a previous-state diff — so the reported labels are the
 * evidence, exactly as they are for an assignment.
 *
 * Filter and normalization are one pass (ADR-0015), and the assignee is read
 * from the issue rather than from the delivery: the question is whose issue it
 * was, not who GitHub happened to name at the top level.
 */
export function criticalClosure(payload: unknown): Ingestion {
  const delivery = asRecord(payload);
  if (delivery?.action !== 'closed') {
    return { kind: 'ignored', reason: 'not-critical' };
  }
  const issue = asRecord(delivery.issue);
  if (!issue || !carriesCriticalLabel(labelNames(issue.labels))) {
    return { kind: 'ignored', reason: 'not-critical' };
  }

  const actor = loginOf(delivery.sender);
  // GitHub stamps the closure on the issue: `updated_at` moves when the state
  // does, and the payload carries no closer timestamp of its own.
  const when = isoInstant(issue.updated_at);
  const subject = issue.title;
  const url = issue.html_url;
  if (!actor || !when || typeof subject !== 'string' || typeof url !== 'string') {
    return { kind: 'ignored', reason: 'malformed' };
  }

  return {
    kind: 'event',
    event: {
      type: closureType(issue.state_reason),
      repository: repositoryName(delivery.repository),
      branch: null,
      actor,
      subject,
      assignee: reportedAssignees(issue)[0] ?? null,
      labels: labelNames(issue.labels),
      url,
      when,
    },
  };
}

import { isoInstant } from '../domain/activity-event.js';
import { isMainBranch } from '../domain/critical.js';
import type { Ingestion } from './ingestion.js';
import { asRecord, labelNames, loginOf, repositoryName } from './payload.js';

/**
 * The merge that landed on main, or why nothing will be stored.
 *
 * Detection keys on the reported merge state only — never on intermediate
 * payload details: GitHub reports `closed` together with `merged: true` for a
 * merged pull request, and the filter reads that pair rather than inferring a
 * merge from anything else in the payload.
 *
 * One pass, because the filter and the normalization read the same payload. They
 * used to be two functions joined by a type predicate that proved three fields and
 * promised eight, so the mapper re-read every field it touched (ADR-0015). Now the
 * question and the answer are the same walk, and an answer is either the event or
 * a refusal that says which of the two ways it found nothing.
 */
export function mergeToMain(payload: unknown): Ingestion {
  const delivery = asRecord(payload);
  if (delivery?.action !== 'closed') {
    return { kind: 'ignored', reason: 'not-critical' };
  }
  const pullRequest = asRecord(delivery.pull_request);
  if (pullRequest?.merged !== true) {
    return { kind: 'ignored', reason: 'not-critical' };
  }
  if (!isMainBranch(asRecord(pullRequest.base)?.ref)) {
    return { kind: 'ignored', reason: 'not-critical' };
  }

  // Past the filter, so a missing field is a disagreement about the shape rather
  // than a payload the relay meant to skip: it is refused as malformed, not as
  // not-critical.
  const actor = loginOf(delivery.sender);
  const when = isoInstant(pullRequest.merged_at);
  const subject = pullRequest.title;
  const url = pullRequest.html_url;
  if (
    !actor ||
    !when ||
    typeof subject !== 'string' ||
    typeof url !== 'string' ||
    !Array.isArray(pullRequest.labels)
  ) {
    return { kind: 'ignored', reason: 'malformed' };
  }

  // The source branch is supplementary: a payload without a usable head ref still
  // normalizes, it simply has no branch to name (ADR-0013).
  const headRef = asRecord(pullRequest.head)?.ref;
  return {
    kind: 'event',
    event: {
      type: 'pr.merged',
      repository: repositoryName(delivery.repository),
      branch: typeof headRef === 'string' && headRef !== '' ? headRef : null,
      actor,
      subject,
      assignee: null,
      labels: labelNames(pullRequest.labels),
      url,
      when,
    },
  };
}

import type { Ingestion } from './ingestion.js';
import { criticalAssignment } from './issue-assignment.js';
import { mergeToMain } from './merge-to-main.js';

/**
 * The single entry point into the GitHub adapter: turns any webhook payload into
 * the project's one decision about it — an event, or the reason there is none
 * (ADR-0014).
 *
 * This is the only module that knows GitHub's payload shapes, so the domain and
 * the HTTP seam stay ignorant of `pull_request`, `issues` and their fields.
 *
 * A `malformed` answer is terminal. The payload already passed one adapter's
 * filter, so the other adapter must not read it again: a merge that is critical
 * but unreadable must not end up stored as an assignment just because it is also
 * shaped like nothing the filter would reject. One payload, one answer.
 */
export function ingest(payload: unknown): Ingestion {
  const merge = mergeToMain(payload);
  if (merge.kind === 'event' || merge.reason === 'malformed') {
    return merge;
  }
  return criticalAssignment(payload);
}

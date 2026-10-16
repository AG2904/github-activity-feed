import type { ActivityEvent } from '../domain/activity-event.js';
import { isCriticalAssignment, toAssignmentEvent } from './issue-assignment.js';
import { isMergeToMain, toMergeToMainEvent } from './merge-to-main.js';

/**
 * The single entry point into the GitHub adapter: turns any webhook payload
 * into the project's one ActivityEvent shape.
 *
 * This is the only module that knows GitHub's payload shapes, so the domain
 * and the HTTP seam stay ignorant of `pull_request`, `issues` and their fields.
 *
 * It returns `undefined` for two different situations on purpose: a payload
 * that is not a critical event, and one that is critical but cannot be
 * normalized. They are indistinguishable from outside the relay — both are
 * acknowledged with a success code and neither is persisted — so there is
 * nothing for a caller to branch on.
 */
export function toActivityEvent(payload: unknown): ActivityEvent | undefined {
  if (isMergeToMain(payload)) {
    return toMergeToMainEvent(payload);
  }
  if (isCriticalAssignment(payload)) {
    return toAssignmentEvent(payload);
  }
  return undefined;
}

import { isoInstant, type ActivityEvent } from '../domain/activity-event.js';
import { isMainBranch } from '../domain/critical.js';
import { asRecord, labelNames, loginOf } from './payload.js';

interface GitHubPullRequest {
  title: string;
  merged: boolean;
  merged_at: string | null;
  html_url: string;
  base: { ref: string };
  labels: Array<{ name: string }>;
}

export interface PullRequestPayload {
  action: string;
  sender: { login: string };
  pull_request: GitHubPullRequest;
}

/**
 * A critical event exactly when GitHub reports the pull request closed,
 * merged, with base branch `main`. Detection keys on the reported merge state
 * only — never on intermediate payload details.
 */
export function isMergeToMain(payload: unknown): payload is PullRequestPayload {
  const webhook = asRecord(payload);
  const pullRequest = asRecord(webhook?.pull_request);
  const base = asRecord(pullRequest?.base);
  return (
    webhook?.action === 'closed' && pullRequest?.merged === true && isMainBranch(base?.ref)
  );
}

/**
 * Maps GitHub's payload to the ActivityEvent shape, flattening labels to
 * strings. Returns undefined when the payload passes the filter but cannot be
 * normalized — per the spec such events are ignored without error rather than
 * failing the request, so every field read here is guarded.
 */
export function toMergeToMainEvent(payload: PullRequestPayload): ActivityEvent | undefined {
  const pullRequest = asRecord(payload?.pull_request);
  const actor = loginOf(payload?.sender);
  const when = isoInstant(pullRequest?.merged_at);
  const subject = pullRequest?.title;
  const url = pullRequest?.html_url;
  if (
    !actor ||
    !when ||
    typeof subject !== 'string' ||
    typeof url !== 'string' ||
    !Array.isArray(pullRequest?.labels)
  ) {
    return undefined;
  }
  return {
    type: 'pr.merged',
    actor,
    subject,
    assignee: null,
    labels: labelNames(pullRequest.labels),
    url,
    when,
  };
}

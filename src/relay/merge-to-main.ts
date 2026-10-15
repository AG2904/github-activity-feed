import type { ActivityEvent } from './domain/activity-event.js';

interface GitHubPullRequest {
  number: number;
  title: string;
  merged: boolean;
  merged_at: string | null;
  html_url: string;
  base: { ref: string };
  user: { login: string };
  labels: Array<{ name: string }>;
}

interface GitHubWebhookPayload {
  action: string;
  sender: { login: string };
  pull_request: GitHubPullRequest;
}

/**
 * A critical event exactly when GitHub reports the pull request closed,
 * merged, with base branch `main`. Detection keys on the reported merge
 * state only — never on intermediate payload details.
 */
export function isMergeToMain(payload: unknown): payload is GitHubWebhookPayload {
  if (typeof payload !== 'object' || payload === null) {
    return false;
  }
  const webhook = payload as Record<string, unknown>;
  const pullRequest = webhook.pull_request as Record<string, unknown> | undefined;
  const base = pullRequest?.base as Record<string, unknown> | undefined;
  return (
    webhook.action === 'closed' && pullRequest?.merged === true && base?.ref === 'main'
  );
}

/** Maps GitHub's payload to the ActivityEvent shape, flattening labels to strings. */
export function toActivityEvent(payload: GitHubWebhookPayload): ActivityEvent {
  const mergedAt = payload.pull_request.merged_at ?? new Date(0).toISOString();
  return {
    type: 'pr.merged',
    actor: payload.sender.login,
    subject: payload.pull_request.title,
    assignee: '',
    labels: payload.pull_request.labels.map((label) => label.name),
    url: payload.pull_request.html_url,
    when: new Date(mergedAt).toISOString(),
  };
}

import { Test } from '@nestjs/testing';
import type { NestExpressApplication } from '@nestjs/platform-express';
import { createHmac } from 'node:crypto';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import request from 'supertest';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { AppModule } from '../src/app.module.js';
import { configureIngress, INGRESS_OPTIONS } from '../src/relay/ingress.js';

const SECRET = 'test-secret';

/**
 * A body that is not valid JSON, sent the way GitHub would send it: with a
 * signature over exactly these bytes. The relay must verify that signature and
 * then discard the body, never let a parser reject it first.
 */
const UNREADABLE_BODY = '{ "action": "closed", "pull_request": { "merged": true,';

function sign(body: string, secret: string = SECRET): string {
  return `sha256=${createHmac('sha256', secret).update(body).digest('hex')}`;
}

function postWebhook(body: string, signature = sign(body)) {
  return request(app.getHttpServer())
    .post('/')
    .set('Content-Type', 'application/json')
    .set('X-Hub-Signature-256', signature)
    .send(body);
}

function getHistory() {
  return request(app.getHttpServer()).get('/history');
}

/** The audit history the relay serves, as it stands right now. */
async function storedEvents(): Promise<unknown[]> {
  const response = await getHistory().expect(200);
  return response.body as unknown[];
}

/**
 * A captured `pull_request` delivery. `action` is the action GitHub reports at
 * the top level; every other key overrides a field of the `pull_request` object.
 */
function pullRequestPayload(
  overrides: { action?: string } & Record<string, unknown> = {},
): Record<string, unknown> {
  const { action = 'closed', ...pullRequest } = overrides;
  return {
    action,
    sender: { login: 'maria.perez' },
    pull_request: {
      number: 42,
      title: 'Fix memory leak in parser component',
      merged: true,
      merged_at: '2026-10-15T12:00:00Z',
      html_url: 'https://github.com/AG2904/Gitlab-notifier/pull/42',
      base: { ref: 'main' },
      user: { login: 'maria.perez' },
      labels: [{ name: 'bug' }, { name: 'area:relay' }],
      ...pullRequest,
    },
  };
}

function issuePayload(
  overrides: { issue?: Record<string, unknown> } & Record<string, unknown> = {},
): Record<string, unknown> {
  const { issue, ...rest } = overrides;
  return {
    action: 'assigned',
    sender: { login: 'lucia.gomez' },
    assignee: { login: 'tomas.ferreira' },
    issue: {
      number: 7,
      title: 'Webhook drops events under load',
      html_url: 'https://github.com/AG2904/Gitlab-notifier/issues/7',
      created_at: '2026-10-15T09:00:00Z',
      updated_at: '2026-10-15T11:00:00Z',
      labels: [{ name: 'bug' }],
      assignee: { login: 'tomas.ferreira' },
      assignees: [{ login: 'tomas.ferreira' }],
      ...issue,
    },
    ...rest,
  };
}

let app: NestExpressApplication;

describe('Relay HTTP seam', () => {
  const dbDir = mkdtempSync(join(tmpdir(), 'relay-test-'));

  beforeAll(async () => {
    process.env.GITHUB_WEBHOOK_SECRET = SECRET;
    process.env.DATABASE_PATH = join(dbDir, 'relay.db');
    const moduleRef = await Test.createTestingModule({ imports: [AppModule] }).compile();
    app = moduleRef.createNestApplication<NestExpressApplication>(INGRESS_OPTIONS);
    configureIngress(app);
    await app.init();
  });

  afterAll(async () => {
    await app?.close();
    rmSync(dbDir, { recursive: true, force: true });
  });

  describe('authentication (ticket 01)', () => {
    it('rejects a webhook that does not carry a signature with 401', async () => {
      await request(app.getHttpServer())
        .post('/')
        .set('Content-Type', 'application/json')
        .send('{}')
        .expect(401);
    });

    it('rejects a webhook signed with the wrong secret with 401', async () => {
      const body = JSON.stringify({ zen: 'design for failure' });
      await postWebhook(body, sign(body, 'not-the-secret')).expect(401);
    });

    it('rejects a webhook whose body does not match its signature with 401', async () => {
      await postWebhook(
        JSON.stringify({ action: 'closed' }),
        sign(JSON.stringify({ action: 'opened' })),
      ).expect(401);
    });

    it('rejects a malformed signature header with 401', async () => {
      await postWebhook('{}', 'deadbeef').expect(401);
    });

    it('rejects a well-formed digest that is not valid hex with 401', async () => {
      await postWebhook('{}', `sha256=${'z'.repeat(64)}`).expect(401);
    });

    it('acknowledges a webhook carrying a valid signature over the raw body', async () => {
      const body = JSON.stringify({ object_kind: 'push' });
      await postWebhook(body).expect(200);
    });

    it('acknowledges a body signed byte-for-byte as sent, with non-canonical whitespace', async () => {
      const body = '{\n  "object_kind": "push"\n}';
      await postWebhook(body).expect(200);
    });
  });

  describe('audit history', () => {
    it('serves an empty audit history before anything is stored', async () => {
      const response = await getHistory().expect(200);
      expect(response.body).toEqual([]);
    });
  });

  describe('a merge into main becomes an event (ticket 02)', () => {
    it('stores a PR merged into main as an ActivityEvent readable from history', async () => {
      const response = await postWebhook(JSON.stringify(pullRequestPayload())).expect(200);
      expect(response.body).toEqual({ received: true });

      const history = await getHistory().expect(200);
      expect(history.body).toContainEqual({
        type: 'pr.merged',
        actor: 'maria.perez',
        subject: 'Fix memory leak in parser component',
        assignee: null,
        labels: ['bug', 'area:relay'],
        url: 'https://github.com/AG2904/Gitlab-notifier/pull/42',
        when: '2026-10-15T12:00:00.000Z',
      });
    });

    it('stores timestamps in canonical UTC form regardless of the offset in the payload', async () => {
      await postWebhook(
        JSON.stringify(pullRequestPayload({ merged_at: '2026-10-15T14:00:00+02:00' })),
      ).expect(200);

      const history = await getHistory().expect(200);
      expect(history.body).toContainEqual(
        expect.objectContaining({ url: 'https://github.com/AG2904/Gitlab-notifier/pull/42', when: '2026-10-15T12:00:00.000Z' }),
      );
    });

    it('ignores a PR merged into a branch other than main', async () => {
      await postWebhook(
        JSON.stringify(
          pullRequestPayload({
            base: { ref: 'develop' },
            html_url: 'https://github.com/AG2904/Gitlab-notifier/pull/43',
          }),
        ),
      ).expect(200);

      const history = await getHistory().expect(200);
      expect(history.body).not.toContainEqual(
        expect.objectContaining({ url: 'https://github.com/AG2904/Gitlab-notifier/pull/43' }),
      );
    });

    it('ignores a closed PR that was not merged', async () => {
      await postWebhook(
        JSON.stringify(
          pullRequestPayload({
            merged: false,
            merged_at: null,
            html_url: 'https://github.com/AG2904/Gitlab-notifier/pull/44',
          }),
        ),
      ).expect(200);

      const history = await getHistory().expect(200);
      expect(history.body).not.toContainEqual(
        expect.objectContaining({ url: 'https://github.com/AG2904/Gitlab-notifier/pull/44' }),
      );
    });

    it('ignores a critical-looking payload that cannot be normalized', async () => {
      await postWebhook(
        JSON.stringify({
          action: 'closed',
          sender: { login: 'maria.perez' },
          pull_request: {
            merged: true,
            base: { ref: 'main' },
            html_url: 'https://github.com/AG2904/Gitlab-notifier/pull/45',
          },
        }),
      ).expect(200);

      const history = await getHistory().expect(200);
      expect(history.body).not.toContainEqual(
        expect.objectContaining({ url: 'https://github.com/AG2904/Gitlab-notifier/pull/45' }),
      );
    });

    it('ignores a filter-passing merge payload with no actor, without error', async () => {
      await postWebhook(
        JSON.stringify({
          action: 'closed',
          // No `sender`: the payload passes the filter but has nobody to
          // attribute it to, so it must be dropped rather than throw.
          pull_request: {
            title: 'Fix memory leak in parser component',
            merged: true,
            merged_at: '2026-10-15T12:00:00Z',
            html_url: 'https://github.com/AG2904/Gitlab-notifier/pull/46',
            base: { ref: 'main' },
            labels: [{ name: 'bug' }],
          },
        }),
      ).expect(200);

      const history = await getHistory().expect(200);
      expect(history.body).not.toContainEqual(
        expect.objectContaining({ url: 'https://github.com/AG2904/Gitlab-notifier/pull/46' }),
      );
    });
  });

  describe('an issue assignment becomes an event (ticket 03)', () => {
    it('stores an issue update that adds an assignee while carrying bug', async () => {
      await postWebhook(
        JSON.stringify(
          issuePayload({
            issue: { html_url: 'https://github.com/AG2904/Gitlab-notifier/issues/71' },
          }),
        ),
      ).expect(200);

      const history = await getHistory().expect(200);
      expect(history.body).toContainEqual({
        type: 'issue.assigned',
        actor: 'lucia.gomez',
        subject: 'Webhook drops events under load',
        assignee: 'tomas.ferreira',
        labels: ['bug'],
        url: 'https://github.com/AG2904/Gitlab-notifier/issues/71',
        when: '2026-10-15T11:00:00.000Z',
      });
    });

    it('stores an issue created with an assignee and carrying feedback', async () => {
      await postWebhook(
        JSON.stringify(
          issuePayload({
            action: 'opened',
            // GitHub sends no top-level `assignee` on `opened`; the issue itself
            // carries who is on it.
            assignee: undefined,
            issue: {
              html_url: 'https://github.com/AG2904/Gitlab-notifier/issues/72',
              labels: [{ name: 'feedback' }],
              created_at: '2026-10-15T08:30:00Z',
              // Deliberately later than created_at, so that reading the wrong
              // field for `when` fails this test instead of passing by luck.
              updated_at: '2026-10-15T09:45:00Z',
            },
          }),
        ),
      ).expect(200);

      const history = await getHistory().expect(200);
      expect(history.body).toContainEqual(
        expect.objectContaining({
          type: 'issue.assigned',
          actor: 'lucia.gomez',
          assignee: 'tomas.ferreira',
          labels: ['feedback'],
          url: 'https://github.com/AG2904/Gitlab-notifier/issues/72',
          when: '2026-10-15T08:30:00.000Z',
        }),
      );
    });

    it('ignores an issue assignment that carries no critical label', async () => {
      await postWebhook(
        JSON.stringify(
          issuePayload({
            issue: {
              html_url: 'https://github.com/AG2904/Gitlab-notifier/issues/73',
              labels: [{ name: 'chore' }, { name: 'area:relay' }],
            },
          }),
        ),
      ).expect(200);

      const history = await getHistory().expect(200);
      expect(history.body).not.toContainEqual(
        expect.objectContaining({ url: 'https://github.com/AG2904/Gitlab-notifier/issues/73' }),
      );
    });

    it('ignores an issue opened with a critical label but nobody assigned', async () => {
      await postWebhook(
        JSON.stringify(
          issuePayload({
            action: 'opened',
            assignee: undefined,
            issue: {
              html_url: 'https://github.com/AG2904/Gitlab-notifier/issues/74',
              assignee: null,
              assignees: [],
            },
          }),
        ),
      ).expect(200);

      const history = await getHistory().expect(200);
      expect(history.body).not.toContainEqual(
        expect.objectContaining({ url: 'https://github.com/AG2904/Gitlab-notifier/issues/74' }),
      );
    });

    it('ignores an already-assigned issue that is instead labeled', async () => {
      // The issue carries a critical label and someone is on it, but nothing
      // was assigned: firing here would credit the labeler with the assignment.
      await postWebhook(
        JSON.stringify(
          issuePayload({
            action: 'labeled',
            assignee: undefined,
            issue: { html_url: 'https://github.com/AG2904/Gitlab-notifier/issues/75' },
          }),
        ),
      ).expect(200);

      const history = await getHistory().expect(200);
      expect(history.body).not.toContainEqual(
        expect.objectContaining({ url: 'https://github.com/AG2904/Gitlab-notifier/issues/75' }),
      );
    });

    it('ignores a malformed issue payload without error', async () => {
      await postWebhook(JSON.stringify({ action: 'assigned', issue: null })).expect(200);
    });
  });

  describe('everything else is ignored (ticket 04)', () => {
    const pullUrl = (number: number) => `https://github.com/AG2904/Gitlab-notifier/pull/${number}`;
    const issueUrl = (number: number) =>
      `https://github.com/AG2904/Gitlab-notifier/issues/${number}`;

    /**
     * The negative half of the filter, as one table: every webhook in it has to
     * be acknowledged and discarded. It is a table because the ticket's last
     * criterion is a sweep — posting the whole set must leave the audit history
     * exactly as it was.
     *
     * `marker` is the URL the payload would be stored under if the filter ever
     * let it through, so a silent regression names the payload that leaked.
     */
    interface NonCriticalWebhook {
      /** The hook GitHub names in `X-GitHub-Event`; the relay keys on the payload. */
      event: string;
      name: string;
      body: string;
      marker?: string;
    }

    const NON_CRITICAL_WEBHOOKS: NonCriticalWebhook[] = [
      {
        event: 'pull_request',
        name: 'a pull request merged into a branch other than main',
        body: JSON.stringify(
          pullRequestPayload({ base: { ref: 'develop' }, html_url: pullUrl(141) }),
        ),
        marker: pullUrl(141),
      },
      {
        event: 'pull_request',
        name: 'a pull request that is opened',
        body: JSON.stringify(
          pullRequestPayload({
            action: 'opened',
            merged: false,
            merged_at: null,
            html_url: pullUrl(142),
          }),
        ),
        marker: pullUrl(142),
      },
      {
        event: 'pull_request',
        name: 'a pull request that is closed without being merged',
        body: JSON.stringify(
          pullRequestPayload({ merged: false, merged_at: null, html_url: pullUrl(143) }),
        ),
        marker: pullUrl(143),
      },
      {
        event: 'pull_request',
        name: 'a pull request that is reopened',
        body: JSON.stringify(
          pullRequestPayload({
            action: 'reopened',
            merged: false,
            merged_at: null,
            html_url: pullUrl(144),
          }),
        ),
        marker: pullUrl(144),
      },
      {
        // Editing a merged pull request's title is an ordinary delivery, and the
        // merge state alone — closed, merged, base `main` — would store it. This
        // is the fixture that proves the action carries half the filter.
        event: 'pull_request',
        name: 'a merged pull request that is only edited afterwards',
        body: JSON.stringify(pullRequestPayload({ action: 'edited', html_url: pullUrl(145) })),
        marker: pullUrl(145),
      },
      {
        event: 'issues',
        name: 'an issue assigned without a critical label',
        body: JSON.stringify(
          issuePayload({ issue: { html_url: issueUrl(81), labels: [{ name: 'chore' }] } }),
        ),
        marker: issueUrl(81),
      },
      {
        event: 'ping',
        name: 'a ping, the hook GitHub sends when a webhook is configured',
        body: JSON.stringify({ zen: 'Non-blocking is better than blocking.' }),
      },
      {
        event: 'push',
        name: 'a push to main, which is not a merge',
        body: JSON.stringify({
          ref: 'refs/heads/main',
          after: 'a1b2c3d4e5f6',
          sender: { login: 'maria.perez' },
        }),
      },
      {
        event: 'issues',
        name: 'a payload whose shape is not usable',
        body: JSON.stringify({ action: 'assigned', issue: null }),
      },
      {
        event: 'issues',
        name: 'a payload that is not an object',
        body: JSON.stringify('a string where an object should be'),
      },
      {
        event: 'pull_request',
        name: 'a body the relay cannot read at all',
        body: UNREADABLE_BODY,
      },
    ];

    /** Posts a fixture the way GitHub would: with the hook named in the header. */
    function postFixture(webhook: NonCriticalWebhook) {
      return postWebhook(webhook.body).set('X-GitHub-Event', webhook.event);
    }

    it.each(NON_CRITICAL_WEBHOOKS)('acknowledges $name and stores nothing', async (webhook) => {
      const before = await storedEvents();

      const response = await postFixture(webhook).expect(200);
      expect(response.body).toEqual({ received: true });

      const after = await storedEvents();
      expect(after).toEqual(before);
      if (webhook.marker) {
        expect(after).not.toContainEqual(expect.objectContaining({ url: webhook.marker }));
      }
    });

    it('reports no new rows after the whole negative set is posted', async () => {
      const before = await storedEvents();

      for (const webhook of NON_CRITICAL_WEBHOOKS) {
        await postFixture(webhook).expect(200);
      }

      const after = await storedEvents();
      expect(after).toHaveLength(before.length);
      expect(after).toEqual(before);
    });

    it('rejects a body it cannot read that carries no signature with 401', async () => {
      await request(app.getHttpServer())
        .post('/')
        .set('Content-Type', 'application/json')
        .set('X-GitHub-Event', 'pull_request')
        .send(UNREADABLE_BODY)
        .expect(401);
    });

    it('rejects a body it cannot read signed with the wrong secret with 401', async () => {
      await postWebhook(UNREADABLE_BODY, sign(UNREADABLE_BODY, 'not-the-secret'))
        .set('X-GitHub-Event', 'pull_request')
        .expect(401);
    });
  });
});

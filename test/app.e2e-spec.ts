import { type INestApplication } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import { createHmac } from 'node:crypto';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import request from 'supertest';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { AppModule } from '../src/app.module.js';

const SECRET = 'test-secret';

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

function pullRequestPayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    action: 'closed',
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
      ...overrides,
    },
  };
}

let app: INestApplication;

describe('Relay HTTP seam', () => {
  const dbDir = mkdtempSync(join(tmpdir(), 'relay-test-'));

  beforeAll(async () => {
    process.env.GITHUB_WEBHOOK_SECRET = SECRET;
    process.env.DATABASE_PATH = join(dbDir, 'relay.db');
    const moduleRef = await Test.createTestingModule({ imports: [AppModule] }).compile();
    app = moduleRef.createNestApplication({ rawBody: true });
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
        assignee: '',
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
  });
});

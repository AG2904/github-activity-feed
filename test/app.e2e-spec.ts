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

/** The URL a pull request fixture is stored under, which is how a test finds it. */
const pullUrl = (number: number) => `https://github.com/AG2904/Gitlab-notifier/pull/${number}`;

/** The URL an issue fixture is stored under. */
const issueUrl = (number: number) =>
  `https://github.com/AG2904/Gitlab-notifier/issues/${number}`;

/**
 * A merge posted the way GitHub delivers it: the hook header, and the identifier
 * of the delivery that carried it. Passing no identifier posts a delivery that
 * arrived untagged.
 */
function postMerge(url: string, deliveryId?: string) {
  const delivery = postWebhook(JSON.stringify(pullRequestPayload({ html_url: url }))).set(
    'X-GitHub-Event',
    'pull_request',
  );
  return deliveryId ? delivery.set('X-GitHub-Delivery', deliveryId) : delivery;
}

/** The stored events pointing at one URL, which is the identity of a fixture. */
async function storedFor(url: string): Promise<unknown[]> {
  const events = await storedEvents();
  return events.filter((event) => (event as { url?: string }).url === url);
}

let app: NestExpressApplication;
let baseUrl: string;

describe('Relay HTTP seam', () => {
  const dbDir = mkdtempSync(join(tmpdir(), 'relay-test-'));

  beforeAll(async () => {
    process.env.GITHUB_WEBHOOK_SECRET = SECRET;
    process.env.DATABASE_PATH = join(dbDir, 'relay.db');
    const moduleRef = await Test.createTestingModule({ imports: [AppModule] }).compile();
    app = moduleRef.createNestApplication<NestExpressApplication>(INGRESS_OPTIONS);
    configureIngress(app);
    await app.init();
    // Listening on a real ephemeral port, because a held-open stream is not
    // something supertest can observe: its requests are answered, not streamed.
    await app.listen(0, '127.0.0.1');
    baseUrl = await app.getUrl();
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

  describe('a resent event is stored once (ticket 05)', () => {
    // Each fixture names its own delivery identifier, so a test can only ever
    // collide with itself: the suite shares one database, and an identifier names
    // a delivery rather than an event.

    it('stores the same delivery once, acknowledging each one as if it were fresh', async () => {
      const url = pullUrl(151);

      await postMerge(url, 'delivery-151').expect(200);

      // GitHub does not redeliver on its own, but a missed response budget gets
      // retried and its UI can redeliver by hand — same identifier, same delivery.
      const redelivery = await postMerge(url, 'delivery-151').expect(200);
      expect(redelivery.body).toEqual({ received: true });

      expect(await storedFor(url)).toHaveLength(1);
    });

    it('stores two different events posted back to back', async () => {
      const first = pullUrl(152);
      const second = pullUrl(153);

      await postMerge(first, 'delivery-152').expect(200);
      await postMerge(second, 'delivery-153').expect(200);

      expect(await storedFor(first)).toHaveLength(1);
      expect(await storedFor(second)).toHaveLength(1);
    });

    it('keeps one row when the same delivery arrives twice at once', async () => {
      const url = pullUrl(154);
      const delivery = 'delivery-154';

      // Both deliveries are in flight before either is answered. This cannot
      // distinguish a constraint-rejected insert from a read followed by a write:
      // the driver is synchronous, so the two writes are serialized on the event
      // loop and a reader-first implementation would pass this too. What it can
      // check is the contract — both acknowledged, exactly one row — and the
      // absence of a read is structural: `store` never looks before it inserts.
      const responses = await Promise.all([
        postMerge(url, delivery),
        postMerge(url, delivery),
      ]);

      expect(responses.map((response) => response.status)).toEqual([200, 200]);
      expect(await storedFor(url)).toHaveLength(1);
    });

    it('drops a repeated identifier even when the payload differs', async () => {
      const delivery = 'delivery-156';

      // The identifier names a delivery, not an event. A second delivery carrying
      // it is that delivery again, whatever its payload now says — which is also
      // what proves the header's value is the key actually stored on the row.
      await postMerge(pullUrl(156), delivery).expect(200);
      await postMerge(pullUrl(157), delivery).expect(200);

      expect(await storedFor(pullUrl(156))).toHaveLength(1);
      expect(await storedFor(pullUrl(157))).toEqual([]);
    });

    it('keeps an event that arrives with no delivery identifier, and cannot dedupe it', async () => {
      const url = pullUrl(158);

      // GitHub stamps every delivery, so this path is a safety net rather than a
      // normal one: an authenticated critical event is worth keeping even when
      // there is nothing to dedupe it by. The column is nullable for that reason,
      // which also means two untagged arrivals are two rows (ADR-0005 prefers a
      // duplicate to a dropped event).
      await postMerge(url).expect(200);
      await postMerge(url).expect(200);

      expect(await storedFor(url)).toHaveLength(2);
    });

    it('serves the stored event without the delivery identifier', async () => {
      const url = pullUrl(155);
      await postMerge(url, 'delivery-155').expect(200);

      const [stored] = await storedFor(url);
      // Deep equality on purpose: exactly seven fields reach the wire, so an
      // eighth column arriving there fails here (ADR-0005).
      expect(stored).toEqual({
        type: 'pr.merged',
        actor: 'maria.perez',
        subject: 'Fix memory leak in parser component',
        assignee: null,
        labels: ['bug', 'area:relay'],
        url,
        when: '2026-10-15T12:00:00.000Z',
      });
    });
  });

  describe('events stream to connected clients (ticket 06)', () => {
    /** Every connection this block opens, closed together at the end. */
    const connections = new AbortController();

    afterAll(() => connections.abort());

    /**
     * The stream, read one event at a time.
     *
     * A plain GET is the whole handshake (ADR-0001): no upgrade, no negotiation,
     * nothing sent back upstream. Waiting for the response is what tells the caller
     * the client is connected, and it is deterministic rather than merely likely —
     * Nest subscribes the route handler and only then commits the SSE headers on
     * the next macrotask, so bytes on the wire mean the subscription exists.
     */
    async function openStream(): Promise<() => Promise<Record<string, unknown>>> {
      const response = await fetch(new URL('/stream', baseUrl), {
        signal: connections.signal,
      });
      expect(response.status).toBe(200);
      expect(response.headers.get('content-type')).toContain('text/event-stream');

      const reader = response.body!.getReader();
      const decoder = new TextDecoder();
      let buffered = '';

      return async function nextEvent(): Promise<Record<string, unknown>> {
        for (;;) {
          const frameEnd = buffered.indexOf('\n\n');
          if (frameEnd === -1) {
            const { value, done } = await reader.read();
            if (done) {
              throw new Error('the stream closed before an event arrived');
            }
            buffered += decoder.decode(value, { stream: true });
            continue;
          }
          const frame = buffered.slice(0, frameEnd);
          buffered = buffered.slice(frameEnd + 2);
          // The bare newline Nest writes when it commits the headers, and any
          // comment frame, carry no data. A client ignores both; so does this.
          const data = frame
            .split('\n')
            .filter((line) => line.startsWith('data: '))
            .map((line) => line.slice('data: '.length))
            .join('\n');
          if (data !== '') {
            return JSON.parse(data) as Record<string, unknown>;
          }
        }
      };
    }

    it('holds a plain HTTP connection open and sends an event posted while it is connected', async () => {
      const first = pullUrl(161);
      const second = pullUrl(162);
      const nextEvent = await openStream();

      await postMerge(first, 'delivery-161').expect(200);
      // The streamed event and the stored one are the same event in the same
      // shape, so a client parses one thing (user story 16).
      expect(await nextEvent()).toEqual((await storedFor(first))[0]);

      // One connection, still open, still current: this is what holding it open
      // and emitting events as they are stored means.
      await postMerge(second, 'delivery-162').expect(200);
      expect(await nextEvent()).toMatchObject({ url: second });
    });

    it('delivers every stored event to every connected client', async () => {
      const url = pullUrl(163);
      const firstClient = await openStream();
      const secondClient = await openStream();

      await postMerge(url, 'delivery-163').expect(200);

      expect(await firstClient()).toMatchObject({ url });
      expect(await secondClient()).toMatchObject({ url });
    });

    it('does not replay what was stored before a client connected', async () => {
      const alreadyStored = pullUrl(164);
      const afterwards = pullUrl(165);

      await postMerge(alreadyStored, 'delivery-164').expect(200);

      const nextEvent = await openStream();
      await postMerge(afterwards, 'delivery-165').expect(200);

      // The channel is ordered, so the past would have arrived ahead of this one:
      // the stream is live, and the history is where the past lives.
      expect(await nextEvent()).toMatchObject({ url: afterwards });
    });
  });
});

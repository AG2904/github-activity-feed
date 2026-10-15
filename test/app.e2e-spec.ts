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

describe('Relay HTTP seam (ticket 01)', () => {
  let app: INestApplication;
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

  it('rejects a webhook that does not carry a signature with 401', async () => {
    await request(app.getHttpServer())
      .post('/')
      .set('Content-Type', 'application/json')
      .send('{}')
      .expect(401);
  });

  it('rejects a webhook signed with the wrong secret with 401', async () => {
    const body = JSON.stringify({ zen: 'design for failure' });
    await request(app.getHttpServer())
      .post('/')
      .set('Content-Type', 'application/json')
      .set('X-Hub-Signature-256', sign(body, 'not-the-secret'))
      .send(body)
      .expect(401);
  });

  it('rejects a webhook whose body does not match its signature with 401', async () => {
    const signedBody = JSON.stringify({ action: 'opened' });
    const sentBody = JSON.stringify({ action: 'closed' });
    await request(app.getHttpServer())
      .post('/')
      .set('Content-Type', 'application/json')
      .set('X-Hub-Signature-256', sign(signedBody))
      .send(sentBody)
      .expect(401);
  });

  it('rejects a malformed signature header with 401', async () => {
    const body = '{}';
    await request(app.getHttpServer())
      .post('/')
      .set('Content-Type', 'application/json')
      .set('X-Hub-Signature-256', 'deadbeef')
      .send(body)
      .expect(401);
  });

  it('acknowledges a webhook carrying a valid signature over the raw body', async () => {
    const body = JSON.stringify({ object_kind: 'push' });
    await request(app.getHttpServer())
      .post('/')
      .set('Content-Type', 'application/json')
      .set('X-Hub-Signature-256', sign(body))
      .send(body)
      .expect(200);
  });

  it('acknowledges a body signed byte-for-byte as sent, with non-canonical whitespace', async () => {
    const body = '{\n  "object_kind": "push"\n}';
    await request(app.getHttpServer())
      .post('/')
      .set('Content-Type', 'application/json')
      .set('X-Hub-Signature-256', sign(body))
      .send(body)
      .expect(200);
  });

  it('rejects a well-formed digest that is not valid hex with 401', async () => {
    const body = '{}';
    await request(app.getHttpServer())
      .post('/')
      .set('Content-Type', 'application/json')
      .set('X-Hub-Signature-256', `sha256=${'z'.repeat(64)}`)
      .send(body)
      .expect(401);
  });

  it('serves an empty audit history before anything is stored', async () => {
    const response = await request(app.getHttpServer()).get('/history').expect(200);
    expect(response.body).toEqual([]);
  });
});

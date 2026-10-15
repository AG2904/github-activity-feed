import { type INestApplication } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import request from 'supertest';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { AppModule } from '../src/app.module.js';

describe('Relay HTTP seam (ticket 01)', () => {
  let app: INestApplication;
  const dbDir = mkdtempSync(join(tmpdir(), 'relay-test-'));

  beforeAll(async () => {
    process.env.GITLAB_WEBHOOK_SECRET = 'test-secret';
    process.env.DATABASE_PATH = join(dbDir, 'relay.db');
    const moduleRef = await Test.createTestingModule({ imports: [AppModule] }).compile();
    app = moduleRef.createNestApplication();
    await app.init();
  });

  afterAll(async () => {
    await app?.close();
    rmSync(dbDir, { recursive: true, force: true });
  });

  it('rejects a webhook that does not carry a token with 401', async () => {
    await request(app.getHttpServer()).post('/').send({}).expect(401);
  });

  it('rejects a webhook carrying the wrong token with 401', async () => {
    await request(app.getHttpServer())
      .post('/')
      .set('X-Gitlab-Token', 'not-the-secret')
      .send({})
      .expect(401);
  });

  it('acknowledges a webhook carrying the correct token', async () => {
    await request(app.getHttpServer())
      .post('/')
      .set('X-Gitlab-Token', 'test-secret')
      .send({})
      .expect(200);
  });

  it('serves an empty audit history before anything is stored', async () => {
    const response = await request(app.getHttpServer()).get('/history').expect(200);
    expect(response.body).toEqual([]);
  });
});

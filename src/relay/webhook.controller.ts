import { Controller, Headers, HttpCode, Post, Req, UnauthorizedException } from '@nestjs/common';
import type { RawBodyRequest } from '@nestjs/common';
import type { Request } from 'express';
import { createHmac, timingSafeEqual } from 'node:crypto';
import { ActivityEventsService } from './activity-events.service.js';
import { ActivityStream } from './activity-stream.js';
import { ingest } from './github/to-activity-event.js';

const SIGNATURE_HEADER = 'x-hub-signature-256';
const SIGNATURE_PREFIX = 'sha256=';
const DELIVERY_HEADER = 'x-github-delivery';

/**
 * Verifies GitHub's X-Hub-Signature-256: an HMAC-SHA256 of the raw request
 * body, hex-encoded and prefixed with "sha256=". Fails closed on a missing
 * body, header, or secret, and compares the fixed-length hex digests in
 * constant time so the comparison never short-circuits.
 */
function signatureMatches(
  rawBody: Buffer | undefined,
  signature: string | undefined,
  secret: string,
): boolean {
  if (!rawBody || !signature || !secret || !signature.startsWith(SIGNATURE_PREFIX)) {
    return false;
  }
  const provided = signature.slice(SIGNATURE_PREFIX.length);
  const expected = createHmac('sha256', secret).update(rawBody).digest('hex');
  if (provided.length !== expected.length) {
    return false;
  }
  return timingSafeEqual(Buffer.from(provided, 'utf8'), Buffer.from(expected, 'utf8'));
}

/**
 * The request body, and the two ways it can fail to be one. A missing body and a
 * body that is not JSON used to answer the same `undefined`, which made a sender
 * that broke its serialization indistinguishable from a delivery carrying nothing
 * worth reporting (ADR-0015).
 */
type Body = { kind: 'parsed'; payload: unknown } | { kind: 'unparseable' };

function parseBody(rawBody: Buffer | undefined): Body {
  if (!rawBody) {
    return { kind: 'unparseable' };
  }
  try {
    return { kind: 'parsed', payload: JSON.parse(rawBody.toString('utf8')) };
  } catch {
    return { kind: 'unparseable' };
  }
}

@Controller()
export class WebhookController {
  private readonly secret: string;

  constructor(
    private readonly events: ActivityEventsService,
    private readonly stream: ActivityStream,
  ) {
    this.secret = process.env.GITHUB_WEBHOOK_SECRET ?? '';
  }

  /**
   * Verifies the signature over the raw bytes, and parses only after that. A
   * malformed, non-critical, or unrecognized payload is acknowledged and
   * discarded — never rejected, and never stored (ADR-0002, ticket 04). The
   * adapter names why it produced nothing (ADR-0015); nothing here consumes that
   * reason yet, and the answer to the sender is the same in every case.
   */
  @Post()
  @HttpCode(200)
  async accept(
    @Req() req: RawBodyRequest<Request>,
    @Headers(SIGNATURE_HEADER) signature?: string,
    @Headers(DELIVERY_HEADER) deliveryId?: string,
  ): Promise<{ received: true }> {
    if (!signatureMatches(req.rawBody, signature, this.secret)) {
      throw new UnauthorizedException('invalid or missing X-Hub-Signature-256');
    }
    const body = parseBody(req.rawBody);
    if (body.kind === 'unparseable') {
      return { received: true };
    }
    const ingestion = ingest(body.payload);
    if (ingestion.kind === 'event') {
      // A missing header and an empty one both mean the delivery arrived untagged.
      const stored = await this.events.store(ingestion.event, deliveryId || null);
      if (stored) {
        // A redelivery was announced when it first arrived, so it must not reach
        // the stream a second time.
        this.stream.deliver(ingestion.event);
      }
    }
    return { received: true };
  }
}

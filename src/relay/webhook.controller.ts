import { Controller, Headers, HttpCode, Post, Req, UnauthorizedException } from '@nestjs/common';
import type { RawBodyRequest } from '@nestjs/common';
import type { Request } from 'express';
import { createHmac, timingSafeEqual } from 'node:crypto';

const SIGNATURE_HEADER = 'x-hub-signature-256';
const SIGNATURE_PREFIX = 'sha256=';

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

@Controller()
export class WebhookController {
  private readonly secret: string;

  constructor() {
    this.secret = process.env.GITHUB_WEBHOOK_SECRET ?? '';
  }

  @Post()
  @HttpCode(200)
  accept(
    @Req() req: RawBodyRequest<Request>,
    @Headers(SIGNATURE_HEADER) signature?: string,
  ): { received: true } {
    if (!signatureMatches(req.rawBody, signature, this.secret)) {
      throw new UnauthorizedException('invalid or missing X-Hub-Signature-256');
    }
    return { received: true };
  }
}

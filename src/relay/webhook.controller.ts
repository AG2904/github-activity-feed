import { Controller, Headers, HttpCode, Post, UnauthorizedException } from '@nestjs/common';
import { createHash, timingSafeEqual } from 'node:crypto';

/**
 * Compares the presented token against the shared secret in constant time.
 * Both sides are SHA-256 hashed first so the comparison never leaks the
 * secret's length and never short-circuits on the first differing byte.
 * An absent token or an empty secret always fails closed.
 */
function tokenMatchesSecret(token: string | undefined, secret: string): boolean {
  if (!token || !secret) {
    return false;
  }
  const tokenHash = createHash('sha256').update(token).digest();
  const secretHash = createHash('sha256').update(secret).digest();
  return timingSafeEqual(tokenHash, secretHash);
}

@Controller()
export class WebhookController {
  private readonly secret: string;

  constructor() {
    this.secret = process.env.GITLAB_WEBHOOK_SECRET ?? '';
  }

  @Post()
  @HttpCode(200)
  accept(@Headers('x-gitlab-token') token?: string): { received: true } {
    if (!tokenMatchesSecret(token, this.secret)) {
      throw new UnauthorizedException('invalid or missing X-Gitlab-Token');
    }
    return { received: true };
  }
}

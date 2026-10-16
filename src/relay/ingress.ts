import type { NestApplicationOptions } from '@nestjs/common';
import type { NestExpressApplication } from '@nestjs/platform-express';

/**
 * The options the relay must be created with so that the ingress sees the bytes
 * GitHub signed (ADR-0002).
 *
 * `bodyParser: false` removes Nest's JSON and urlencoded parsers, which read the
 * body *before* the controller can verify anything about it. Left in place they
 * answer `400` for a body they cannot parse — and `json`'s `strict` mode rejects
 * even a perfectly valid JSON scalar — so a webhook the relay is meant to
 * acknowledge and discard would instead be reported to GitHub as a failed
 * delivery, with its signature never checked. `rawBody: true` keeps those exact
 * bytes on `req.rawBody`, which is what the HMAC is computed over.
 */
export const INGRESS_OPTIONS: NestApplicationOptions = { rawBody: true, bodyParser: false };

/**
 * Registers the one parser the ingress needs: the raw body, for every content
 * type. The controller verifies the signature over those bytes and only then
 * parses them, so an unreadable body is discarded rather than rejected.
 *
 * `main.ts` and the e2e harness both call this, so production and tests cannot
 * drift on the single thing this wiring exists for.
 */
export function configureIngress(app: NestExpressApplication): void {
  app.useBodyParser('raw', { type: '*/*' });
}

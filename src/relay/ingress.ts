import type { NestApplicationOptions } from '@nestjs/common';
import type { NestExpressApplication } from '@nestjs/platform-express';

/**
 * The options the relay must be created with, for two reasons that share one
 * requirement: production and the e2e harness must not drift.
 *
 * **The signed bytes** (ADR-0002). `bodyParser: false` removes Nest's JSON and
 * urlencoded parsers, which read the body *before* the controller can verify
 * anything about it. Left in place they answer `400` for a body they cannot
 * parse — and `json`'s `strict` mode rejects even a perfectly valid JSON scalar
 * — so a webhook the relay is meant to acknowledge and discard would instead be
 * reported to GitHub as a failed delivery, with its signature never checked.
 * `rawBody: true` keeps those exact bytes on `req.rawBody`, which is what the
 * HMAC is computed over.
 *
 * **Stopping.** `forceCloseConnections` makes `close()` finish even when a
 * client is holding an HTTP connection open. The stream completes on shutdown
 * (ActivityStream) so a client that is reading sees its stream end; this covers
 * the socket that outlives its response, which without it holds
 * `server.close()` open until the container runtime gives up and kills the
 * process.
 */
export const INGRESS_OPTIONS: NestApplicationOptions = {
  rawBody: true,
  bodyParser: false,
  forceCloseConnections: true,
};

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

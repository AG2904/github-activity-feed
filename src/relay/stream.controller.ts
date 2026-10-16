import { Controller, Sse } from '@nestjs/common';
import type { MessageEvent } from '@nestjs/common';
import { map } from 'rxjs';
import type { Observable } from 'rxjs';
import { ActivityStream } from './activity-stream.js';

@Controller()
export class StreamController {
  constructor(private readonly stream: ActivityStream) {}

  /**
   * The realtime channel (ADR-0001): a client holds this connection open and
   * receives every event the relay stores while it is connected. It is a plain
   * SSE GET — no upgrade, no negotiation, and nothing ever sent back upstream.
   * The connection is answered on the request itself, before any event exists:
   * Nest commits the response headers on the next macrotask after subscribing.
   *
   * An event travels as the message's `data`, never as the message itself: an
   * ActivityEvent carries its own `type`, which an SSE message reads as the event
   * *name* — the frame would come out as `event: pr.merged` with no `data` line at
   * all, and the payload would never reach the client. Wrapping it also keeps the
   * per-connection `id` Nest writes off the event object.
   */
  @Sse('stream')
  events(): Observable<MessageEvent> {
    return this.stream.receive().pipe(map((event) => ({ data: event })));
  }
}

import { Controller, Sse } from '@nestjs/common';
import type { MessageEvent } from '@nestjs/common';
import { map, merge } from 'rxjs';
import type { Observable } from 'rxjs';
import { ActivityStream } from './activity-stream.js';
import { heartbeats } from './stream-heartbeat.js';

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
   *
   * A heartbeat is merged in beside the events, and travels as a `comment` for the
   * same reason read the other way: a comment frame carries no data, so a client
   * skips it by rule and cannot mistake it for an ActivityEvent. The two streams
   * share the connection without either knowing about the other (stream-heartbeat).
   */
  @Sse('stream')
  events(): Observable<MessageEvent> {
    const events = this.stream.receive();

    return merge(
      events.pipe(map((event) => ({ data: event }))),
      heartbeats(events),
    );
  }
}

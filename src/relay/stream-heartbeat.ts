import { map, merge, of, switchMap, timer } from 'rxjs';
import type { MessageEvent } from '@nestjs/common';
import type { Observable } from 'rxjs';

/** The comment frame the stream writes when it has nothing else to say. */
const HEARTBEAT_COMMENT = 'heartbeat';

/** What an unset, empty, or unusable `STREAM_HEARTBEAT_MS` falls back to. */
const DEFAULT_HEARTBEAT_MS = 15_000;

/**
 * How often an idle stream announces that it is alive, in milliseconds.
 *
 * A proxy reaps a connection that has carried no bytes for long enough, and the
 * documented defaults are tight: AWS's load-balancer idle timeout is 60 seconds
 * and nginx's `proxy_read_timeout` is the same, so a stream that says nothing for
 * a minute is a stream that dies. Fifteen seconds is four times under that, which
 * leaves room for one delayed frame without costing the client its connection.
 *
 * The timeout belongs to the deployment rather than to the relay, so the interval
 * is configurable. A value that is missing, empty, or not a positive number is
 * treated as unset and the default stands: unlike the webhook secret, which has no
 * safe default and stops the relay booting, a mistyped interval has one. The one
 * thing it must never do is reach `timer` as a zero, which would write heartbeat
 * frames as fast as the socket would accept them.
 */
export function heartbeatMs(): number {
  const configured = Number(process.env.STREAM_HEARTBEAT_MS);
  return Number.isFinite(configured) && configured > 0 ? configured : DEFAULT_HEARTBEAT_MS;
}

/**
 * The liveness half of the stream: a comment frame whenever the connection would
 * otherwise be idle.
 *
 * It travels as a `comment` rather than as `data` so that no client can mistake it
 * for an ActivityEvent — a comment frame carries no data, and a client skips it by
 * rule. That is what lets liveness and the event stream share one connection.
 *
 * Every event restarts the periodic timer, so a stream carrying events never
 * writes a heartbeat and a stream gone quiet writes one per interval. The
 * `of(null)` starts the first timer, because a client connecting to a relay with
 * nothing to say has no event to restart it.
 */
export function heartbeats<T>(activity: Observable<T>): Observable<MessageEvent> {
  return merge(of(null), activity).pipe(
    switchMap(() => timer(heartbeatMs(), heartbeatMs())),
    map(() => ({ comment: HEARTBEAT_COMMENT })),
  );
}

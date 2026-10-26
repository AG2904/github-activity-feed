import { Injectable } from '@nestjs/common';
import type { OnModuleDestroy } from '@nestjs/common';
import { Observable, Subject } from 'rxjs';
import type { ActivityEvent } from './domain/activity-event.js';

/**
 * The relay's broadcast (see GLOSSARY.md): every connected client receives every
 * ActivityEvent and filters for itself, because the channel carries no identity
 * and no authentication (ADR-0001).
 *
 * It holds no history. An event stored while nobody was listening is not replayed
 * to the next client that connects — the audit history is what reconstructs the
 * past, and this is only what is happening now.
 */
@Injectable()
export class ActivityStream implements OnModuleDestroy {
  private readonly events = new Subject<ActivityEvent>();

  /**
   * Hands a stored event to every client listening right now. The stream names
   * this "deliver" rather than "publish" because the glossary's word is *receive*:
   * every connected client receives every event and filters for itself.
   */
  deliver(event: ActivityEvent): void {
    this.events.next(event);
  }

  /** The channel as a client sees it: read-only, one subscription per connection. */
  receive(): Observable<ActivityEvent> {
    return this.events.asObservable();
  }

  /**
   * Closing the broadcast is what lets the process stop.
   *
   * An SSE client never ends its own connection, so closing the HTTP server
   * would wait for a request that has no end. Completing the Subject ends every
   * open stream first, and a `docker stop` becomes a clean shutdown instead of a
   * ten-second wait followed by SIGKILL.
   */
  onModuleDestroy(): void {
    this.events.complete();
  }
}

import { Injectable } from '@nestjs/common';
import { Observable, Subject } from 'rxjs';
import type { ActivityEvent } from './domain/activity-event.js';

/**
 * The relay's broadcast (see GLOSSARY.md): every connected CLI receives every
 * ActivityEvent and filters for itself, because the channel carries no identity
 * and no authentication (ADR-0001).
 *
 * It holds no history. An event stored while nobody was listening is not replayed
 * to the next client that connects — the audit history is what reconstructs the
 * past, and this is only what is happening now.
 */
@Injectable()
export class ActivityStream {
  private readonly events = new Subject<ActivityEvent>();

  /**
   * Hands a stored event to every client listening right now. The stream names
   * this "deliver" rather than "publish" because the glossary's word is *receive*:
   * every connected CLI receives every event and filters for itself.
   */
  deliver(event: ActivityEvent): void {
    this.events.next(event);
  }

  /** The channel as a client sees it: read-only, one subscription per connection. */
  receive(): Observable<ActivityEvent> {
    return this.events.asObservable();
  }
}

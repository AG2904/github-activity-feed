import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { QueryFailedError, Repository } from 'typeorm';
import { ActivityEventEntity } from './activity-event.entity.js';
import type { ActivityEvent, ActivityEventType, IsoInstant } from './domain/activity-event.js';

/**
 * Storage row -> domain event. Named rather than inlined because this is the
 * one place the two shapes meet: the row has no opinion, the domain's types do.
 *
 * The two assertions are the storage trust boundary, and they are the only
 * ones in the relay: the row is written by the adapter alone, through
 * `isoInstant` and the closed type set, so re-deriving either here would be
 * validating our own writer on every read.
 */
function fromStoredRow(row: ActivityEventEntity): ActivityEvent {
  return {
    type: row.type as ActivityEventType,
    actor: row.actor,
    subject: row.subject,
    assignee: row.assignee,
    labels: row.labels,
    url: row.url,
    when: row.when as IsoInstant,
  };
}

/** SQLite's code for a row refused by a unique index. */
const UNIQUE_VIOLATION = 'SQLITE_CONSTRAINT_UNIQUE';

/**
 * Whether the row was refused because that delivery identifier is already
 * stored. Matched on the driver's own code rather than on a message, and the
 * delivery identifier is the table's only unique column, so any other refusal
 * still surfaces instead of being mistaken for a redelivery.
 */
function isDuplicateDelivery(error: unknown): boolean {
  const driverError = error instanceof QueryFailedError ? error.driverError : undefined;
  return (driverError as { code?: string } | undefined)?.code === UNIQUE_VIOLATION;
}

@Injectable()
export class ActivityEventsService {
  constructor(
    @InjectRepository(ActivityEventEntity)
    private readonly events: Repository<ActivityEventEntity>,
  ) {}

  /**
   * Stores an event under the delivery that carried it, and answers whether the
   * row was written. A redelivery collides with the row it already wrote, on the
   * delivery identifier's unique index, and that collision is the entire dedupe:
   * the row is dropped here, the caller answers success anyway, and a duplicate is
   * indistinguishable from a fresh store in the response (ADR-0005).
   *
   * The answer is what tells the two apart where it matters: only an event that
   * was actually stored may reach the stream.
   *
   * Nothing is read before the write. Reading first would mean two deliveries
   * that arrive together both find nothing and both insert, which is exactly the
   * case the constraint exists to settle.
   */
  async store(event: ActivityEvent, deliveryId: string | null): Promise<boolean> {
    try {
      await this.events.save(this.events.create({ ...event, deliveryId }));
      return true;
    } catch (error) {
      if (isDuplicateDelivery(error)) {
        return false;
      }
      throw error;
    }
  }

  /** Audit history, newest first, in the ActivityEvent shape (no storage id leaks). */
  async list(): Promise<ActivityEvent[]> {
    const rows = await this.events.find({ order: { id: 'DESC' } });
    return rows.map(fromStoredRow);
  }
}

import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
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

@Injectable()
export class ActivityEventsService {
  constructor(
    @InjectRepository(ActivityEventEntity)
    private readonly events: Repository<ActivityEventEntity>,
  ) {}

  async store(event: ActivityEvent): Promise<void> {
    await this.events.save(this.events.create(event));
  }

  /** Audit history, newest first, in the ActivityEvent shape (no storage id leaks). */
  async list(): Promise<ActivityEvent[]> {
    const rows = await this.events.find({ order: { id: 'DESC' } });
    return rows.map(fromStoredRow);
  }
}

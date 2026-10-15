import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { ActivityEventEntity } from './activity-event.entity.js';
import type { ActivityEvent } from './domain/activity-event.js';

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
    return rows.map((row) => ({
      type: row.type,
      actor: row.actor,
      subject: row.subject,
      assignee: row.assignee,
      labels: row.labels,
      url: row.url,
      when: row.when,
    }));
  }
}

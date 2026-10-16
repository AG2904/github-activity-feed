import { Controller, Get, Query } from '@nestjs/common';
import { ActivityEventsService } from './activity-events.service.js';
import type { ActivityEvent } from './domain/activity-event.js';

@Controller()
export class HistoryController {
  constructor(private readonly events: ActivityEventsService) {}

  /**
   * The audit history, whole or narrowed to one user. The same normalized shape
   * either way, and the same one the stream carries, so a client parses one thing
   * (story 16).
   *
   * No filter and an empty one mean the same request: there is no user to narrow
   * by, so the whole record comes back. The rule itself lives in the service.
   */
  @Get('history')
  listAuditHistory(@Query('user') user?: string): Promise<ActivityEvent[]> {
    return this.events.list(user);
  }
}

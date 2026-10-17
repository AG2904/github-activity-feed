import { Controller, Get, Query } from '@nestjs/common';
import { ActivityEventsService } from './activity-events.service.js';
import type { ActivityEvent } from './domain/activity-event.js';

@Controller()
export class HistoryController {
  constructor(private readonly events: ActivityEventsService) {}

  /**
   * The audit history, whole or narrowed to one user, one repository, or both.
   * The same normalized shape either way, and the same one the stream carries, so
   * a client parses one thing (story 16).
   *
   * No filter and an empty one mean the same request: there is nothing to narrow
   * by, so the whole record comes back. The rules themselves live in the service.
   */
  @Get('history')
  listAuditHistory(
    @Query('user') user?: string,
    @Query('repository') repository?: string,
  ): Promise<ActivityEvent[]> {
    return this.events.list({ user, repository });
  }
}

import { Controller, Get } from '@nestjs/common';
import { ActivityEventsService } from './activity-events.service.js';
import type { ActivityEvent } from './domain/activity-event.js';

@Controller()
export class HistoryController {
  constructor(private readonly events: ActivityEventsService) {}

  @Get('history')
  listAuditHistory(): Promise<ActivityEvent[]> {
    return this.events.list();
  }
}

import { Controller, Get } from '@nestjs/common';

@Controller()
export class HistoryController {
  @Get('history')
  listAuditHistory(): unknown[] {
    return [];
  }
}

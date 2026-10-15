import { Module } from '@nestjs/common';
import { HistoryController } from './history.controller.js';
import { WebhookController } from './webhook.controller.js';

@Module({
  controllers: [WebhookController, HistoryController],
})
export class RelayModule {}

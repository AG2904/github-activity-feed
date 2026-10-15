import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { ActivityEventEntity } from './activity-event.entity.js';
import { ActivityEventsService } from './activity-events.service.js';
import { HistoryController } from './history.controller.js';
import { WebhookController } from './webhook.controller.js';

@Module({
  imports: [TypeOrmModule.forFeature([ActivityEventEntity])],
  controllers: [WebhookController, HistoryController],
  providers: [ActivityEventsService],
})
export class RelayModule {}

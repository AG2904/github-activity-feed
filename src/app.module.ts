import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import type { DataSourceOptions } from 'typeorm';
import { RelayModule } from './relay/relay.module.js';

@Module({
  imports: [
    TypeOrmModule.forRootAsync({
      useFactory: (): DataSourceOptions => ({
        type: 'better-sqlite3',
        database: process.env.DATABASE_PATH ?? 'relay.db',
        entities: [],
        synchronize: false,
      }),
    }),
    RelayModule,
  ],
})
export class AppModule {
  constructor() {
    if (!process.env.GITHUB_WEBHOOK_SECRET) {
      throw new Error('GITHUB_WEBHOOK_SECRET is not set; the relay refuses to boot.');
    }
  }
}

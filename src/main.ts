import { NestFactory } from '@nestjs/core';
import type { NestExpressApplication } from '@nestjs/platform-express';
import { AppModule } from './app.module.js';
import { loadEnvFile } from './env.js';
import { configureIngress, INGRESS_OPTIONS } from './relay/ingress.js';

async function bootstrap() {
  loadEnvFile();
  const app = await NestFactory.create<NestExpressApplication>(AppModule, INGRESS_OPTIONS);
  configureIngress(app);
  await app.listen(process.env.PORT ?? 3000);
}
await bootstrap();

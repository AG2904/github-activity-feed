import { NestFactory } from '@nestjs/core';
import type { NestExpressApplication } from '@nestjs/platform-express';
import { AppModule } from './app.module.js';
import { loadEnvFile } from './env.js';
import { configureIngress, INGRESS_OPTIONS } from './relay/ingress.js';

async function bootstrap() {
  loadEnvFile();
  const app = await NestFactory.create<NestExpressApplication>(AppModule, INGRESS_OPTIONS);
  configureIngress(app);
  // SIGTERM is how a container asks the process to stop. Without this, Node's
  // default handler ends the process where it stands: open streams are severed
  // and the database is never closed. With it, `docker stop` runs the shutdown
  // path the e2e suite exercises.
  app.enableShutdownHooks();
  await app.listen(process.env.PORT ?? 3000);
}
await bootstrap();

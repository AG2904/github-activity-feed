# TypeORM over SQLite as the persistence layer

The relay persists ActivityEvents to a local SQLite file through TypeORM, wired with the `@nestjs/typeorm` integration.

## Considered Options

- **Prisma**: better developer experience, but it ships a native query-engine binary to install and keep in sync — exactly the kind of friction a TI-restricted environment punishes.
- **Drizzle**: lighter, typed, no codegen, but not idiomatic in a NestJS codebase.

## Consequences

There is one table and a handful of queries, so the deciding factor is familiarity: TypeORM is the NestJS-default data layer the team already reads fluently. SQLite keeps deployment to a single file with no database server for TI to approve.

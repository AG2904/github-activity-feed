import { Column, Entity, PrimaryGeneratedColumn } from 'typeorm';

/** Stored record of an ActivityEvent: the relay's audit history. */
@Entity('activity_events')
export class ActivityEventEntity {
  @PrimaryGeneratedColumn()
  id!: number;

  /**
   * GitHub's per-delivery identifier, unique so a redelivery cannot become a
   * second row. It is a storage column and never a field of the ActivityEvent
   * shape (ADR-0005): this identifies a *delivery*, not a happening. Nullable,
   * because an event that arrives without one is still a happening.
   */
  @Column({ type: 'text', nullable: true, unique: true })
  deliveryId!: string | null;

  @Column()
  type!: string;

  /**
   * The repository the happening belongs to, as `owner/name`.
   *
   * Nullable, and only because of the upgrade: SQLite cannot add a `NOT NULL`
   * column to a populated table, and TypeORM answers a schema change by recreating
   * the table and copying the rows, so a `NOT NULL` column makes the copy fail and
   * the relay refuse to boot. A row written before this column existed has no
   * recorded repository, and deriving one from its `url` would only work for URLs
   * of the shape we happen to produce, so the honest answer for those rows is that
   * it is unknown.
   */
  @Column({ type: 'text', nullable: true })
  repository!: string | null;

  /**
   * The source branch that landed, from the pull request's `head.ref`.
   *
   * Nullable, and only because of the upgrade, exactly like `repository`:
   * SQLite cannot add a `NOT NULL` column to a populated table, and TypeORM
   * answers a schema change by recreating the table and copying the rows, so a
   * `NOT NULL` column makes the copy fail and the relay refuse to boot. A row
   * written before this column existed has no recorded branch, and a merge
   * whose payload carries no usable head ref has none to record (ADR-0013).
   */
  @Column({ type: 'text', nullable: true })
  branch!: string | null;

  @Column()
  actor!: string;

  @Column()
  subject!: string;

  @Column({ type: 'text', nullable: true })
  assignee!: string | null;

  @Column({ type: 'json' })
  labels!: string[];

  @Column()
  url!: string;

  @Column()
  when!: string;
}

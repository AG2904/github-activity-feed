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

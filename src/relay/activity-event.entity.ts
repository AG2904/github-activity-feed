import { Column, Entity, PrimaryGeneratedColumn } from 'typeorm';

/** Stored record of an ActivityEvent: the relay's audit history. */
@Entity('activity_events')
export class ActivityEventEntity {
  @PrimaryGeneratedColumn()
  id!: number;

  @Column()
  type!: string;

  @Column()
  actor!: string;

  @Column()
  subject!: string;

  @Column()
  assignee!: string;

  @Column({ type: 'json' })
  labels!: string[];

  @Column()
  url!: string;

  @Column()
  when!: string;
}

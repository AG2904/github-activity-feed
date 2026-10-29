import type { ActivityEvent } from '../domain/activity-event.js';

/**
 * What the GitHub adapter made of one delivered payload: the event, or the
 * reason there is none (ADR-0015).
 *
 * The two ways of producing nothing are named apart on purpose. `not-critical`
 * is a decision — the filter read the payload and passed on it, which is the
 * relay working as designed. `malformed` is a defect: the payload passed a
 * filter and a field the event needs was missing or unusable, so this adapter
 * and GitHub disagree about a shape. Only the second one is worth a second look,
 * and `undefined` could not tell them apart.
 *
 * The wire cannot tell them apart either: every authenticated delivery is
 * acknowledged with the same success code whether it stored, refused or
 * duplicated (ADR-0002). This type is therefore the only place the difference
 * survives, and every consumer has to say which of the two it is handling.
 */
export type Ingestion =
  | { kind: 'event'; event: ActivityEvent }
  | { kind: 'ignored'; reason: IngestionRefusal };

/**
 * Why a payload produced no event.
 *
 * `not-critical` is the common case and says nothing about the relay's health.
 * `malformed` means a payload a filter accepted could not be normalized, so it
 * is the one worth reporting wherever refusals are counted.
 */
export type IngestionRefusal = 'not-critical' | 'malformed';

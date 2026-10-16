/**
 * The single normalized shape the relay stores and serves (see GLOSSARY.md).
 *
 * Seven fields, no more. An ActivityEvent describes a *happening* — what
 * happened, to whom, where to look, and when — so the identifiers that belong
 * to a delivery or to a storage row deliberately do not appear here (ADR-0005).
 */

/** The critical happenings the relay knows how to normalize. A closed set. */
export type ActivityEventType = 'pr.merged' | 'issue.assigned';

declare const isoInstantBrand: unique symbol;

/**
 * A UTC ISO-8601 instant in the project's one canonical form — what
 * `Date#toISOString` produces, milliseconds included. Nominal, so an arbitrary
 * string cannot be passed where a canonical instant is expected without going
 * through `isoInstant` first.
 */
export type IsoInstant = string & { readonly [isoInstantBrand]: 'IsoInstant' };

/**
 * The canonical instant for a timestamp reported to the relay. Returns
 * `undefined` when the input is missing or unparseable, so a malformed payload
 * is ignored rather than failing the request. This is the only producer.
 */
export function isoInstant(source: unknown): IsoInstant | undefined {
  if (typeof source !== 'string' || source === '') {
    return undefined;
  }
  const date = new Date(source);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  return date.toISOString() as IsoInstant;
}

export interface ActivityEvent {
  type: ActivityEventType;
  actor: string;
  subject: string;
  /** The user the issue was assigned to; `null` for an event that has none. */
  assignee: string | null;
  labels: string[];
  url: string;
  when: IsoInstant;
}

/**
 * The single normalized shape the relay stores and serves (see GLOSSARY.md).
 *
 * Nine fields, no more. An ActivityEvent describes a *happening* — what
 * happened, where it happened and which line of work it came from, to whom,
 * where to look, and when — so the identifiers that belong to a delivery or
 * to a storage row deliberately do not appear here (ADR-0005).
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
  /**
   * The repository the happening belongs to, as `owner/name`.
   *
   * Null only for an event stored before the relay recorded one: the column
   * arrived after the audit history did, so a row that predates it has no
   * repository to report rather than one to guess at (ticket 09).
   */
  repository: string | null;
  /**
   * The source branch that landed, from the pull request's `head.ref`.
   *
   * Null for an event that has no branch to name: an assignment, a merge whose
   * payload carried no usable head ref, or a row stored before the relay
   * recorded one (ADR-0013). Supplementary, never load-bearing: an event is
   * still an event without it.
   */
  branch: string | null;
  actor: string;
  subject: string;
  /** The user the issue was assigned to; `null` for an event that has none. */
  assignee: string | null;
  labels: string[];
  url: string;
  when: IsoInstant;
}

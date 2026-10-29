/**
 * Helpers for reading GitHub's payloads, so that each adapter describes *which*
 * fields it needs and not *how* to safely reach them.
 *
 * Everything here takes `unknown` and answers with a value or nothing: GitHub
 * can and does send partial payloads, and a malformed one has to be ignored
 * without error rather than fail the request.
 */

// The envelope every webhook carries, whatever it announces, is a comment rather
// than a type: nothing narrows a whole payload any more — each adapter reads the
// envelope it needs and answers an `Ingestion` (ADR-0015), so a shared interface
// would only describe fields no reader has verified. A delivery is an action, the
// sender who caused it, and the repository it happened in; the repository is read
// through `repositoryName` below.

/** A non-null object, the precondition every payload walk starts from. */
export function asRecord(value: unknown): Record<string, unknown> | undefined {
  return typeof value === 'object' && value !== null
    ? (value as Record<string, unknown>)
    : undefined;
}

/** The `login` of a GitHub user object, when there is a usable one. */
export function loginOf(user: unknown): string | undefined {
  const login = asRecord(user)?.login;
  return typeof login === 'string' && login !== '' ? login : undefined;
}

/** GitHub's label objects, flattened to plain names. */
export function labelNames(labels: unknown): string[] {
  if (!Array.isArray(labels)) {
    return [];
  }
  return labels.flatMap((label) => {
    const name = asRecord(label)?.name;
    return typeof name === 'string' ? [name] : [];
  });
}

/**
 * GitHub's repository object, as the `owner/name` a happening is filed under.
 *
 * Every webhook carries it, and it is the only honest source for *where* an event
 * happened: the `url` happens to contain the same two path segments, but a link is
 * not a data carrier (ticket 09, and the reversal of ADR-0006).
 */
export function repositoryName(repository: unknown): string | null {
  const fullName = asRecord(repository)?.full_name;
  return typeof fullName === 'string' && fullName !== '' ? fullName : null;
}

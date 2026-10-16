/**
 * Helpers for reading GitHub's payloads, so that each adapter describes *which*
 * fields it needs and not *how* to safely reach them.
 *
 * Everything here takes `unknown` and answers with a value or nothing: GitHub
 * can and does send partial payloads, and a malformed one has to be ignored
 * without error rather than fail the request.
 */

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

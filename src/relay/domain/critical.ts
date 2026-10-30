/**
 * The domain's definition of a critical event (see GLOSSARY.md): the branch
 * whose merges are integrations, and the labels that make an issue event worth
 * interrupting someone for.
 *
 * These are the product's rules, not GitHub's field paths. The adapter
 * extracts `base.ref` and the label names out of the payload and asks these
 * whether the result is critical, so that "what makes an event critical" is
 * answered in one place and without reading JSON.
 */

/** An issue-side event is critical only while the issue carries one of these. */
export const CRITICAL_LABELS: readonly string[] = ['bug', 'feedback'];

/** A merge is critical only when its target is this branch. */
export const MAIN_BRANCH = 'main';

export function isMainBranch(target: unknown): boolean {
  return target === MAIN_BRANCH;
}

export function carriesCriticalLabel(labels: readonly string[]): boolean {
  return labels.some((label) => CRITICAL_LABELS.includes(label));
}

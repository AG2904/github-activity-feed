/**
 * The single normalized shape the relay stores and serves (see GLOSSARY.md).
 * `when` is the canonical UTC ISO-8601 form, regardless of the sender.
 */
export interface ActivityEvent {
  type: string;
  actor: string;
  subject: string;
  assignee: string;
  labels: string[];
  url: string;
  when: string;
}

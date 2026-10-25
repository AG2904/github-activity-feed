# 05: (Optional) The relay runs for the team

**What to build:** The optional step of `.scratch/validation/spec.md`: put the relay
somewhere the three developers can reach, register the webhook for the confirmed
GitHub instance, and run the feed with them for a short trial. It does not gate the
phase.

**Blocked by:** 04, and by the human decision on the GitHub instance. It needs a
server, a secret and a reachable URL that a maintainer holds.

**Status:** ready-for-human

- [ ] A host or internal server runs the relay with `GITHUB_WEBHOOK_SECRET` set and a
      persistent `DATABASE_PATH`, reachable from the team's machines.
- [ ] The webhook for the confirmed instance points at the relay's root — the
      `Payload URL` with a trailing slash, not a `/webhook` path — with the same
      secret, and a real delivery arrives end to end.
- [ ] The three developers run the released client against the relay (from the README,
      not from a private note) for a short trial, and their findings are recorded.
- [ ] What was deployed, where, and how it is restarted is written down — a runbook
      fragment, not tribal knowledge.
- [ ] If it does not happen, the phase still closes: this ticket is explicitly
      optional and its absence is noted rather than hidden.

## Comments

- 2026-10-23: Kept `ready-for-human` because it needs infrastructure and credentials
  an agent does not have, and because `ARCHITECTURE.md` §7 lists deployment as
  optional. The phase's definition of done in the spec excludes it.

# 04: Everything else is ignored

**What to build:** Every webhook that is not one of the two critical events is acknowledged and discarded — nothing is persisted. This is the negative half of the filter, and it is where a filter breaks silently.

**Blocked by:** 02, 03

**Status:** done

- [x] A pull request merged into a branch other than `main` stores nothing.
- [x] A pull request that is opened, closed, reopened, or edited stores nothing.
- [x] An issue assigned without a `bug` or `feedback` label stores nothing.
- [x] An unrecognized hook type stores nothing.
- [x] A malformed payload is handled without error and stores nothing.
- [x] Each of the above is still acknowledged with a success status.
- [x] The history endpoint reports no new rows after the whole negative fixture set is posted.

## Comments

- 2026-10-16: The sweep found an ingress defect the ticket did not name. Nest's JSON parser — which runs before the controller — answered `400` for a body it could not parse, and its `strict` mode rejected even a valid JSON scalar. So a webhook the relay is supposed to acknowledge and discard came back to GitHub as a failed delivery, and the handler's own `parseBody` tolerance for unreadable input was unreachable for `Content-Type: application/json`. The ticket 02 record's claim that malformed JSON is skipped was never true on the wire.

  Andrés decided to close it inside this ticket rather than record it as a deviation, on one condition: a `2xx` must still mean the signature was verified. The relay is now created with `bodyParser: false`, registers a single raw-body parser (`src/relay/ingress.ts`), and verifies the HMAC over `req.rawBody` before parsing. A signed-but-unreadable body is acknowledged with `200` and stores nothing; a missing or wrong signature is `401` and nothing is parsed. ADR-0002 was amended with the decision and the alternative considered.

- 2026-10-16: Completed (34 e2e cases, 14 new). Three commits — `fix: verify the webhook signature before parsing the body`, `test: prove every non-critical webhook is acknowledged and discarded (ticket 04)` and `docs: add the ticket 04 record and refresh the repo tree` — made on `feat/04-ignore-everything-else` and fast-forwarded onto `main`, then pushed, with no PR (Andrés's call: personal repo, feature branching adds nothing here). Each commit passes the suite on its own, so the history stays bisectable. Status `ready-for-human` per the canonical triage labels — implementation done and landed, human review pending. The evidence record is at `docs/tickets/relay/04-ignore-everything-else.md`.

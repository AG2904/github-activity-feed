# 02: The CLI renders closed and dismissed issues

**What to build:** The client's vocabulary is the relay's contract, named once in
`internal/activity` and switched on in `internal/render`. Two new literals arrive
from the relay, and a client that only knows `pr.merged` and `issue.assigned`
would degrade them to their raw value in the headline — readable, but not the
sentence the feed promises. Name the literals, give each its human words, and
keep the session's account honest.

**Status:** done

- [x] `activity.TypeIssueClosed` and `activity.TypeIssueDismissed` name the two
      literals; the parser still accepts any `type` the relay sends.
- [x] An `issue.closed` card reads `actor closed`; an `issue.dismissed` card
      reads `actor dismissed as not planned`. In both, the labels ride with the
      title and the context line keeps the repository and the freshness.
- [x] `--json` hands over the relay's bytes unchanged for both types.
- [x] `--user` matches the actor or the assignee for both types, as it does for
      assignments.
- [x] The stop line reports the new types: `stopped · N events · N merged ·
      N assigned · N closed · N dismissed`, and `stopped · no events` stays.
- [x] The client suite renders both cards and pins the new stop line; the
      existing merge and assignment fixtures are untouched.
- [x] `npm run cli:test` and `npm run cli:lint` are green.
- [x] `docs/cli-flow.md`, `docs/manual-testing.md` and
      `docs/end-to-end-testing.md` follow the new vocabulary.

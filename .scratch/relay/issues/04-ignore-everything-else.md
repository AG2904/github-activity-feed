# 04: Everything else is ignored

**What to build:** Every webhook that is not one of the two critical events is acknowledged and discarded — nothing is persisted. This is the negative half of the filter, and it is where a filter breaks silently.

**Blocked by:** 02, 03

**Status:** ready-for-agent

- [ ] A pull request merged into a branch other than `main` stores nothing.
- [ ] A pull request that is opened, closed, reopened, or edited stores nothing.
- [ ] An issue assigned without a `bug` or `feedback` label stores nothing.
- [ ] An unrecognized hook type stores nothing.
- [ ] A malformed payload is handled without error and stores nothing.
- [ ] Each of the above is still acknowledged with a success status.
- [ ] The history endpoint reports no new rows after the whole negative fixture set is posted.

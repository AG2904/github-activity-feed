# 06: The first connection does not start blank

**What to build:** A backfill that fills the terminal on the first connection, so a developer can tell the CLI works before the next critical event happens.

**Blocked by:** 05

**Status:** ready-for-agent

- [ ] On the first connection the CLI queries the stored history, prints the most recent `--backfill` events, and then follows the stream.
- [ ] `--backfill N` defaults to 10; `--backfill 0` disables it, and the CLI prints nothing until a live event arrives.
- [ ] The backfilled events honour `--user` and `--repo` exactly as the live stream does.
- [ ] The backfill is flat and ignores `--group-by-repo`.
- [ ] A reconnection never re-runs the backfill.
- [ ] An empty history is not an error: the CLI prints nothing and follows the stream.

## Comments

- 2026-10-17: The relay replays nothing on connect, so without this the terminal stays empty until the next merge into `main` — possibly days — and the developer cannot tell a working CLI from a broken one. Running it on the first connection only is the point: a backfill on every reconnect would reprint history and undo the dedupe the relay went to the trouble of guaranteeing.

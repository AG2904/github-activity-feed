# 01: The watch can ring

**What to build:** a developer running `feed watch` can ask for a signal outside
the feed and get one. The `watch` command gains a `--notify` flag that takes
`off`, `bell`, `osc` and `auto`; `off` is the default, and this ticket makes
`bell` real while `osc` and `auto` are accepted and behave as `off` until their
own ticket lands. One bell is written per event the feed shows, and nothing at all
is written for the backfill, for `feed history`, or for an event the local filter
discards. The signal goes to the console the process is attached to, never to
stdout or stderr, and with no console attached the whole thing is a no-op. No
process is spawned and no dependency is added.

**Blocked by:** None (can start immediately)

**Status:** ready-for-human

Verificación manual pendiente, con los comandos y el entorno ya relevado:
[`../manual-verification.md`](../manual-verification.md).

- [ ] `feed watch --notify=bell` rings once per shown live event; `--notify=off`
      and the no-flag default ring for none.
- [x] An unknown `--notify` value exits `2` and prints the usage line naming the
      four accepted values; `feed watch --help` shows the same four.
- [x] Nothing new reaches stdout or stderr in any mode: a redirected watch is
      byte-identical to the base commit for the same scripted relay, and `--json`
      output is unchanged.
- [ ] Backfill and `feed history` never ring, even with a mode enabled.
- [ ] A filtered event is not signalled, because the rule is "shown", not
      "received"; the by-hand case is recorded in the manual testing document.
- [x] With no console attached (the suite's pipes, a service, a redirected
      shell), the mode is a no-op and every exit code is unchanged.
- [x] A signal that cannot be delivered never surfaces as an error, is not
      retried, and does not change the `stopped · …` session account.
- [x] The client still exits `0` on `Ctrl-C` with a mode enabled.
- [x] The client's flow document describes the sink as part of the output policy
      and states explicitly that it is not output, with the "shown, not received"
      rule; the manual testing document gains the by-hand bell check.
- [x] The shipped client still imports no process-spawning package.

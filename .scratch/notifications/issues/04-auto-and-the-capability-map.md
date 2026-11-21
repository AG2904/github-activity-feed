# 04: Auto and the capability map

**What to build:** a developer who does not know their terminal sets one flag and
gets the best signal that terminal can produce. `auto` picks the frame when the
environment identifies a terminal known to understand it, and the bell otherwise,
using only what the terminal already publishes — no probe, no query, no bytes
written to find out. The same ticket publishes the map a reader needs to verify
the feature by hand: which terminals act on the sequences and from which version,
which ones fall back to the bell, how to make the bell visible in Windows
Terminal, what tmux needs and why the client does not set it, and the explicit
statement that none of those terminal settings are touched by the client.

**Blocked by:** 03 (the frame)

**Status:** ready-for-human

El efecto visual de `auto` se verifica junto con el frame del ticket 03, en
`docs/cli-testing.md` (paso 13): la elección ya está fijada por
`client/internal/notify/notify_test.go`.

- [x] `--notify=auto` frames on a terminal the environment identifies as
      supporting notifications, and rings the bell otherwise; the selection is a
      pure function of an injected lookup, so no test depends on the machine it
      runs on.
- [x] `auto` never probes the terminal and writes nothing to discover support.
- [x] The manual testing document carries the per-terminal table, the Windows
      Terminal enablement step, the bell-style note, and the tmux paragraph.
- [x] The flag and its four modes are listed where the watch is documented, and
      ADR-0017 is linked from the point where the decision is described.
- [x] The client's flow document gains nothing it does not need: the capability
      table belongs to the manual testing document, not to the map.

# Desktop notifications for the feed

Status: ready-for-human

## Problem Statement

A developer watching the feed has to keep the terminal visible to notice that an
event arrived. The relay already knows a merge or a critical assignment happened,
and the client already prints it — but the information only exists inside one
window. When the developer switches to another application, opens a PR review, or
walks away while a build runs, an event lands in a terminal nobody is looking at,
and the watch only "works" for whoever happens to be staring at it. The
complaint is the one every terminal tool has: the feed is loud only when you are
already paying attention.

## Solution

`feed watch` can raise a desktop notification for each event it shows. It is a
flag — `--notify` — that is off unless asked for, and it produces the notification
through the terminal the feed is already running in, not through an operating
system call. The event's card still lands on stdout exactly as it does today; the
notification is an additional ping outside the window, courtesy of the emulator
that turns an escape sequence into a system notification.

Four modes cover the machines people actually use: `off` (the default), `bell`
for a terminal nobody has taught to notify, `osc` for the escape sequence, and
`auto` to pick between those from what the terminal advertises. A terminal that
does not understand the sequence drops it silently, so the failure mode is "no
notification", never "garbage in the feed".

## User Stories

1. As a developer watching the feed, I want a desktop notification when an event
   is shown, so that I notice a merge without staring at the terminal.
2. As a developer who switches to a browser to review a PR, I want the merge
   notification to arrive outside the terminal, so that the feed does not require
   the terminal to be the focused window.
3. As a developer running a long build in another pane, I want an event to reach
   me while I am not looking at that pane, so that I do not discover it an hour
   later.
4. As a developer on a terminal that supports desktop notifications, I want the
   notification to look like a normal application notification, so that it lands
   in the same place as my mail and my calendar reminders.
5. As a developer whose terminal does not support the sequence, I want the same
   flag to ring the bell instead, so that I still get an audible signal.
6. As a developer who wants silence, I want notifications off by default, so that
   installing an update does not start interrupting me.
7. As a developer who wants to be interrupted, I want to turn notifications on
   with one flag, without installing a helper, a daemon, or a second binary.
8. As a developer watching a filtered feed (`--user`, `--repo`), I want to be
   notified only for the events I asked to see, so that the ping and the card
   cannot disagree.
9. As a developer starting a watch with `--backfill`, I want history to arrive
   without a burst of notifications, so that opening the feed does not ring ten
   times for events I already knew about.
10. As a developer running `feed history`, I want that command never to notify,
    so that asking a question is not treated as news arriving.
11. As a developer piping the feed into `jq` or `tee`, I want stdout to keep
    carrying events and nothing else, so that the notification never corrupts the
    stream.
12. As a developer piping the feed into a log, I want stderr to keep carrying
    plain text, so that the log I collect has no escape sequence in it.
13. As a developer using `--json`, I want notifications to be decided by the
    notification flag alone, so that one flag means one thing.
14. As a developer whose terminal never supports notifications, I want the client
    to keep watching without errors, so that an unsupported feature never becomes
    an outage.
15. As a developer running the feed in an environment with no console (CI, a
    service, a redirected shell), I want the mode to degrade to a no-op, so that
    the process does not fail for a notification it cannot deliver.
16. As a developer on Windows Terminal, I want the client to work the moment I
    enable the terminal's notification support, so that I do not need a new
    binary.
17. As a developer on a terminal that needs the bell, I want to be told in the
    documentation which setting turns the bell into a visual alert, so that the
    feature is usable without reading the emulator's source.
18. As a developer inside tmux, I want the documentation to tell me honestly that
    the sequence does not pass through by default, so that I am not left guessing
    why nothing rings.
19. As a developer on macOS or Linux, I want the same flag to work in iTerm2,
    Ghostty, WezTerm and kitty, so that the feature is not Windows-only.
20. As a developer receiving a notification for an issue with a very long title, I
    want the text truncated, so that the notification is readable and cannot
    stall the notification daemon.
21. As a developer receiving an event whose text contains a semicolon, I want the
    notification to keep the title and the body apart, so that the payload cannot
    rewrite what the notification says it is.
22. As a developer receiving an event whose text contains control or bidi
    characters, I want them neutralised, so that the notification cannot hide or
    reverse what it displays.
23. As a security reviewer, I want the notification frame to be built by one
    constructor from a fixed template, so that the escape a terminal acts on is
    always the program's own.
24. As a security reviewer, I want the client to spawn no process at runtime, so
    that a notification can never become a command string.
25. As a security reviewer, I want the default to be off, so that the feature
    adds no vector to somebody who never asked for it.
26. As a security reviewer, I want a test that pins the frame's shape — one
    introducer, one terminator, no control byte in a field — so that the class of
    injection this ADR rejected cannot come back unnoticed.
27. As a maintainer, I want the notification text and the card to derive the same
    sentence from the same event, so that the two cannot describe one event
    differently.
28. As a maintainer, I want a refusal for an unknown `--notify` value with the
    usage line, so that a typo fails loudly instead of silently doing nothing.
29. As a maintainer, I want the watch's session account (the banner, the
    `connected` line, the `stopped` line) unchanged, so that notifications do not
    complicate what the session says about itself.
30. As a maintainer, I want the capability notes for each terminal in the manual
    testing document, so that a reader can verify the feature by hand.
31. As a contributor, I want the decision written down as an ADR that names the
    rejected alternative and why, so that the next person does not reopen it as a
    "small fix".
32. As a person demoing the project, I want the notification to be opt-in per
    command, so that the recorded demo decides whether it rings.

## Implementation Decisions

- **Flag surface.** `watch` gains `--notify`, accepting `off`, `bell`, `osc` and
  `auto`; the default is `off`. An unknown value is a usage error with exit code
  `2` and the usage line, like the existing flag refusals. `history` does not
  grow the flag: it is a question, not a subscription.
- **One meaning.** The flag decides whether a shown event also rings; it does not
  interact with `--json`, `--no-color` or the filters. A filtered event never
  notifies, because the rule is "shown", not "received".
- **Mode semantics.** `bell` writes a single bell character and carries no event
  text. `osc` writes the frame for the detected emulator. `auto` selects `osc`
  when the environment identifies a terminal with a known notification protocol
  and `bell` otherwise. `off` writes nothing.
- **Detection is environment-only.** The choice reads what the terminal already
  publishes (`WT_SESSION`, `TERM_PROGRAM`, `KITTY_WINDOW_ID`, `TERM`). There is
  no query sequence and no probe: a probe writes the terminal's answer into a
  scrollback the session cannot clean up, and terminals that do not implement the
  query print the bytes literally.
- **Protocol per terminal.** `OSC 9` where only a message is understood, `OSC 777`
  where a title and a body are wanted, `OSC 99` for kitty. The frame's
  introducer, separators and terminator are literals owned by the program.
- **A new `notify` package in the client** owns the whole decision: the mode
  parsing, the environment detection, the protocol choice, and the frame
  encoder. Its encoder is a pure function of the event — it takes an event and
  returns bytes — so the property that matters is testable without a terminal.
- **The sink is the console device.** The notifier writes to the console the
  process is attached to, not to stdout or stderr, and it is opened once at the
  process edge. When no console exists the notifier is a no-op. A notification
  failure is swallowed: it is never reported, never retried and never counted.
- **Where it is called.** Inside the live-stream loop, only after a successful
  card write, and only for a payload that arrived on the connection. The backfill
  and the history path share the card writer but never notify.
- **Sanitisation is part of the frame, not of the caller.** Every variable field
  passes through the renderer's existing control-byte cleaner before it reaches
  the frame; the frame additionally turns each `;` into a space, drops bidi
  override characters, and clamps the title to 64 runes and the body to 200. The
  URL never travels: no hyperlink and no markup. The prefix that identifies the
  origin is fixed text, not event text.
- **One sentence, two renderings.** The rendered card and the notification must
  derive the same sentence from the same event. The renderer exposes the plain
  sentence, the palette keeps composing its coloured form, and a test pins that
  the coloured form with its escapes removed equals the plain one. Unknown event
  types keep degrading to their raw value in both.
- **No process, ever.** The shipped client spawns no process; `os/exec` stays
  confined to the test harness that builds and drives the binary. This is a
  property of the feature, asserted in review, not a preference.
- **No dependency.** The client's module continues to carry no `require`.
- **Docs follow the code.** The client's flow document gains the notification as
  part of the session's output policy — explicitly not output — with the sink and
  the "shown, not received" rule; the manual testing document gains the per-
  terminal capability table and the tmux note; the ADR that decided this is
  referenced from both.

## Testing Decisions

A good test here asserts what a user or a reviewer can observe: the flag's
surface and refusals, the bytes the process puts on its two documented streams,
and the exact shape of the frame that leaves the program. It does not assert
which function was called, which file opened the console, or how the protocol was
chosen internally.

Two seams, one of them new:

- **The process boundary (existing, preferred).** The client's suite compiles the
  binary and speaks to it across the process boundary. It covers the flag's
  parsing and refusals, the exit codes, `--notify=off` producing byte-identical
  output to the base commit, and the invariant that enabling a mode adds no byte
  to stdout or to stderr while the feed is redirected. Prior art: the end-to-end
  cases that drive the binary against a fake relay, and the security cases that
  pin the two streams' contracts.
- **The encoder (new, and the only new one).** A byte written to a console cannot
  be observed from a pipe, so the frame's shape is pinned by a unit test beside
  the new package: exactly one introducer, exactly one terminator, no control
  byte inside a field, a `;` that becomes a space, a clamped title and body, bidi
  stripped, no URL, and a plain sentence equal to the coloured card with escapes
  removed. The environment detection is tested the same way, as a pure function
  over an injected lookup, so no test depends on the machine it runs on. It is
  the client's first unit test that pins a byte no pipe can observe, and the ADR
  records the reason.

Not tested, deliberately: the console write itself, which is a single call whose
failure is ignored by design, and the notification daemon's behaviour, which is
the terminal's and the desktop's to get right.

## Out of Scope

- **Click to open.** A notification that opens the pull request needs an OS-level
  notification with an activation handler and a registered identity; the escape
  path has no click target and this spec does not invent one.
- **An unread badge or an inbox.** No tab title changes, no unread counters, no
  persistence across processes. `feed history` remains the place to read what
  happened.
- **Any OS notifier.** No PowerShell, no `osascript`, no `notify-send`, no
  third-party notification library, no AppUserModelID, no Start Menu shortcut.
- **tmux passthrough.** The client does not wrap the frame in a multiplexer
  escape, because that requires the user to allow passthrough for every pane. The
  limitation is documented instead.
- **Probing the terminal** for notification support.
- **Notifications for history or backfill**, or for any command other than
  `watch`.
- **Sound selection, urgency levels, icons and actions.** The terminal owns the
  presentation.
- **Changes to the relay, the wire contract, or the stored history.**

## Further Notes

The decision and its rejected alternative live in ADR-0017
(`docs/adr/0017-notifications-are-an-escape-sequence-not-a-process.md`), which
carries the security argument: the shell-out path is a command string built from
event text, and 2026 alone produced a local-root escalation in a notification
hook, an arbitrary-code-execution report in a notification plugin, and a
high-severity clipboard advisory in a terminal that trusted its output. The
escape path is safer for a structural reason rather than a stylistic one — the
control byte that would start a second sequence is already filtered out before
the frame is built.

The support matrix a reader needs, and which the manual testing document should
carry: Windows Terminal understands `OSC 777` from v1.26 with
`compatibility.allowOSC777` enabled, and its older answer is the bell styled as a
taskbar flash; Ghostty, iTerm2, WezTerm and kitty act on the sequences without
configuration, with their own notification switches; GNOME Terminal, Alacritty
and macOS Terminal.app fall back to the bell. None of those settings are touched
by the client.

The work splits naturally into three slices: the `notify` package with its frame
and detection (both testable without a console), the wiring into `watch` with the
flag and the two stream invariants, and the documentation plus the capability
table. Slices one and two are the ones a reviewer should read together, since the
seam is only meaningful with the call site.

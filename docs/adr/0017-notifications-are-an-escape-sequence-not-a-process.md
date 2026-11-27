# A notification is an escape sequence, and the client never runs a program

`feed watch` gains an opt-in notification: when an event is shown on the live
stream, the client writes one terminal escape sequence to the console device and
the emulator turns it into a desktop notification. The flag is `--notify`, it
takes `off` (the default), `bell`, `osc` and `auto`, and it decides one thing —
whether a shown event also rings something outside the feed. It never changes
what stdout carries, which is still events and nothing else.

The mechanism is OSC, not an OS call. `osc` writes `OSC 9` to iTerm2, Ghostty and
WezTerm, `OSC 777;notify` where a title and a body are wanted, and `OSC 99` to
kitty; a terminal that does not know the sequence drops the whole frame in
silence, which is the same reason the sequence is safe to emit without probing.
`auto` chooses between those and `bell` from the environment the terminal leaves
behind (`WT_SESSION`, `TERM_PROGRAM`, `KITTY_WINDOW_ID`, `TERM`), because a probe
would ask the terminal a question and write its answer into a scrollback the
session cannot clean up. `bell` carries no text at all: it is the mode for a
terminal nobody has taught to notify, and it is the mode that cannot be injected
by construction.

## The frame is a fixed template over cleaned fields

The provider of every field in an event is the relay, which stored whatever
GitHub sent. That text reaches a terminal twice today — the card and the `OSC 8`
hyperlink — and `render.Clean` is what keeps both honest: it drops C0, DEL and
C1, so it drops `ESC` (0x1b) and `BEL` (0x07), the two bytes a second sequence
would need. The notification frame inherits that property only if it is built the
same way, so the contract is:

1. one constructor, owned by a small `internal/notify` package, that takes an
   `activity.Event` and returns the bytes — never a function that takes strings;
2. a fixed template — the introducer, the terminator and the separator are the
   program's own literals, and the only variable parts are fields that came out
   of `render.Clean`;
3. `;` becomes a space in each field, because `OSC 777` uses it as the separator
   between title and body and a subject is free to contain one;
4. each field is clamped (title to 64 runes, body to 200), because a notification
   daemon is not a scrollback and a 4 KB subject is a denial of service against
   it;
5. bidi overrides (U+202A–202E, U+2066–2069) are dropped, because `render.Clean`
   predates them and they reverse what a toast shows without changing a visible
   byte;
6. the URL never travels: no hyperlink, no markup. A ping says what happened, and
   the feed remains the place where a link is clickable.

The property that follows is testable as a sentence: the output contains exactly
one `ESC`, exactly one terminator, and no control byte inside the fields. It is
the first assertion that pins the class, and the reason this ADR prefers OSC over
any mechanism whose safety is a matter of remembering to escape.

## Why not the operating system's own notification

The obvious alternative is to shell out: `powershell.exe` and WinRT toasts on
Windows, `osascript` on macOS, `notify-send` on Linux. It produces the toast a
person expects in the Action Center, and it is the option this ADR rejects, on
evidence rather than taste.

Every one of those paths is a string built by the program and parsed by something
else, and the class has a body count this year alone. needrestart's
`400-notify-send` hook interpolated an attacker-chosen process name into a
`su -c` line, which made the next `apt` run execute that name as root — local
privilege escalation, CVE-2026-124757, disclosed on 17 November 2026 and fixed by
passing the title and the body through environment variables instead. A
notification plugin for an agent runner built `osascript -e 'display
notification "…"'` from the folder name and escaped only double quotes, so a
single quote in a directory name executed code on the first completion
(opencode-notifier#85, June 2026; the fix is `execFile` with an `on run argv`
handler and no shell at all). `notify-send` bodies accept markup per the
freedesktop specification, `<img src="…">` included, so unescaped event text is
a remote beacon at best. And the terminal side of the same coin is not
hypothetical either: Warp shipped OSC 52 clipboard access from terminal output
with no confirmation step — CVE-2026-48725, CVSS 8.1 — and closed it by denying
by default.

The asymmetry is what decides. An escape frame built from literals plus `clean`
output cannot execute anything; its worst case is a spoofed toast, an inverted
line, or an emulator that misbehaves. A shell string built from payload fields
has arbitrary code execution as its worst case, and its safety depends on a
discipline that has already failed in three projects above. `client/` ships no
`os/exec` today; the only imports of it live in `client/test`, where they build
and drive the binary. After this ADR that stays true, and it is written down as a
property rather than a coincidence: the client's runtime spawns no process.

## Considered Options

- **Shell out to the platform's notifier** — rejected. It buys a real toast with
  an identity of its own and click-to-focus, and it pays with a process spawn per
  event, a command string built from attacker-influenced text, a notification
  attributed to `Windows PowerShell` unless a shortcut with an AppUserModelID is
  installed, and a TCC prompt on macOS. The failures above are the reason the
  price is not worth it here.
- **`gen2brain/beeep` or another notification library** — rejected. It is the
  same shell-out with less control and a supply chain attached, and it would put a
  `require` in a `go.mod` that has none
  ([ADR-0008](0008-stdlib-only-and-flag-over-cobra.md)). The escape path needs no
  library at all.
- **Notify from `emitter.paint`** — rejected. `paint` is shared by the live
  stream and the backfill, so a watch started with the default `--backfill 10`
  would fire ten notifications for history the reader is looking at.
- **Notify through stdout or stderr** — rejected. stdout is the events contract —
  `feed watch | jq` must keep working — and stderr is documented as plain text
  without escapes. The sink is the console device itself, `CONOUT$` on Windows and
  `/dev/tty` elsewhere, opened by the process edge; when there is no console the
  mode is a no-op.
- **Default the flag on** — rejected. On is a vector by omission for a program
  whose input is a repository's own text, and it also writes event text into the
  notification center, where it outlives the terminal and can reach a lock
  screen. `off` is the default and the user's first notification is a decision
  they made.
- **Wrap tmux passthrough by default** — rejected. `tmux` drops these sequences
  unless `allow-passthrough all` is set, and `all` is the multiplexer's filtering
  turned off for every pane, not just this one. The documented answer is the
  sequence, not a weakened terminal setting; a user who wants it inside tmux sets
  it themselves.
- **Bell only** — rejected as the only mode, kept as one. It is the one mechanism
  that carries zero bytes of event text, so it is strictly the safest, but it also
  says nothing: a bell rings the same for a merge and a dismissal. `--notify=bell`
  exists, `auto` falls back to it, and `off` remains the default.

## Consequences

The seam is `stream()`, after a successful `paint` and only for a payload that
arrived on the live connection: history and backfill never ring. The notifier is
built once in `watch()` with the rest of the fixed-per-process decisions, and a
write failure is swallowed — a notification that cannot be delivered must never
take down a watch. `--json` does not silence it, because the flag that decides
notifications is `--notify`; one flag, one meaning.

Two terminal-side facts are the user's to own, and the documentation says so
rather than reaching for them: Windows Terminal only understands `OSC 777` from
v1.26 and only with `profiles.defaults."compatibility.allowOSC777": true`, and
Ghostty and the other emulators expose their own notification switch. None of
them are touched by the client. The capability table, the per-terminal caveats
and the tmux note belong in `docs/cli-testing.md`, where a reader already goes to
run the thing by hand.

The frame's encoder is a pure function, and pinning its property — one `ESC`, one
terminator, no control byte in a field — needs a unit test inside `client/`
rather than one across the process boundary: a byte that goes to a console cannot
be observed from a pipe, so `internal/notify/frame_test.go` pins the frame's
shape beside the package that builds it. The e2e suite keeps its half: `--notify`
parsing and refusals, and the invariant that enabling notifications adds no byte
to stdout or stderr.

What this does not do is worth being explicit about, because it is what the
other option was for: no click opens the pull request, no unread badge survives
in the tab, and no inbox is created. The ping tells a person that something
happened; `feed history` is where they read what it was.

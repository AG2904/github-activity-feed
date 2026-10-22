# CLI output — the feed reads like a feed

Status: ready-for-agent

Supersedes the **Rendering** decision of [`.scratch/cli/spec.md`](../cli/spec.md), and
user stories 4 to 7 with it. Everything else in that spec stands.

## Problem Statement

The CLI answers a question email does not: it puts the event where the work already is,
without a tab change and without an inbox. But its output is a log line, not a feed:

```text
[pr.merged] Fix memory leak in parser component
AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/…/pull/42
```

Three things are wrong with it, and all three are the same thing — it speaks in the
schema's vocabulary instead of the reader's:

- `[pr.merged]` is a wire value. Nobody thinks "pull request dot merged"; they think
  "María integró esto en main".
- The line carries a badge, a subject, a repository, an actor, a time and a URL with
  the same weight, so nothing leads and nothing follows.
- The URL is repeated as text even when the terminal can make the subject clickable,
  and the labels — the very reason an assignment interrupted you — are parsed and
  thrown away.

The result is the worst of both worlds: the interruptiveness of a notification with the
legibility of a log. If the output is not better than the email it replaces, the CLI has
no reason to exist beyond novelty.

## Solution

One event becomes a **card of three lines** — a sentence, the thing (prefixed by the
reason), the context — with a blank line between cards:

```text
maria.perez merged fix/memory-leak into main
  [bug] Fix memory leak in parser component
  AG2904/github-activity-feed · 2h ago

lucia.gomez assigned to tomas.ferreira
  [bug, feedback] Ship the onboarding checklist
  AG2904/github-activity-feed · 2h ago
```

When the terminal cannot hyperlink (a pipe, `--no-color`), a fourth indented line
carries the URL, because a subject with no link is a dead end:

```text
maria.perez merged fix/memory-leak into main
  [bug] Fix memory leak in parser component
  AG2904/github-activity-feed · 2h ago
  https://github.com/AG2904/github-activity-feed/pull/42
```

The card is not the whole screen. `watch` also says what it is doing and what
happened, on **stderr**, in plain words — the session chrome:

```text
watching http://localhost:3000/stream · backfill 10     (stderr, on start)
connected · 3 events from history                       (stderr, after the first connect)
stopped · 12 events · 7 merged · 5 assigned             (stderr, on exit)
```

Colour carries **roles**, never decoration, and never the only signal: the words say
what happened whether or not the terminal shows colour.

## User Stories

1. As a developer, I want the event told as a sentence — who did what, and where it
   landed — so that I understand it without decoding a badge.
2. As a developer, I want the title on its own line, so that the thing that happened has
   room to be read instead of competing with metadata.
3. As a developer, I want the label that made an event critical to read immediately
   before the thing it qualifies, so that the reason arrives before the content.
4. As a developer, I want one event to look like one card, so that a burst reads as a
   list and not as a wall.
5. As a developer, I want colour to separate people, states, places and reasons, so that
   I can scan a feed without reading every word.
6. As a developer, I want the same output to stay readable with no colour at all, so
   that a pipe, a CI log or a colour-blind reader loses nothing.
7. As a developer, I want relative times with human boundaries — `yesterday`, not
   `1d ago` — so that the feed sounds like someone wrote it.
8. As a developer, I want the live feed, the backfill and the history report to use the
   same card, so that an event looks the same wherever I meet it.
9. As a developer, I want `watch` to say what it is watching and whether it is connected,
   so that a silent terminal reads as "nothing happened" and not as "it hung".
10. As a developer, I want a one-line account when I stop the watch, so that the session
    tells me what it saw before it goes away.
11. As a developer, I want all of that on stderr, so that redirecting stdout to a file or
    a pipe leaves the machine-readable feed untouched.

## Implementation Decisions

- **Scope: the human output only.** `--json` is the contract: the CLI still hands over
  the relay's bytes verbatim and reshapes nothing, so the filters, the order, the flags
  and the wire format stay untouched. The bytes themselves did change shape, because
  the relay's event gained `branch` (ticket 05, ADR-0013); see the dated amendment below.
- **The card.** Three lines, fixed indentation of two spaces on lines two and three:
  1. `actor verb [destination]`
  2. `[labels] title` — the labels the event carries, in brackets and separated from
     the title by one space, hyperlinked when colour is on; a card with no labels
     starts with the title.
  3. `repository · time` — each segment omitted when the event does not carry it (a
     legacy row has no repository).
  A fourth indented line carries the URL only when hyperlinks are unavailable.
- **Vocabulary.** `merged` and `assigned` are the human words; `pr.merged` and
  `issue.assigned` stay on the wire and in `--json`. An unknown type falls back to its
  raw value, so a new event type degrades to legible rather than to blank.
- **The destination names the branch when the event carries one.** A merge's source
  branch (`branch`, from the relay's `head.ref`, ADR-0013) renders `actor merged
  <branch> into main`. When `branch` is null — a row stored before the field existed,
  or a payload with no usable head ref — the phrase falls back to `actor merged into
  main`, because `pr.merged` still *means* a merge into `main`; a future type with a
  different destination gets its own phrase rather than reusing this one. An
  assignment with a null assignee renders `assigned` with no `to`.
- **Labels ride with the title.** They are the reason the event exists, and the CLI
  already receives them: `[bug]`, `[bug, feedback]`, immediately before the thing they
  qualify — the load-bearing word first, the way cargo puts the qualifier next to the
  object (`Compiling serde v1.0.203`). The old decision to parse and ignore them was
  already reversed; this amendment moves them out of the context line because a reason
  the reader reaches last is a reason they scan past. An assignment without labels is
  unchanged, and the bold-red-on-bold pair survives conhost: the reset between the two
  segments keeps them distinct.
- **Time keeps four human boundaries**: `just now` under a minute, `Nm ago` under an
  hour, `Nh ago` under a day, **`yesterday` from 24 to 48 hours**, `Nd ago` beyond
  that. The 24–48 window is an approximation of a calendar day and is documented as
  one rather than computed against the local midnight.
- **Separation belongs to the emitter, not the renderer.** The renderer paints one
  card; the emitter writes the blank line *before* every card except the first. That
  keeps the last event from leaving a gap under the prompt, and keeps the renderer
  stateless for the feed. The grouped report is the documented exception: it renders a
  whole document — header included — in one call, so `Palette.Grouped` owns its own
  spacing and the emitter has nothing to interleave there. `--json` gets no blank lines.
- **The session chrome is stderr, and it is plain.** `watch` narrates its own life in
  three lines, all on stderr, all uncoloured:
  1. on start, the banner already printed today, extended: `watching <url> · backfill
     <n>`, with the `--user` / `--repo` filters appended when set (`watching <url> ·
     backfill 10 · user maria · repo AG2904/x`);
  2. after the first connection is accepted: `connected · <n> events from history` when
     the backfill printed some, `connected · waiting for events` when it printed none;
     a later reconnect keeps the existing `watch: reconnected` notice, so only the
     first connection gets the affirmative line;
  3. on exit — Ctrl-C included, whenever `watch` returns — `stopped · <n> events ·
     <k> merged · <l> assigned` when any event was shown, `stopped · no events`
     otherwise. The counts are of *shown* events, filter included, because that is
     what the reader saw.
  The chrome is not TTY-gated, unlike gh's epilogue: it is the connection banner's
  family — today's `watching <url>` is already unconditional — and every line is one
  sentence of state a redirected log benefits from just as much as a terminal does.
  What it must never do is reach stdout or carry an escape: it is messaging, which is
  stderr's job, and it is plain text by construction, so the escape discipline below
  still describes stdout and nothing about the chrome can break it.
- **No heartbeat, and no spinner.** A feed that can stay silent for hours is exactly
  where a periodic "still alive" line tempts you, and it is refused here: gh redraws it
  on an alternate screen where repetition is invisible, while on our append-only
  terminal every repeat is permanent ink and the line that said "alive" is the line
  that starts saying "spam". Silence is already answered by the reconnect notices — a
  dead relay speaks, a healthy quiet one does not — and by the time on the last card.
  The first-connection line answers the only moment "is it alive?" has no other
  reply: the one right after start.
- **Colour by role, 16-colour ANSI**, applied to `render.Palette`:

  | Element | Colour | Why |
  | --- | --- | --- |
  | actor | bold cyan | the person who acted |
  | `merged` | bold green | integration: something reached `main` |
  | `assigned` | bold amber | attention: work landed on someone |
  | `main` | bold green | the destination of the same fact |
  | assignee | bold cyan | the other person in the sentence |
  | `into` / `to` | grey | the connector is grammar, not information |
  | title | bold, default, OSC 8 | the content; it does not compete |
  | repository | blue | a location, the way a path is shown |
  | branch | blue | a location, the way a path is shown |
  | labels | bold red | the reason it interrupted you |
  | `·` and the time | grey | context |

  Sixteen colours, not 256: the binary is distributed to terminals that are not Windows
  Terminal, and every one of these codes works wherever VT works. "Grey" is bright black
  (`90`), not faint (`2`): conhost with VT ignores the faint attribute, so the context
  would lose its de-emphasis exactly on the console this palette is bounded by. No emoji,
  no box drawing, no reverse video: the console is a portability boundary.
- **Colour is never the only signal.** `merged` and `assigned` are different words, the
  repository contains a slash, labels sit in brackets. A reader who loses colour loses
  reinforcement and nothing else.
- **Escape discipline is unchanged**: colour and hyperlinks only when stdout is a
  terminal and neither `--no-color` nor `NO_COLOR` is in effect. A pipe gets the card
  with the URL line and not one `\x1b`.
- **Grouped report.** The repository heading stays, in bold blue, and the cards under it
  drop the repository segment from their context line; a blank line separates the
  heading from the first card and each card from the next.
- **No wrapping.** A title longer than the terminal wraps at the edge, as it does today.
  Manual wrapping needs a width the CLI does not know when stdout is a pipe, and it is
  out of scope for this feature.

## Testing Decisions

- **The seam does not change**: the process boundary. The suite compiles the binary and
  asserts exact stdout for one card of each type, with colour off (a pipe), so the
  fixtures are the contract.
- **Cases that must be pinned**:
  - a merged event renders the three lines, in that order, with the labels before the
    title, and names its branch when the event carries one;
  - a merged event with `branch: null` renders `actor merged into main`, with no empty
    segment and no doubled `into main`;
  - an assigned event renders `actor assigned to assignee`, and `assigned` alone when
    the assignee is null;
  - an event with no labels starts line two with the title instead of printing an
    empty `[]`;
  - a legacy event with `repository: null` renders a context line of just the time;
  - `yesterday` appears at 30 hours and `2d ago` at 50;
  - two consecutive events are separated by exactly one blank line, and the first card
    is not preceded by one;
  - the same card appears in `watch` (live), in the backfill, and in `history`, with the
    grouped report omitting the repository;
  - without colour the URL is the fourth line; `--json` is byte-identical to the relay;
  - the start banner names the backfill count and any filters; after the first
    connection stderr carries `connected · <n> events from history`, or `connected ·
    waiting for events` when the history was empty; a reconnect carries
    `watch: reconnected` and not a second `connected` line;
  - stopping the watch prints `stopped · <n> events · <k> merged · <l> assigned`, with
    filter-discarded events not counted, and `stopped · no events` when nothing was
    shown.
- **The chrome is asserted on stderr only.** Every chrome assertion reads the stderr
  channel; stdout keeps the exact card fixtures, so a chrome line that leaked into the
  feed would fail the stdout comparison even before the stderr one.
- **What is not automatable here**: colour itself. A pipe has no TTY, so the suite can
  only assert the *absence* of escapes; the presence and the hues are verified by hand
  in `docs/cli-testing.md`, in Windows Terminal and in a conhost with VT.
- **Fixtures and docs move with the code**: the pinned strings in `client/test/e2e_test.go`,
  the expected outputs in `docs/cli-testing.md`, the render description in
  `docs/cli-flow.md`, and any sample output in `docs/end-to-end-testing.md`.

## Out of Scope

- Wrapping, truncation, paging, or a maximum title length.
- 256-colour, truecolour, themes, or a configurable palette.
- Filtering by label or by event type, and grouping the live feed.
- A TUI, desktop notifications, sound, or a daemon.
- Any change to `--json` beyond carrying the relay's bytes verbatim: the CLI does not
  reshape it. The wire format and the relay change only through a documented
  amendment such as ADR-0013, which adds `branch` to the event.
- Colouring the stderr banner, the session chrome, or the reconnect notices: all of it
  is plain text by decision, so the escape discipline above has exactly one stream to
  describe.

## Further Notes

- **Why the card is worth its height.** An email competes for attention; a terminal
  feed shares it with the work. Only critical events reach this channel, so three lines
  per event is affordable in a way it would not be for a log stream. The vertical cost
  is the price of being readable at a glance.
- **What was rejected, and why.** Emoji and rail glyphs (`▌`, `●`) fail on consoles that
  are not UTF-8; reverse-video label chips read as selection in some terminals; a
  one-line format was the previous design and is the complaint that opened this
  feature; a full sentence with the title quoted inline was rejected because quotes
  force a typographic decision the console cannot guarantee. A **table** — `gh run
  list`'s columns, one row per event — was re-checked against the evidence in
  [`.scratch/cli-output/research/other-cli-output-designs.md`](research/other-cli-output-designs.md)
  and stays rejected: gh pads its columns because it fetches the whole batch before
  printing, while a live feed emits events one at a time and cannot re-pad rows already
  on screen; without truncation — which is out of scope — the rows only fit short
  titles.
- **Why the labels moved up, and not the time.** The label is the only segment that
  answers *why*; the time answers *when*, and the eye that is deciding whether to
  interrupt itself has already read the sentence by the time the context line ends.
  Putting the reason next to the thing it qualifies is the same move cargo makes with
  `Compiling serde v1.0.203` and compose makes with its source prefix: the qualifier
  rides with the content, not with the footer.
- **The colour palette is a claim about roles, not about taste.** If a new element needs
  a hue, it needs a role first; otherwise it inherits `grey`.
- **The renderer stays stateless.** Separation, backfill and grouping are the emitter's
  job; the renderer answers "how does one event look", which is what makes the four
  call sites agree.

## Comments

- 2026-10-21: Two design rounds with Andrés. The first proposal kept the two-line shape
  with aligned columns and was rejected as *escueto*: the card was the answer, because
  an event can tell a story instead of filling a row. The second round added colour at
  his request, mapped by role rather than by element, and capped at 16 colours so the
  distributed binary is not prettier only on this machine.
- 2026-10-21: The `into main` phrase was challenged in review of this spec and kept:
  the event carries no branch, and the type already means "merge to main", so deriving
  the phrase from the type is the only honest source. The note above records the
  condition under which it stops being safe.
- 2026-10-22: **Amendment — the card gains the reason, the session gains a voice.**
  After research across `gh`, `git`, `cargo`, `bun`, `npm` and `docker compose`
  ([the note](research/other-cli-output-designs.md)), two changes. First, the labels
  move from the context line to the front of the title: the *why* arrives before the
  *what*, which strengthens user story 3 instead of weakening it. Second, `watch` gets
  session chrome — a banner that names its filters, one line after the first
  connection, one line on stop — all on stderr, all plain, none of it TTY-gated,
  because the connection banner already prints unconditionally and a redirected log
  deserves the same lifecycle a terminal does. The chrome is deliberately not a
  heartbeat: repetition on an append-only terminal is permanent ink, and the reconnect
  notices already answer "did it die?". The table shape was checked against the
  evidence a second time and rejected for a reason now written down instead of
  remembered.
- 2026-10-22: **Amendment — the merge names its branch.** `pr.merged` now carries the
  source branch (`branch`, read from `head.ref` and nullable for the same SQLite
  reason as `repository`; ADR-0013), and the card reads `actor merged <branch> into
  main`. The "the event carries no branch" note under Implementation Decisions is
  retired, the palette gains the branch in the repository's location role, and the
  Out of Scope clause no longer forbids the wire change: `--json` still hands over the
  relay's bytes verbatim, but those bytes now include `branch`, and a legacy row or a
  payload without a usable head ref carries `null`. The card falls back to `merged
  into main` there, because `pr.merged` still means a merge into main.
- 2026-10-22: The session-chrome bullet's banner example was corrected to `watching
  <url> · backfill 10 · user maria · repo AG2904/x`: the example had the segments in an
  order that contradicted the prose beside it and ticket 03's checklist (backfill
  first, then user, then repo), and the prose is what the code follows.

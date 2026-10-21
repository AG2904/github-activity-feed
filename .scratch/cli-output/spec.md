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

One event becomes a **card of three lines** — a sentence, the thing, the context — with
a blank line between cards:

```text
maria.perez merged into main
  Fix memory leak in parser component
  AG2904/github-activity-feed · [bug] · 2h ago

lucia.gomez assigned to tomas.ferreira
  Ship the onboarding checklist
  AG2904/github-activity-feed · [bug] · 2h ago
```

When the terminal cannot hyperlink (a pipe, `--no-color`), a fourth indented line
carries the URL, because a subject with no link is a dead end:

```text
maria.perez merged into main
  Fix memory leak in parser component
  AG2904/github-activity-feed · [bug] · 2h ago
  https://github.com/AG2904/github-activity-feed/pull/42
```

Colour carries **roles**, never decoration, and never the only signal: the words say
what happened whether or not the terminal shows colour.

## User Stories

1. As a developer, I want the event told as a sentence — who did what, and where it
   landed — so that I understand it without decoding a badge.
2. As a developer, I want the title on its own line, so that the thing that happened has
   room to be read instead of competing with metadata.
3. As a developer, I want to see the label that made an assignment critical, so that I
   know why it interrupted me.
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

## Implementation Decisions

- **Scope: the human output only.** `--json` is the contract and does not change by a
  byte; the filters, the order, the flags and the wire format are untouched.
- **The card.** Three lines, fixed indentation of two spaces on lines two and three:
  1. `actor verb [destination]`
  2. the title, hyperlinked when colour is on
  3. `repository · [labels] · time` — each segment omitted when the event does not
     carry it (a legacy row has no repository; a merge with no labels has no label
     segment).
  A fourth indented line carries the URL only when hyperlinks are unavailable.
- **Vocabulary.** `merged` and `assigned` are the human words; `pr.merged` and
  `issue.assigned` stay on the wire and in `--json`. An unknown type falls back to its
  raw value, so a new event type degrades to legible rather than to blank.
- **The destination is derived from the type.** The event carries no branch and no
  assignee phrase, so `into main` is safe only because `pr.merged` *means* a merge into
  `main`; a future type with a different destination gets its own phrase rather than
  reusing this one. An assignment with a null assignee renders `assigned` with no `to`.
- **Labels are shown.** They are the reason the event exists, and the CLI already
  receives them: `[bug]`, `[bug, feedback]`, in the context line. The old decision to
  parse and ignore them is reversed here.
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
  - a merged event renders the three lines, in that order, with the label segment;
  - an assigned event renders `actor assigned to assignee`, and `assigned` alone when
    the assignee is null;
  - an event with no labels omits the segment instead of printing an empty `[]`;
  - a legacy event with `repository: null` starts the context line at the label;
  - `yesterday` appears at 30 hours and `2d ago` at 50;
  - two consecutive events are separated by exactly one blank line, and the first card
    is not preceded by one;
  - the same card appears in `watch` (live), in the backfill, and in `history`, with the
    grouped report omitting the repository;
  - without colour the URL is the fourth line; `--json` is byte-identical to the relay.
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
- Any change to `--json`, to the wire format, or to the relay.
- Colouring the stderr banner or the reconnect notices.

## Further Notes

- **Why the card is worth its height.** An email competes for attention; a terminal
  feed shares it with the work. Only critical events reach this channel, so three lines
  per event is affordable in a way it would not be for a log stream. The vertical cost
  is the price of being readable at a glance.
- **What was rejected, and why.** Emoji and rail glyphs (`▌`, `●`) fail on consoles that
  are not UTF-8; reverse-video label chips read as selection in some terminals; a
  one-line format was the previous design and is the complaint that opened this
  feature; a full sentence with the title quoted inline was rejected because quotes
  force a typographic decision the console cannot guarantee.
- **The colour palette is a claim about roles, not about taste.** If a new element needs
  a hue, it needs a role first; otherwise it inherits `dim`.
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

# Other CLI output designs — research note

Research for the `feed` CLI output redesign (`.scratch/cli-output/spec.md` is the binding
contract; this note feeds it, it does not amend it).

**Method.** Every sample below is either (a) *captured locally* (git 2.55.0.windows.5,
bun 1.4.2, npm — raw bytes through a pipe), (b) *verbatim from the tool's own test
fixtures or docs*, or (c) *reconstructed from the tool's source format strings* and
labelled as such. Nothing is invented.

## 1. Question

What makes terminal output feel user-friendly for a low-volume event feed — a screen that
must answer "who did what, where, and why did it interrupt me" at a glance — given our
constraints: Go stdlib only, zero escape bytes on a pipe, `--json` untouchable, 16-colour
SGR only, and Windows Terminal *plus* legacy conhost (VT on) as the portability floor, so
no emoji, no box drawing, no reverse video, no faint (2)? We study the closest real
analogues — `gh`'s live run watcher, git's progress and colour layering, cargo's status
vocabulary, compose's per-source log prefixes, package managers' install summaries — and
the two design specs (clig.dev, cli.style), to extract devices that survive those bounds.

## 2. Findings per tool

### 2.1 `gh run watch` — the closest thing to a live event feed

Reconstructed from source ([watch.go](https://github.com/cli/cli/blob/trunk/pkg/cmd/run/watch/watch.go),
[presentation.go](https://github.com/cli/cli/blob/trunk/pkg/cmd/run/shared/presentation.go));
the job lines are **verbatim from gh's own test fixture**
([presentation_test.go](https://github.com/cli/cli/blob/trunk/pkg/cmd/run/shared/presentation_test.go)):

```text
Refreshing run status every 3 seconds. Press Ctrl+C to quit.

X fix and test · 1234567890
Triggered via push 2m30s ago

JOBS
X build in 1m0s (ID 999)
  ✓ passed
  X failed
* test (ID 999)
  * in-progress

✓ Run fix and test (1234567890) completed with 'success'
```

Devices, and the problem each solves:

- **State symbol + state colour + state word, always together.** `Symbol()` in
  [shared.go](https://github.com/cli/cli/blob/trunk/pkg/cmd/run/shared/shared.go) returns
  (`✓`, green) for success, (`X`, red) for failure, (`-`, muted) for skipped/neutral,
  (`*`, yellow) for in-progress. Note the failure glyph is ASCII `X`, not `✗` — a
  portability decision visible in the source. Colour reinforces; the glyph and word carry
  the meaning alone.
- **Two-space indent for the detail level** (steps under jobs). Detail is opt-in: steps
  print only when the job failed *or* `--verbose` ([presentation.go](https://github.com/cli/cli/blob/trunk/pkg/cmd/run/shared/presentation.go):
  `if verbose || IsFailureState(job.Conclusion)`). Signal-to-noise: success collapses to one line.
- **A "still alive" line, printed once per refresh cycle** before the data
  (`watch.go`, bold: "Refreshing run status every 3 seconds. Press Ctrl+C to quit.") —
  answers "is it hung or idle?" without a spinner. **Caps section headers** (`JOBS`,
  `ANNOTATIONS`) group the blocks.
- **The closing line is TTY-gated**: `if opts.IO.IsStdoutTTY()` before "Run … completed
  with …" (`watch.go`). A pipe gets the stream, not the epilogue. Whole-screen refresh
  uses the alternate screen buffer — a TTY-only affordance, out of scope for us, but it
  keeps redraw off stdout's append-only history.
- Times use `text.FuzzyAgo` ("2m30s ago") — relative, human-boundary, grey/muted.

### 2.2 `gh run list` / `gh pr list` — tables that reshape per TTY

Reconstructed from source ([run/list.go](https://github.com/cli/cli/blob/trunk/pkg/cmd/run/list/list.go),
[pr/list.go](https://github.com/cli/cli/blob/trunk/pkg/cmd/pr/list/list.go)):

```text
STATUS  TITLE                        WORKFLOW   BRANCH      EVENT  ID         ELAPSED  AGE
✓       Fix memory leak in parser    ci         main        push   123456789  2m30s    2h ago
X       Ship the onboarding check    ci         feature/x   push   123456780  45s      3h ago
```

- **The pipe output is not just the TTY output minus colour.** On a pipe, `run list`
  replaces the single symbol cell with *two textual columns*
  (`tp.AddField(string(run.Status)); tp.AddField(string(run.Conclusion))`), and
  `pr list` drops the `#` prefix and *adds* a STATE column
  (`if !isTTY { headers = append(headers, "STATE") }`). The machine format carries more
  words, not fewer.
- Colour roles: PR number coloured by state (`ColorForPRState`), branch cyan, title bold,
  ID cyan, age muted (`.AddTimeField(..., cs.Muted)`) — grey context at the right edge,
  scanning target on the left. gh's palette is 16-colour (`mgutz/ansi`), 256/truecolor
  only behind capability gates; "gray" is `black+h` = **bright black, SGR 90** — the same
  grey we chose, for the same conhost reason
  ([iostreams/color.go](https://github.com/cli/cli/blob/trunk/pkg/iostreams/color.go));
  every colour func returns text unchanged when `Enabled` is false.

### 2.3 git — plain structure first, one hue to mark identity, progress on stderr

Captured locally (piped stdout unless noted; git 2.55.0.windows.5, this repo):

```text
$ git status
On branch main
Your branch is up to date with 'origin/main'.

nothing to commit, working tree clean

$ git log --oneline -3                        # piped: zero escapes
24c3254 feat: colour the feed by role (ticket 02)

$ git -c color.ui=always log --oneline -3      # escapes shown as <ESC>
<ESC>[33m24c3254<ESC>[m feat: colour the feed by role (ticket 02)

$ git -c color.ui=always log -1
<ESC>[33mcommit 24c3254f838502f04ab6cee0795aedd357981331<ESC>[m
Author: AG2904 <…>
Date:   Wed Oct 21 18:44:32 2026 -0400

    feat: colour the feed by role (ticket 02)

# progress — observed on stderr during a sparse checkout (shell flagged each line stderr):
Cloning into '…/cargo-src'…
Updating index flags: 99% (3048/3078), done.
Updating files: 100% (20/20), done.
```

- Git's structure is **text**: indentation (4-space body), keyword lines, field prefixes
  (`Author:`, `Date:`). Colour touches exactly one thing — the SHA/identity — yellow (33),
  reset (`<ESC>[m`, not `<ESC>[0m`). Everything else is default weight.
- On a clean tree `git status` printed **zero escapes even with colour forced**: git
  colours *state entries* (changed files), not chrome. Colour marks what changed.
- Progress goes to **stderr**, is per-line self-updating (`99% (3048/3078)`), and ends
  with a permanent `done.` summary line — the final line survives even where the running
  line was overwritten.
- When piped, `git status`/`log` degrade to the same plain text — the structure, not the
  decoration, is the interface. (cf. clig.dev quoting the same output as the reference
  example for "Make it easy to see the current state of the system".)

### 2.4 cargo — right-aligned status-word column, everything human on stderr

Reconstructed from source
([crates/cargo-util-terminal/src/shell.rs](https://github.com/rust-lang/cargo/blob/master/crates/cargo-util-terminal/src/shell.rs));
the alignment comes from the literal format
`write!(&mut buffer, "{style}{status:>12}{style:#}")` in `ShellOut::message_stderr`:

```text
   Compiling serde v1.0.203 (/…)
   Finished dev [unoptimized + debuginfo] target(s) in 12.34s
```

(`{:>12}` right-justifies the status word in a 12-column gutter. Word list verified at
call sites: `status("Compiling", &unit.pkg)` and `status("Finished", message)` in
[src/compiler/job_queue/mod.rs](https://github.com/rust-lang/cargo/blob/master/src/compiler/job_queue/mod.rs),
`"Updating"` in [src/sources/registry/http_remote.rs](https://github.com/rust-lang/cargo/blob/master/src/sources/registry/http_remote.rs),
`"Running"` only under `.verbose(...)`.)

- **A fixed-width verb gutter**: the *object* starts at one predictable column while the
  eye scans states down the left. `Shell::status` is documented "right-align and color
  green"; `error` is a red `error:` prefix, `warn` amber `warning:`, `note` cyan — one
  word and one colour per state, forever.
- **Streams are split**: statuses go to **stderr** (`message_stderr` → `self.stderr()`);
  machine output to stdout (`print_json` writes `self.out()`). Colour is `CargoAuto` →
  anstream `Auto` (strips escapes when not a TTY); `--color always|never` overrides.
- **Progress is capability- and environment-gated**: `progress_supported()` is false for
  `-q`, `TERM=dumb`, or any CI. A live progress line is erased before each status print —
  and only `if err_supports_color()`; on a non-colour stream cargo never emits the erase
  sequence.

### 2.5 docker compose logs — per-source padded prefix + rotating colour

Reconstructed from source format strings
([cmd/formatter/logs.go](https://github.com/docker/compose/blob/main/cmd/formatter/logs.go),
[cmd/formatter/colors.go](https://github.com/docker/compose/blob/main/cmd/formatter/colors.go),
[pkg/compose/printer.go](https://github.com/docker/compose/blob/main/pkg/compose/printer.go)):

```text
web-1 | GET /health 200 1.2ms
db-1  | database system is ready to accept connections
web-1 exited with code 0 (restarting)
```

- **The prefix is data, not decoration**: `fmt.Sprintf("%-"+width+"s | ", name)` — every
  service name padded to the widest one, then ` | `. Interleaved lines from many sources
  stay attributable without colour, and `--no-log-prefix` exists precisely because the
  prefix is the machine-readable part
  ([logs reference](https://docs.docker.com/reference/cli/docker/compose/logs/)).
- **Colour = identity, rotating**: each new service takes the next of ten colours
  (`rainbowColor()` cycles cyan, yellow, green, magenta, blue, then bright variants — all
  plain 16-colour SGR). Colour answers "which stream is this line from?" — never "how
  important is it?".
- **Status events break the pattern on purpose**: `printer.HandleEvent` turns lifecycle
  events into `web-1 exited with code 0 (restarting)` — no prefix padding, prefixed by a
  line-clear (`goterm.RESET_LINE`) so the message owns the line.
- Colour is off when `--no-color` or when stdout is not a terminal
  (`useAnsi`: `Auto → streams.Out().IsTerminal()`); the prefix and the text survive pipes
  unchanged.

### 2.6 bun and npm — install summaries

Captured locally (network to the registry was flaky, so the success tail of bun is
unverified — see §5):

```text
bun install v1.4.2 (744846f84)
Resolving dependencies
Resolved, downloaded and extracted [6]
error: ConnectionClosed downloading package manifest is-odd
error: is-odd@3.0.1 failed to resolve
```

```text
$ npm install --no-audit --no-fund          # empty dependency set
up to date in 3s
```

- **bun**: a version banner line first (tool identity + version + short hash), then
  *phase words* ("Resolving dependencies", "Resolved, downloaded and extracted [6]") —
  one line per phase, counts in brackets. The phase lines went to **stderr** (the shell
  flagged them); stdout carries the result. (The "N packages installed" success summary
  could not be captured here — flagged unverified in §5.)
- **npm**: the entire no-op result is one line, and the **duration is last**:
  `up to date in 3s`. Success is a statement of state, not a celebration — exactly what
  cli.style #31 forbids.

### 2.7 The two specs

[clig.dev](https://clig.dev/) (Output / Errors sections):

- "Human-readable output is paramount" — TTY detection is the heuristic for who reads
  which stream; "Send messaging to `stderr`"; "Send output to `stdout`".
- "Use color with intention… Don't overuse it — if everything is a different color, then
  the color means nothing and only makes it harder to read." Color must die for: non-TTY
  stdout/stderr (checked per stream), non-empty `NO_COLOR`, `TERM=dumb`, `--no-color` —
  exactly our escape discipline. "If stdout is not an interactive terminal, don't display
  any animations."
- "If you change state, tell the user" and "Make it easy to see the current state of the
  system" — both exemplified with `git push` and `git status` (§2.3).
- Errors: "Signal-to-noise ratio is crucial"; "Consider where the user will look first.
  Put the most important information at the end."

[cli.style](https://cli.style/) (numbered principles):

- **#29 output is a budget** — "Every line printed is spent from someone's finite
  budget"; **#31 say little on success** — "Print what was produced, or print nothing."
- **#32 colour is an enhancement, never information** — with a worked example: a status
  shown green/red only "looks identical… without ANSI"; the fix is `✓ healthy …` /
  `✗ degraded … (node-3 unreachable)` — word (+glyph) first, colour second. This is
  literally our spec's "colour is never the only signal" rule.
- **#33 progress goes on stderr, and only for a terminal** — "TTY stderr: `[====> ] 45%`…
  non-TTY stderr: `draining prod-2... done (14.2s)`" — the milestone-line fallback.
- **#35 design the empty result** — on a terminal say what was searched (`No clusters in
  eu-west-1.`), under `--json` return `[]`, when piped emit **zero** lines so `wc -l`
  counts zero.
- **#36 line-oriented plain output**, **#37 stream results as you get them** (the target
  usage: `app logs tail prod-2 | grep --line-buffered ERROR`), **#45 one sentence first,
  detail underneath** — "Nobody reads down; everybody reads the first line."
- **#39 detect the terminal per stream** — stdout redirected while stderr is a terminal is
  the normal case.

## 3. Cross-cutting patterns

Device → problem solved → portability cost under our constraints.

1. **Fixed-width right-aligned state-word gutter** (cargo `{:>12}`; word in colour, rest
   default) → scannable state column while content keeps one predictable start column →
   cost: none — ASCII padding + one SGR colour; conhost-safe; the word survives pipes.
2. **State = word + colour, never colour alone** (gh `✓/X/*/‑` + green/red/yellow/muted;
   cli.style #32) → glanceability without literacy in our palette → cost: the *glyphs*
   are the risky part (gh's `✓` is non-ASCII; its `X` deliberately is not). Our spec
   already keeps words as the signal; glyphs stay out.
3. **Padded source prefix + ` | ` (compose) / two-space indent (gh)** → grouping and
   attribution without blank-line cost → cost: prefix padding needs the widest name
   *in advance* — fine for a grouped report (batch known), wrong for an incremental feed
   (rows can't be re-padded after print).
4. **Rotating per-source colour** (compose `rainbowColor`, all 16-colour SGR) →
   interleaved streams stay separable → cost: this is colour-as-identity, not
   colour-as-role; under our palette rule it would need a role argument ("which repo is
   this?" — plausible for multi-repo feeds, decorative for one repo).
5. **Lifecycle chrome: start banner, per-cycle refresh line, TTY-only epilogue** (gh
   `run watch`; bun version banner; gh's `if opts.IO.IsStdoutTTY()`) → "what is this
   screen and is it alive?" → cost: must be TTY-gated to keep the pipe byte-clean; must be
   uncoloured (our spec puts the banner colour out of scope anyway).
6. **Muted grey = bright black (90), time last** (gh `gray = black+h`, `AddTimeField(…, cs.Muted)`;
   npm `in 3s`) → de-emphasis and recency-as-context that survive every VT terminal →
   cost: none; corroboration that our grey and time placement match gh's.
7. **Verbosity tiering of detail** (cargo `"Running"` only under `-v`; gh steps only on
   failure or `--verbose`) → the default screen stays short → cost: none; our "verbose"
   layer is the grouped/history report, which already exists.
8. **Line-clear before overwrite, gated on colour support** (cargo
   `err_erase_line` only `if err_supports_color()`; compose `RESET_LINE`) → a live redraw
   can never corrupt a pipe → cost: we have no redraw; the takeaway is the gate: any
   future "waiting…" indicator must be stderr + TTY-gated.
9. **A pipe changes structure, not just colour** (gh run list gains text status columns;
   compose keeps prefixes; cli.style #35's empty-state triad) → machines parse words →
   cost: our equivalent already exists (URL as fourth line, `--json` untouched).

## 4. Candidate designs for our feed

Each is rendered with our two spec events. "Breaks" refers to sections of
`.scratch/cli-output/spec.md` (Solution / Implementation Decisions / Further Notes /
Comments are binding).

### 4.1 Candidate A — the verb gutter (cargo's status column on line 1)

```text
    merged maria.perez into main
      Fix memory leak in parser component
      AG2904/github-activity-feed · [bug] · 2h ago

  assigned lucia.gomez to tomas.ferreira
      Ship the onboarding checklist
      AG2904/github-activity-feed · [bug] · 2h ago
```

The verb right-aligns in a fixed 8-wide gutter (`merged`→col 3, `assigned`→col 1), in its
role colour (green/amber), rest of the line unchanged; lines 2–3 identical to the current
card. A burst becomes a left-edge scan of *what kind of thing happened*, at the cost of
ragged sentence starts.
**Cost under constraints:** none — ASCII spaces + the SGR colours we already use; widths
are constants, no terminal-width probing.
**Breaks:** Solution ("a sentence… `actor verb [destination]`") and Implementation
Decisions → "The card" bullet line 1 (order becomes verb-first); user story 1's
actor-first reading; the line-1 fixture in Testing Decisions. Nothing else moves.

### 4.2 Candidate B — the ledger row (gh run list's table, one line per event)

```text
EVENT    ACTOR          TITLE                             REPO                     AGE
merged   maria.perez    Fix memory leak in parser …       AG2904/github-…-feed     2h ago
assigned lucia.gomez    Ship the onboarding checklist     AG2904/github-…-feed     2h ago
```

**Cost under constraints:** this is where the evidence bites. gh can pad columns because
`run list` fetches the whole batch before printing (`tableprinter` over a slice); a live
feed emits events one at a time and **cannot re-pad rows already on screen**. Fixed
column widths mean truncation — which is explicitly out of scope ("Wrapping, truncation,
paging"). Truncation-free rows only fit short titles, and the URL line has no column.
Labels would need yet another column or their own row.
**Breaks:** essentially all of the Solution; user story 4 ("one event = one card");
Testing Decisions fixtures wholesale; and it *revives the design rejected on 2026-10-21*
(Comments: the two-line aligned-columns shape, rejected as *escueto*). Recommended only as
the documented "we checked the table road and here is why it fails a stream" direction.

### 4.3 Candidate C — session chrome around the unchanged card (gh watch's lifecycle lines)

Cards stay exactly as specced. The emitter adds TTY-only, colourless session lines:

```text
feed · watching AG2904/github-activity-feed · connected     ← start (TTY only)

maria.perez merged into main                                ← card, unchanged
  Fix memory leak in parser component
  AG2904/github-activity-feed · [bug] · 2h ago

…waiting for events…                                        ← after long silence (TTY only)

3 events · 1 merged · 2 assigned · Ctrl+C to stop           ← exit summary (TTY only)
```

An empty start prints the cli.style #35 empty state instead: `no events yet · watching
AG2904/github-activity-feed`.
**Cost under constraints:** near zero. Every chrome line is gated on a TTY (gh's
`if opts.IO.IsStdoutTTY()` precedent), carries no escapes (spec already rules the banner's
colour out of scope), and vanishes on a pipe and under `--json`. The "waiting" line is a
plain line printed once, never a redraw — so the erase-gating problem (§3.8) never arises.
**Breaks:** no binding Solution/Implementation Decision — but it *extends* Testing
Decisions (fixtures gain session lines; the "first card is not preceded by a blank line"
invariant must define chrome spacing). The emitter/renderer split stays intact: chrome is
emitter work.

### 4.4 Candidate D — the reason rides with the title (labels move to line 2)

```text
maria.perez merged into main
  [bug] Fix memory leak in parser component
  AG2904/github-activity-feed · 2h ago
```

The label — the *reason* the event interrupted you — sits immediately before the thing it
qualifies, bold red, title stays bold default + OSC 8; the context line loses its label
segment. Precedent: cargo puts the qualifier next to the object (`Compiling serde v1.0.203`),
compose puts the source prefix at the start of the content line; cli.style #45 puts the
load-bearing word first. For an assignment without labels nothing changes.
**Cost under constraints:** none mechanically (same segments, same SGR); the design cost is
two bold-red-plus-bold items adjacent on one line — on conhost's fixed palette the title
may need to drop bold when labels are present, or the labels keep red without bold.
**Breaks:** Implementation Decisions → "The card" line 3 (`repository · [labels] · time`)
and the "Labels are shown … in the context line" decision; Testing Decisions ("with the
label segment"; "a legacy event with `repository: null` starts the context line at the
label"). User story 3 survives — arguably strengthens, since "why" now reads before "what".

### 4.5 What none of them touch (for the record)

`--json`, the wire format, the relay, the escape discipline, the 16-colour palette, the
blank-line rhythm, and the fourth URL line are identical in all four candidates. Highest
evidence-per-cost: Candidate C — every tool surveyed that runs for a living (gh watch,
bun, compose) says what it is and whether it is alive, and says it where a machine will
never see it.

## 5. Sources

(Inline links above point at the same files; plain list for copy-paste.)

- https://clig.dev/ — Output, Errors sections
- https://cli.style/ — principles 28–37, 45
- https://github.com/cli/cli/blob/trunk/pkg/cmd/run/watch/watch.go
- https://github.com/cli/cli/blob/trunk/pkg/cmd/run/shared/shared.go
- https://github.com/cli/cli/blob/trunk/pkg/cmd/run/shared/presentation.go (+ `presentation_test.go` — verbatim fixtures)
- https://github.com/cli/cli/blob/trunk/pkg/cmd/run/list/list.go
- https://github.com/cli/cli/blob/trunk/pkg/cmd/pr/list/list.go
- https://github.com/cli/cli/blob/trunk/pkg/iostreams/color.go
- https://github.com/rust-lang/cargo/blob/master/crates/cargo-util-terminal/src/shell.rs (+ https://docs.rs/cargo-util-terminal/latest/cargo_util_terminal/struct.Shell.html)
- https://github.com/rust-lang/cargo/blob/master/src/compiler/job_queue/mod.rs (Compiling/Finished call sites)
- https://github.com/rust-lang/cargo/blob/master/src/sources/registry/http_remote.rs (Updating call site)
- https://github.com/docker/compose/blob/main/cmd/compose/logs.go
- https://github.com/docker/compose/blob/main/cmd/formatter/logs.go (+ colors.go — prefix and colour rotation)
- https://github.com/docker/compose/blob/main/pkg/compose/printer.go (lifecycle events)
- https://docs.docker.com/reference/cli/docker/compose/logs/
- https://fly.io/docs/flyctl/logs/ (flags only, incl. `--json`; no output sample)
- local captures: git 2.55.0.windows.5 (this repo), bun 1.4.2 (`%TEMP%\opencode\bun-feed-research`), npm (`%TEMP%\opencode\npm-feed-research`)

**Could not be verified (not quoted as verbatim anywhere above):** bun's success summary
line (`N packages installed`, network died mid-install here); npm's fuller install output
(`added N packages in Xs`); `kubectl get events --watch` and `wrangler tail` (not
installed; no output sample on their official doc pages); flyctl log rendering (docs show
flags, no sample).

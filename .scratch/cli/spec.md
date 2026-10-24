# CLI — The terminal feed

Status: ready-for-agent

## Problem Statement

The relay keeps only the two critical events — a pull request merged into `main`, and an issue assigned while carrying a `bug` or `feedback` label — persists them for audit, and streams them over Server-Sent Events. Nothing consumes that stream. A developer still finds out by opening GitHub in a browser, and the audit history is reachable only with `curl` against `/history`.

What is missing is the piece that puts the event where the work is happening: the terminal.

## Solution

A single static Go binary. It holds an SSE connection open to the relay and paints each event as it arrives, two lines per event; it also answers a retrospective question about the stored history, optionally grouped by repository. It never polls, needs no login, and keeps no state between runs.

The relay's read side is an unauthenticated broadcast (ADR-0001), so the CLI carries no identity: every client receives every event and filters for itself. That is why filtering is local, and why the binary is a display client with nothing of its own to dedupe.

## User Stories

1. As a developer, I want one binary I can copy and run, so that receiving events needs no runtime installed on my machine.
2. As a developer, I want to tell the CLI where the relay is, once, so that I don't retype the host every time.
3. As a developer, I want to hold the connection open and see each event appear as it happens, so that I never poll.
4. As a developer, I want each event on a couple of readable lines, so that I can tell what happened at a glance.
5. As a developer, I want an assignment's assignee shown prominently, so that I see who the work landed on without reading the title.
6. As a developer, I want a link I can click straight from the terminal, so that I can jump to the pull request or issue.
7. As a developer, I want timestamps relative to now, so that I know how fresh an event is.
8. As a developer, I want the CLI to survive a dropped connection, so that I don't restart it.
9. As a developer, I want to be told when the connection drops and comes back, so that I can tell silence from a dead connection.
10. As a developer, I want the CLI to notice a connection that died silently, so that I don't wait for events that can never arrive.
11. As a developer, I want the first connection to show me recent events, so that I know the CLI works before the next event happens.
12. As a developer, I want the output to stay clean when I pipe it, so that I can use the CLI in a script.
13. As a developer, I want to filter the live feed to my own events, so that I'm not reading what isn't mine.
14. As a developer, I want to filter by repository, so that I can watch only the project I care about.
15. As a developer, I want to ask the history what a teammate merged or was assigned, so that I can reconstruct it after the fact.
16. As a developer, I want the history grouped by repository, so that I can read it as a per-project record.
17. As a developer, I want the raw event JSON on demand, so that I can script against the CLI.
18. As a developer, I want the same event shape from the stream and the history, so that I parse only one thing.
19. As a developer, I want the binary to know its own version, so that I can tell which build I am running.
20. As a team lead, I want a static binary per platform, so that no developer installs a runtime to use it.
21. As a developer, I want Ctrl-C to exit cleanly, so that it composes with my shell.

## Implementation Decisions

- **Scope**: two subcommands, `watch` and `history`, dispatched explicitly. Neither is implied, so a bare `feed` prints usage and exits rather than silently holding the terminal. Filtering is by user and by repository; filtering by label or event type is not offered, because with two event types it answers a question nobody asked.
- **Configuration**: the relay's location resolves as flag over environment over default — `--relay`, then `RELAY_URL`, then `http://localhost:3000`. The relay's read side requires no credentials (ADR-0001), so the CLI carries no token and has nothing to log in to.
- **Realtime**: the CLI speaks SSE with a hand-rolled parser (ADR-0007). The relay names every frame `id: <n>`, but that counter is per-connection and starts at zero, so it identifies nothing a client can use; `id:`, `event:`, `retry:`, and comment frames are ignored, and only `data:` payloads become events. A frame that does not parse is discarded rather than fatal — the same stance the relay takes on a hook it does not understand.
- **Liveness**: the relay heartbeats a stream that has nothing to say (relay ticket 08), so even a silent connection now carries a byte every fifteen seconds. The CLI still treats five minutes without a byte as death and reopens, now as a backstop for what a heartbeat cannot cover: a connection that dies inside a heartbeat interval, and a relay that stops writing without closing the socket. Reconnection backs off exponentially from one second to a thirty-second ceiling with jitter, and the backoff resets on the first event received. A relay that is unreachable at startup is retried indefinitely: the CLI is a watch, and dying because the relay was restarting is worse than waiting.
- **No replay**: a reconnect never replays what was already printed, and the relay buffers nothing per connection, so events that happen while the CLI is disconnected are not recoverable from the stream. `feed history` is how they are recovered. This is the honest cost of not persisting a cursor client-side.
- **Backfill**: the first connection queries the stored history, prints the most recent `--backfill` events (default 10, `0` disables), and then follows the stream. It runs on the first connection only: a backfill on every reconnect would reprint history and undo the relay's own dedupe work. The backfill is flat whatever `--group-by-repo` says, because it is the head of a stream and not a report.
- **Normalization**: none. The CLI parses the nine-field shape (ADR-0013) and re-serializes nothing; `--json` writes the relay's own bytes (user story 18). Re-marshalling would silently break the contract — key order, `assignee: null`, and the `.000Z` timestamp form survive only if the bytes are passed through.
- **Filtering**: `--user` matches a user's **Involvement** — the glossary's word for being an event's actor or its assignee — which is the same rule the relay applies to `/history?user=`, so the two never disagree about what a user's events are. `--repo owner/name` matches the event's `repository`, which the relay reports as a field of the shape (ADR-0010) and which the CLI reads rather than derives; an event stored before the field existed arrives with `repository: null` and matches no repository filter. Filtering happens locally and never narrows what the relay sends.
- **Rendering**: two lines per event — a type badge, the assignee when there is one, and the subject on the first; repository, actor, and relative time on the second. The subject carries an OSC 8 hyperlink to the event's `url`, and the raw URL is printed instead whenever escape sequences are not in use, because a subject with no link is a dead end. Colour and hyperlinks are used only when stdout is a terminal, and are disabled by `--no-color` or by `NO_COLOR`. *(Shape superseded by [`.scratch/cli-output/spec.md`](../cli-output/spec.md): three-line cards with colour by role. The hyperlink and escape rules below still hold.)*
- **Process contract**: stdout carries events and nothing else, so the CLI composes with `tee` and `jq`; stderr carries the connection banner, reconnect notices, and errors. Ctrl-C exits 0 after flushing. An unreachable relay exits 1 for `history` and is retried indefinitely for `watch`. A flag error exits 2, as the standard library already does.
- **Distribution**: static binaries for windows/amd64, linux/amd64, darwin/amd64, and darwin/arm64, built with `CGO_ENABLED=0` and `-trimpath`, with the version injected through `-ldflags` so a distributed binary can say where it came from. The build is driven by `cli:`-prefixed npm scripts, because the repository already speaks npm and `make` is not installed anywhere this project is developed.
- **Dependencies**: none beyond the standard library, including `flag` rather than a CLI framework (ADR-0008). Two subcommands and a handful of flags do not justify a framework, and the core lives in packages that know nothing about flags, so adopting one later would replace the shell and not the interior.
- **Module**: a Go module at `github.com/AG2904/github-activity-feed/client`, nested in `client/` inside the monorepo (ADR-0009).

## Testing Decisions

- **One seam: the process boundary.** Tests compile the binary, run it against a fake relay that speaks the relay's wire format, and assert on stdout, stderr, and the exit code. This covers flag handling, configuration, the SSE parser, filtering, the backfill, rendering, and `--json` with one seam, and mirrors the relay's own choice of a single boundary over poking at internals.
- **What makes a good test here**: it runs the binary and asserts on what a developer would see. No test imports an internal package, and no test reaches into the SSE parser or the renderer directly.
- **Fixtures**: the frame bytes the relay actually writes, taken from the captured probe in the ticket 06 record and from the manual runbook — `data: {"type":"pr.merged",...}` with the eight keys — plus the frames a broken relay would send: an `id:`-only frame, a comment frame, a truncated payload, and a body that is not JSON.
- **Known gap**: the fake relay speaks a frozen format, so a change to the relay's wire format is not caught here. It is caught by the manual runbook (ticket 08) and by the Phase 3 demo. More tests do not close this gap; only running against the real relay does. Closed on 24 October 2026 by the validation demo (ticket 04): a real `github.com` delivery travelled through the real relay and the CLI rendered it.
- **Streaming**: the stream is tested by holding a connection open against the fake relay and asserting that a frame written after the CLI connected appears on stdout, not by inspecting the parser.

## Out of Scope

- Any write from the CLI to the relay. The channel is one-way by design (ADR-0001).
- Per-developer identity, authentication, or routing on the stream.
- A heartbeat on the relay's stream: built as relay ticket 08, outside this phase and no longer a dependency. The client's own idle timeout stays as a backstop.
- The `repository` field in the `ActivityEvent` shape: built as relay ticket 09, outside this phase, and read rather than derived (ADR-0010).
- A terminal UI, desktop notifications, sound, or running as a daemon.
- Filtering by label or by event type.
- Retention or pruning of the audit history.
- CI, release automation, and distributing the binary to the team (Phase 3).

## Further Notes

- **Precondition**: the repository is renamed to `github-activity-feed` before the first commit of this phase. The Go module path is written into every import, so renaming after the first commit costs more than renaming before it.
- The relay's frame carries `id:` on every message, but that value is a counter that starts at `0` on each connection. A generic SSE client reads it as `Last-Event-ID` and tries to resume from a position the server does not have, and brings its own reconnect policy. That is why the parser is hand-rolled (ADR-0007) and why no cursor is persisted between runs.
- A reconnect leaves a hole: events that happen while the CLI is disconnected never appear on the stream. The CLI does not paper over this with a second backfill, because reprinting history on every flap is worse than a hole that `feed history` can fill.
- The relay acknowledges every authenticated webhook identically whether it stored, ignored, or deduped the delivery, so the CLI cannot infer anything from the relay's response. It does not try.

## Comments

- 2026-10-17: Grilling session that produced this spec, working the design tree in four rounds. Decisions that carried the most weight: the scope is the live feed plus a history query, because the glossary and the architecture document have promised `feed --user` since before the relay existed; a live stream that starts on a blank terminal is a broken first impression, so the first connection backfills; and the relay sends no heartbeat, so liveness is the client's problem and the CLI must not depend on a Phase 1 change to be correct.
- 2026-10-17: Three consequences were accepted knowingly rather than discovered later. A reconnect leaves an unrecoverable hole in the live view, because there is no replay and no cursor. The repository is derived from the event's URL, which makes a link carry data (ADR-0006). And the client ships with no third-party dependency at all, giving up generated help and completion (ADR-0008).
- 2026-10-17: Two follow-ups were filed against the relay rather than folded into this phase: a heartbeat on the stream, and a `repository` field on the shape. Both are recorded as `needs-triage` under `.scratch/relay/issues/`, and the CLI works without either.
- 2026-10-17: Relay ticket 09 landed the `repository` field, so the third consequence recorded above — that the client derives the repository from the URL and a link carries data — no longer exists, and neither does the derivation: `--repo` reads the field (ADR-0010). The CLI does not need `url` for anything but the hyperlink now. One consequence is worth naming rather than hiding: the field is nullable for rows stored before the column existed, so an old event arrives with `repository: null`, matches no `--repo` filter, and groups under nothing rather than under an invented name.
- 2026-10-17: Relay ticket 08 landed the heartbeat, so the premise the Liveness decision above was argued from — that the relay sends no heartbeat — is no longer true, and the entries that call it an open follow-up are superseded by this one. The client's five-minute idle timeout stays, now as a backstop for what a heartbeat cannot cover: a connection that dies inside a heartbeat interval, and a relay that stops writing without closing the socket. Nothing in the client's design changes, and that is the point — the heartbeat travels as an SSE comment, so the parser's contract and the event stream are both untouched.

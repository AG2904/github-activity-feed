# 03: The notification frame

**What to build:** on a terminal that understands it, a shown event becomes a real
desktop notification with a title and a body, delivered as an escape sequence the
client builds itself. `--notify=osc` writes exactly one frame per shown live
event, choosing the protocol the terminal is known to speak, and the text is the
plain sentence from ticket 02 plus the repository and the subject. Event text is
never trusted: every field passes through the renderer's control-byte cleaner, a
semicolon in a field becomes a space, bidi overrides are dropped, the title is
clamped to 64 runes and the body to 200, and the URL never travels. On a terminal
that does not understand the frame, nothing appears in the feed and the watch
keeps running. The frame is produced by one constructor that takes an event, so a
test can pin its shape without a console. This is the client's first unit test.

**Blocked by:** 01 (the call site and the sink), 02 (the sentence)

**Status:** ready-for-human

Chequeo visual pendiente: `--notify=osc` en una terminal soportada, anotando la
terminal y su versión en `docs/cli-testing.md` (paso 13). La forma del frame ya está
fijada por `client/internal/notify/frame_test.go`.

- [ ] `--notify=osc` produces one system notification per shown live event on a
      supporting terminal; the by-hand check and the terminal used are recorded in
      the manual testing document.
- [x] The frame carries exactly one introducer and one terminator.
- [x] No control byte appears inside a field, for event text carrying `ESC`,
      `BEL`, C0/C1, a newline, a tab and a semicolon.
- [x] A payload crafted as a complete escape sequence produces one frame and no
      second introducer: the text can never start a sequence of its own.
- [x] A semicolon in a field is rendered as a space, so the title and the body
      cannot be rewritten by event text.
- [x] The title is clamped to 64 runes and the body to 200; an over-long subject
      neither truncates mid-rune nor stalls the write.
- [x] Bidi override characters are dropped; the URL and any markup or hyperlink
      are absent from every frame.
- [x] On a terminal without support the frame is dropped silently: no bytes reach
      the feed, and the watch keeps running.
- [x] The choice of protocol per terminal is recorded in the flow document, and
      the frame's contract is traceable to ADR-0017.
- [x] The shipped client still imports no process-spawning package.

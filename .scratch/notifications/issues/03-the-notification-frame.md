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

**Status:** ready-for-agent

- [ ] `--notify=osc` produces one system notification per shown live event on a
      supporting terminal; the by-hand check and the terminal used are recorded in
      the manual testing document.
- [ ] The frame carries exactly one introducer and one terminator.
- [ ] No control byte appears inside a field, for event text carrying `ESC`,
      `BEL`, C0/C1, a newline, a tab and a semicolon.
- [ ] A payload crafted as a complete escape sequence produces one frame and no
      second introducer: the text can never start a sequence of its own.
- [ ] A semicolon in a field is rendered as a space, so the title and the body
      cannot be rewritten by event text.
- [ ] The title is clamped to 64 runes and the body to 200; an over-long subject
      neither truncates mid-rune nor stalls the write.
- [ ] Bidi override characters are dropped; the URL and any markup or hyperlink
      are absent from every frame.
- [ ] On a terminal without support the frame is dropped silently: no bytes reach
      the feed, and the watch keeps running.
- [ ] The choice of protocol per terminal is recorded in the flow document, and
      the frame's contract is traceable to ADR-0017.
- [ ] The shipped client still imports no process-spawning package.

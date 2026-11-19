# 02: The sentence exists once

**What to build:** the card and the notifier describe the same event with the same
sentence. The renderer exposes the plain sentence an event produces — `actor
merged <branch> into main`, `actor assigned to <assignee>`, `actor closed`, `actor
dismissed as not planned`, and the raw type for anything unknown — and the
coloured card keeps composing its role-coloured form from that same source of
truth. This is a prefactor: it changes no byte of what a reader sees today, and it
exists so the notification cannot drift from the feed. It is the first ticket that
needs no console and no terminal to be verified.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Every event type has one plain sentence, derived from the event alone.
- [ ] The rendered card is byte-identical to the base commit for every existing
      expectation, in colour and with colour off.
- [ ] A test pins that the coloured card with its escapes removed equals the plain
      sentence, for the four known types and for an unknown one.
- [ ] An unknown type still degrades to its raw value in both forms.
- [ ] No behaviour, flag or output contract changes.

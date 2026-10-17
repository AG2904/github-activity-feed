# 03: `--json` hands the contract over untouched

**What to build:** A raw output mode that writes the relay's own bytes instead of a rendered form, so the CLI is scriptable without a second parser for the shape.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] `feed watch --json` writes one line per event: the frame's `data:` payload, byte for byte, with no re-serialization.
- [ ] The emitted line preserves the eight keys in the relay's order, `assignee: null` for an event that has none, and the `.000Z` timestamp form.
- [ ] `feed history --json` writes the relay's response body verbatim.
- [ ] `--json` combined with `--group-by-repo` exits 2 with a message on stderr: grouping is presentation, and raw output has none.
- [ ] `--json` suppresses colour and hyperlinks regardless of whether stdout is a terminal.

## Comments

- 2026-10-17: Small ticket, sharp invariant. Re-marshalling through Go's `encoding/json` would reorder keys and could re-shape `null`, which is exactly the drift user story 18 exists to prevent. Passing the bytes through makes that impossible rather than unlikely.

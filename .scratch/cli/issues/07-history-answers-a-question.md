# 07: `feed history` answers a question

**What to build:** A one-shot query against the relay's audit history, so a developer can reconstruct what a teammate merged or was assigned after the fact.

**Blocked by:** 05

**Status:** ready-for-agent

- [ ] `feed history` fetches the stored history once, prints it, and exits 0.
- [ ] With no filter it lists every stored event, in the order the relay serves it — newest first.
- [ ] `--user` and `--repo` filter it, with the same meaning they carry on the live stream.
- [ ] `--group-by-repo` prints one header per repository, in alphabetical order, keeping the relay's order inside each group and dropping the repository from the per-event lines.
- [ ] An unreachable relay exits 1 with a message on stderr.
- [ ] An empty history prints nothing and exits 0.

## Comments

- 2026-10-17: This is the `feed --user` the glossary and the architecture document have promised since before the relay existed, and the reason the relay serves one shape for both the stream and the history (user story 18). Grouping lives here and only here: a live stream is unbounded and unordered, so grouping is only meaningful over a finite list.

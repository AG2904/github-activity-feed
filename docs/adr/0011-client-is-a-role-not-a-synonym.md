# `client` is a role, not a synonym for `CLI`

The glossary defined **CLI** and listed `client` among the words to avoid — and then used "connected clients" in its own definition of **Relay**. The Phase 2 documents followed suit: "client" appeared nineteen times against thirty-four of "CLI", and ADR-0001 said "every connected **CLI** receives every ActivityEvent" while the CLI's own spec said "every **client** receives every event".

We added **Client** as a term and took it off CLI's avoid list. **CLI** is the artifact: the Go binary a developer runs. **Client** is the role: a process that holds the stream open or reads the audit history. The CLI plays it, and so does `curl` — which was the only client for as long as the CLI did not exist.

The distinction has to exist because the relay cannot know what is on the other end of the socket. "Every connected CLI receives every ActivityEvent" becomes false the moment something that is not our binary connects, and something that is not our binary connects today. A vocabulary that describes the system as we wish it were is worse than one with two words in it.

This closed a tension the relay's ticket 06 record had already noticed, and had deliberately left to domain modeling rather than settling it inside a feature ticket.

## Considered Options

- **Banning the word** and rewriting the nineteen uses. Cleaner on paper, and rejected: the relay genuinely needs a word for "whoever is holding the stream", and `CLI` is the wrong one. The ban was already being violated by the document that wrote it, which is the evidence that the word was carrying meaning the vocabulary had left no room for.
- **Using `CLI` for both**, the artifact and the role. Rejected for the same reason at a smaller scale: it is false for `curl`, and it would become false for any future client that is not this binary.
- **Leaving it**, since nothing breaks. Rejected because the contradiction was documented rather than accidental: a reader who follows the glossary's own instruction would rename the thing the relay's definition depends on.

## Consequences

The avoid lists are **per-term, not global bans**. A word under one term's `_Avoid_` is the wrong word *for that term*, and may be exactly right elsewhere — `payload` is the clearest case, since the glossary tells you not to call an ActivityEvent a payload while the code correctly calls GitHub's body one. That reading is what made adding **Client** the right fix rather than a capitulation, and it is the part a future contributor is most likely to get wrong.

**Client is a role, not a context.** The repository still holds two contexts — the relay and the CLI — and one of them plays the client role, so the glossary needs no `GLOSSARY-MAP.md`. If a third codebase ever arrives, that is when the map becomes the question.

Everywhere the relay talks about who receives, "CLI" became "client": ADR-0001, two comments in `activity-stream.ts`, and the research document, whose line was paraphrasing a user story that already said "client". The Spanish documents' "Cliente CLI" compounds became "el CLI", because they were naming the binary. Evidence records were left alone, as always: they describe what was true when they were written.

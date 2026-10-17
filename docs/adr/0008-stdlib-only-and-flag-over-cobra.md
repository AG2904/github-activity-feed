# The client has no dependencies beyond the standard library, and uses `flag`

The CLI is built on the standard library alone: `flag` for its two subcommands, `net/http` for the connection, `bufio` for the parser, `encoding/json` for the shape. No CLI framework, no SSE library (ADR-0007), no colour or table package.

A Go project of this shape usually reaches for `cobra`, and the case for it is genuine: generated help, shell completion, subcommand ergonomics, and the familiarity a reader brings to it. It is declined because two subcommands and a handful of flags do not carry it. `cobra` brings `pflag` and `mousetrap` behind it, and the CLI's own dispatch is about fifteen lines. This project has chosen minimal machinery at every other turn — no containers, no queue, no broker — and the same reasoning applies here.

The "single static binary, no dependencies on the developer's machine" promise does **not** argue for this, and it is worth saying so, because it is the tempting argument and it is false. A Go dependency is compiled into the binary, so `cobra` would produce exactly as static a binary as `flag` does. What is being accepted here is an ergonomic cost, not a structural one.

## Consequences

Help output, and any completion if it is ever wanted, are the project's to write and to keep correct.

The core lives in packages that know nothing about flags, so adopting `cobra` later — if the command surface grows, or completion becomes worth having — would replace the shell around `cmd/feed` and leave the interior alone.

`go.mod` carries no `require` directive. That is a property worth keeping, it is cheap to check, and it means the client's supply chain is the Go toolchain and nothing else.

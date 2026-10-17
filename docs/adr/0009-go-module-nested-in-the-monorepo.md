# The Go module is nested in `client/`, inside the monorepo

The client is a Go module at `github.com/AG2904/github-activity-feed/client`, living in a `client/` directory beside `src/`, `test/`, and `docs/`. One repository holds both contexts — the relay and the CLI — and each keeps its own toolchain.

A separate repository was the alternative, and it was rejected because the CLI's reason to exist is the relay's contract. The eight-field shape, the frame format, and the read endpoints *are* the interface between the two contexts, and a change on one side is a change the other must follow. Keeping them in one checkout makes that interface reviewable in a single diff, and makes the Phase 3 work — CI for both, a demo that runs both — one repository to configure instead of two to keep in step.

The cost is that a Go module now lives inside a repository whose root is a Node project. `go build ./...` does not work from the root, tooling has to change directory into `client/`, and the module path is written into every import — which is why the repository was renamed before the first line of Go was written, rather than after. A nested module is a supported and ordinary layout; it is the mixed-toolchain repository that a reader will notice.

## Considered Options

- **A separate `github-activity-feed-cli` repository**: cleaner toolchains, and each context versioned on its own schedule. Rejected — it would put the contract between the two contexts behind a repository boundary, and neither context is large enough to be worth that price.
- **A Go module at the repository root**, beside `package.json`: rejected outright. Two toolchains claiming the same directory is a fight with every tool that walks it, and `go.mod` and `package.json` would each read as the other's noise.

## Consequences

Every command for the client runs inside `client/`. The npm scripts at the root exist so a developer does not have to remember that: `cli:build`, `cli:test`, `cli:lint`, and `cli:dist` each change directory themselves. That couples the Node package manifest to the Go build, which is the price of one entry point per repository.

// Command feed is the terminal client for the GitHub activity relay. It holds
// the relay's SSE stream open, paints each ActivityEvent, and answers
// retrospective questions against the audit history.
//
// The binary carries no dependencies beyond the standard library (ADR-0008).
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/AG2904/github-activity-feed/client/internal/cli"
)

// version is injected at build time with
// -ldflags "-X main.version=<version>", so a distributed binary can say where
// it came from.
var version = "dev"

func main() {
	// Ctrl-C cancels the request or the backoff sleep; Run returns 0 and the
	// shell sees a clean exit. Restoring the default handler as soon as the
	// context is cancelled means a second Ctrl-C can still kill a hung run.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	go func() {
		<-ctx.Done()
		stop()
	}()

	code := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, version)
	stop()
	os.Exit(code)
}

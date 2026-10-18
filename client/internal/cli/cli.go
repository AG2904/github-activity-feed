// Package cli is the feed binary's interior: subcommand dispatch, relay
// configuration, and the two read paths. It never touches os.Args or os.Exit,
// so every test can drive it across the process boundary and nothing else.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

const defaultRelay = "http://localhost:3000"

// Run is the whole process contract: it returns the exit code the binary hands
// to the operating system. stdout carries events and nothing else; stderr
// carries the connection banner, notices, and errors.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, version string) int {
	root := flag.NewFlagSet("feed", flag.ContinueOnError)
	root.SetOutput(stderr)
	showVersion := root.Bool("version", false, "print the version and exit")
	root.Usage = func() { printUsage(stderr) }

	if err := root.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	rest := root.Args()
	if len(rest) == 0 {
		// Neither subcommand is implied: a bare `feed` explains itself instead
		// of silently holding the terminal.
		printUsage(stderr)
		return 2
	}

	switch rest[0] {
	case "watch":
		return watch(ctx, rest[1:], stdout, stderr)
	case "history":
		return history(ctx, rest[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "feed: unknown command %q\n\n", rest[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `feed shows the GitHub activity relay's critical events in the terminal.

Usage:
  feed watch   [--relay <url>] [--user <login>] [--repo <owner/name>] [--backfill <n>] [--no-color] [--json]
  feed history [--relay <url>] [--user <login>] [--repo <owner/name>] [--no-color] [--json | --group-by-repo]
  feed --version

The relay resolves as --relay, then $RELAY_URL, then %s.
`, defaultRelay)
}

// relayFallback is the flag's default, so a flag that is present wins over the
// environment without a second lookup.
func relayFallback() string {
	if relay := os.Getenv("RELAY_URL"); relay != "" {
		return relay
	}
	return defaultRelay
}

func newFlagSet(name string, stderr io.Writer, usage string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, usage) }
	return fs
}

func parseFlags(fs *flag.FlagSet, args []string) (code int, ok bool) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, false
		}
		return 2, false
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(fs.Output(), "feed %s: unexpected argument %q\n\n", fs.Name(), fs.Arg(0))
		fs.Usage()
		return 2, false
	}
	return 0, true
}

// colorEnabled is the single rule for escapes: never by --no-color or
// NO_COLOR, and only when stdout is a terminal. A pipe gets plain text, which
// is what makes `feed watch | tee` and `feed watch | jq` safe.
func colorEnabled(stdout io.Writer, noColor bool) bool {
	if noColor || os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := stdout.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(file)
}

func endpoint(relay, path string) string {
	return relay + path
}

// fetch opens one read request against the relay and refuses anything that is
// not a 200, so the two read paths agree about what a relay that answered badly
// looks like. The caller owns the body.
func fetch(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s answered %s", url, resp.Status)
	}
	return resp, nil
}

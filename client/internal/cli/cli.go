// Package cli is the feed binary's interior: subcommand dispatch, relay
// configuration, and the two read paths. It never touches os.Args or os.Exit,
// so every test can drive it across the process boundary and nothing else.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const defaultRelay = "http://localhost:3000"

// Run is the whole process contract: it returns the exit code the binary hands
// to the operating system. stdout carries events and nothing else; stderr
// carries the connection banner, notices, and errors.
func Run(args []string, stdout, stderr io.Writer, version string) int {
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
		return watch(rest[1:], stderr)
	case "history":
		return history(rest[1:], stderr)
	default:
		fmt.Fprintf(stderr, "feed: unknown command %q\n\n", rest[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `feed shows the GitHub activity relay's critical events in the terminal.

Usage:
  feed watch   [--relay <url>]
  feed history [--relay <url>]
  feed --version

The relay resolves as --relay, then $RELAY_URL, then %s.
`, defaultRelay)
}

// watch holds the relay's stream open. This ticket only proves the seam: the
// SSE parser and the renderer arrive with ticket 02, and reconnect behaviour
// with ticket 04.
func watch(args []string, stderr io.Writer) int {
	relay, ok, code := relayFor("watch", args, stderr)
	if !ok {
		return code
	}

	target := endpoint(relay, "/stream")
	fmt.Fprintf(stderr, "watching %s\n", target)

	resp, err := http.Get(target)
	if err != nil {
		fmt.Fprintf(stderr, "watch: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stderr, "watch: %s answered %s\n", target, resp.Status)
		return 1
	}

	// The frames are discarded until ticket 02 gives them a parser.
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		fmt.Fprintf(stderr, "watch: %v\n", err)
		return 1
	}
	return 0
}

// history fetches the audit history once. This ticket only proves the seam;
// ticket 07 gives the response its shape.
func history(args []string, stderr io.Writer) int {
	relay, ok, code := relayFor("history", args, stderr)
	if !ok {
		return code
	}

	target := endpoint(relay, "/history")
	resp, err := http.Get(target)
	if err != nil {
		fmt.Fprintf(stderr, "history: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stderr, "history: %s answered %s\n", target, resp.Status)
		return 1
	}

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		fmt.Fprintf(stderr, "history: %v\n", err)
		return 1
	}
	return 0
}

// relayFor resolves the relay as flag over environment over default. The flag's
// own default already carries the environment and the fallback, so a flag that
// is present wins without a second lookup.
func relayFor(name string, args []string, stderr io.Writer) (relay string, ok bool, code int) {
	fallback := os.Getenv("RELAY_URL")
	if fallback == "" {
		fallback = defaultRelay
	}

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintf(stderr, "usage: feed %s [--relay <url>]\n", name) }
	resolved := fs.String("relay", fallback, "relay base URL")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return "", false, 0
		}
		return "", false, 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "feed %s: unexpected argument %q\n\n", name, fs.Arg(0))
		fs.Usage()
		return "", false, 2
	}

	return strings.TrimRight(*resolved, "/"), true, 0
}

func endpoint(relay, path string) string {
	return relay + path
}

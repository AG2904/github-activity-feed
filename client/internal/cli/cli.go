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
	"time"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
	"github.com/AG2904/github-activity-feed/client/internal/render"
	"github.com/AG2904/github-activity-feed/client/internal/sse"
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
		return watch(rest[1:], stdout, stderr)
	case "history":
		return history(rest[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "feed: unknown command %q\n\n", rest[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `feed shows the GitHub activity relay's critical events in the terminal.

Usage:
  feed watch   [--relay <url>] [--no-color] [--json]
  feed history [--relay <url>] [--json | --group-by-repo]
  feed --version

The relay resolves as --relay, then $RELAY_URL, then %s.
`, defaultRelay)
}

// watch holds the relay's stream open and paints each frame as it arrives. A
// frame that does not parse is discarded, not fatal - the same stance the relay
// takes on a hook it does not understand. Reconnection arrives with ticket 04.
func watch(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("watch", stderr, "usage: feed watch [--relay <url>] [--no-color] [--json]")
	relay := fs.String("relay", relayFallback(), "relay base URL")
	noColor := fs.Bool("no-color", false, "disable colour and hyperlinks")
	jsonOut := fs.Bool("json", false, "write each event's payload verbatim")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}

	target := endpoint(strings.TrimRight(*relay, "/"), "/stream")
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

	palette := render.Palette{Color: colorEnabled(stdout, *noColor)}
	frames := sse.NewReader(resp.Body)
	for {
		payload, err := frames.Next()
		if errors.Is(err, io.EOF) {
			return 0
		}
		if err != nil {
			fmt.Fprintf(stderr, "watch: %v\n", err)
			return 1
		}

		if err := emit(stdout, palette, *jsonOut, payload); err != nil {
			fmt.Fprintf(stderr, "watch: %v\n", err)
			return 1
		}
	}
}

// emit validates one frame's payload and writes it in the selected mode. A
// payload that does not parse is discarded even with --json: raw output passes
// the relay's bytes through untouched, but it still only passes events.
func emit(stdout io.Writer, palette render.Palette, jsonOut bool, payload string) error {
	event, err := activity.Parse([]byte(payload))
	if err != nil {
		return nil
	}

	if jsonOut {
		// Re-marshalling would reorder keys and could reshape null or the
		// millisecond timestamp.
		_, err := io.WriteString(stdout, payload+"\n")
		return err
	}

	return palette.Event(stdout, event, time.Now())
}

// history fetches the audit history once. This ticket proves the seam and the
// raw mode; ticket 07 gives the rendered response its shape.
func history(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("history", stderr, "usage: feed history [--relay <url>] [--json | --group-by-repo]")
	relay := fs.String("relay", relayFallback(), "relay base URL")
	jsonOut := fs.Bool("json", false, "write the relay's response verbatim")
	groupByRepo := fs.Bool("group-by-repo", false, "group the history by repository")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}

	if *jsonOut && *groupByRepo {
		fmt.Fprintln(stderr, "history: --json and --group-by-repo cannot be combined: grouping is presentation, and raw output has none")
		return 2
	}

	target := endpoint(strings.TrimRight(*relay, "/"), "/history")
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

	if *jsonOut {
		if _, err := io.Copy(stdout, resp.Body); err != nil {
			fmt.Fprintf(stderr, "history: %v\n", err)
			return 1
		}
		return 0
	}

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		fmt.Fprintf(stderr, "history: %v\n", err)
		return 1
	}
	return 0
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

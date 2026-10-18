package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
	"github.com/AG2904/github-activity-feed/client/internal/render"
)

// history answers a retrospective question about the stored past: the audit
// history, once, printed and exited. It is the same feed as the live one — the
// same eight fields, the same two lines, the same hyperlink — because where an
// event came from differs and the thing does not (story 18).
func history(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("history", stderr, "usage: feed history [--relay <url>] [--user <login>] [--repo <owner/name>] [--no-color] [--json | --group-by-repo]")
	relay := fs.String("relay", relayFallback(), "relay base URL")
	user := fs.String("user", "", "only events where this login is the actor or the assignee")
	repository := fs.String("repo", "", "only events in this owner/name repository")
	noColor := fs.Bool("no-color", false, "disable colour and hyperlinks")
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
	resp, err := fetch(ctx, target)
	if err != nil {
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintf(stderr, "history: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	out := emitter{
		out:     stdout,
		palette: render.Palette{Color: colorEnabled(stdout, *noColor)},
		filter:  activity.Filter{User: *user, Repository: *repository},
	}

	// Raw output is the relay's own bytes, the whole array, and not a rendering
	// of them: that contract belongs to whoever is scripting against it. It is
	// also why the emitter below carries no --json: everything past this point
	// is the report.
	if *jsonOut {
		if _, err := io.Copy(stdout, resp.Body); err != nil {
			if ctx.Err() != nil {
				return 0
			}
			fmt.Fprintf(stderr, "history: %v\n", err)
			return 1
		}
		return 0
	}

	payloads, err := storedRows(resp.Body)
	if err != nil {
		fmt.Fprintf(stderr, "history: %v\n", err)
		return 1
	}
	shown := out.shown(payloads)

	// Grouping is presentation over a finite list, which is why it lives here
	// and not on the stream: a live feed is unbounded and unordered.
	if *groupByRepo {
		events := make([]activity.Event, 0, len(shown))
		for _, stored := range shown {
			events = append(events, stored.event)
		}
		if err := out.palette.Grouped(stdout, events, time.Now()); err != nil {
			fmt.Fprintf(stderr, "history: %v\n", err)
			return 1
		}
		return 0
	}

	// Newest first, the order the relay serves: a report is read from the top.
	for _, stored := range shown {
		if err := out.paint(stored.payload, stored.event); err != nil {
			fmt.Fprintf(stderr, "history: %v\n", err)
			return 1
		}
	}
	return 0
}

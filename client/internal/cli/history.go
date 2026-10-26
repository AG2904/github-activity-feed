package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
)

// history answers a retrospective question about the stored past: the audit
// history, once, printed and exited. It is the same feed as the live one — the
// same nine fields, the same card, the same hyperlink — because where an
// event came from differs and the thing does not (story 18).
func history(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("history", stderr, "usage: feed history [--relay <url>] [--user <login>] [--repo <owner/name>] [--no-color] [--json | --group-by-repo]")
	relay := fs.String("relay", relayFallback(), "relay base URL")
	user := fs.String("user", "", "only events where this login is the actor or the assignee")
	repository := fs.String("repo", "", "only events in this owner/name repository")
	noColor := fs.Bool("no-color", false, "disable colour and hyperlinks")
	jsonOut := fs.Bool("json", false, "write each stored event's bytes verbatim")
	groupByRepo := fs.Bool("group-by-repo", false, "group the history by repository")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}

	if *jsonOut && *groupByRepo {
		fmt.Fprintln(stderr, "history: --json and --group-by-repo cannot be combined: grouping is presentation, and raw output has none")
		return 2
	}

	target := endpoint(strings.TrimRight(*relay, "/"), "/history")
	payloads, err := storedHistory(ctx, target)
	if err != nil {
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintf(stderr, "history: %v\n", err)
		return 1
	}

	out := newEmitter(stdout, *noColor, *jsonOut, activity.Filter{User: *user, Repository: *repository})
	rows := out.shown(payloads)

	// Raw output is the relay's own bytes of every event this client shows,
	// framed as the array the relay served them in. Nothing is re-marshalled —
	// key order, null, and the millisecond timestamp arrive as they were — and a
	// filter still narrows it, because raw is a shape and not an exemption.
	if *jsonOut {
		if err := writeArray(stdout, rows); err != nil {
			fmt.Fprintf(stderr, "history: %v\n", err)
			return 1
		}
		return 0
	}

	// Grouping is presentation over a finite list, which is why it lives here
	// and not on the stream: a live feed is unbounded and unordered.
	if *groupByRepo {
		events := make([]activity.Event, 0, len(rows))
		for _, row := range rows {
			events = append(events, row.event)
		}
		if err := out.group(events, time.Now()); err != nil {
			fmt.Fprintf(stderr, "history: %v\n", err)
			return 1
		}
		return 0
	}

	// Newest first, the order the relay serves: a report is read from the top.
	for _, row := range rows {
		if err := out.paint(row.payload, row.event); err != nil {
			fmt.Fprintf(stderr, "history: %v\n", err)
			return 1
		}
	}
	return 0
}

// writeArray frames stored rows as the JSON array the relay served them in. The
// envelope is the CLI's — that is all it adds — and every byte inside it belongs
// to the relay.
func writeArray(w io.Writer, rows []storedEvent) error {
	payloads := make([]string, 0, len(rows))
	for _, row := range rows {
		payloads = append(payloads, row.payload)
	}
	_, err := io.WriteString(w, "["+strings.Join(payloads, ",")+"]")
	return err
}

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
	"github.com/AG2904/github-activity-feed/client/internal/sse"
)

// watch holds the relay's stream open and paints each frame as it arrives,
// reopening it whenever it drops or falls silent. A relay that is unreachable
// at startup is retried indefinitely: this is a watch, and dying because the
// relay was restarting is worse than waiting.
//
// The session also narrates itself on stderr: the banner, one line after the
// first connection, one line of account on exit. All three are plain text and
// none may reach stdout, so a silent terminal reads as "nothing happened" and
// not as "it hung", and the event feed stays a contract.
func watch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("watch", stderr, "usage: feed watch [--relay <url>] [--user <login>] [--repo <owner/name>] [--backfill <n>] [--no-color] [--json]")
	relay := fs.String("relay", relayFallback(), "relay base URL")
	user := fs.String("user", "", "only events where this login is the actor or the assignee")
	repository := fs.String("repo", "", "only events in this owner/name repository")
	backfillFlag := fs.Int("backfill", 10, "how many stored events the first connection prints (0 disables)")
	noColor := fs.Bool("no-color", false, "disable colour and hyperlinks")
	jsonOut := fs.Bool("json", false, "write each event's payload verbatim")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}

	base := strings.TrimRight(*relay, "/")
	target := endpoint(base, "/stream")
	historyURL := endpoint(base, "/history")
	// The banner is the first line of the session chrome: what this watch is
	// watching and with which filters, on stderr and plain, before anything
	// can fail. Filters come after the backfill count, each only when set.
	fmt.Fprintf(stderr, "watching %s · backfill %d", target, *backfillFlag)
	if *user != "" {
		fmt.Fprintf(stderr, " · user %s", *user)
	}
	if *repository != "" {
		fmt.Fprintf(stderr, " · repo %s", *repository)
	}
	fmt.Fprintln(stderr)

	out := newEmitter(stdout, *noColor, *jsonOut, activity.Filter{User: *user, Repository: *repository})
	// The stop line is deferred, not written at each return: there is more
	// than one way out of this function — Ctrl-C, an unusable URL, a closed
	// stdout — and the session's account must speak on all of them.
	defer out.summarize(stderr)
	backoff := &backoff{}
	// The welcome belongs to the first connection the relay accepts, not to
	// the first attempt: a relay that is unreachable or refuses at startup is
	// retried by design, so the attempt that finally lands is still the
	// session's first connection. Only a connection after an accepted one is a
	// recovery, and only then does the reconnect notice speak.
	accepted := false
	filled := false

	for {
		// The history is read before the stream opens, so an event that arrives
		// during the read is not printed twice — the reprint the relay's own
		// dedupe exists to prevent — at the cost of a hole the size of two
		// requests, which is the hole a reconnect leaves and `feed history`
		// recovers. filled stays false until a relay answers with a history, so
		// a CLI started before its relay still fills the terminal, and a history
		// that was read is never asked for twice.
		if !filled {
			var err error
			filled, err = backfill(ctx, historyURL, out, *backfillFlag)
			if err != nil {
				fmt.Fprintf(stderr, "watch: %v\n", err)
				return 1
			}
		}

		eventSeen, err := stream(ctx, target, out, func() {
			// The first accepted connection is the only moment "is it alive?"
			// has no other reply, so it answers in words: how much history
			// printed, or that there is none. Every later connection is a
			// recovery worth naming instead, so a reconnect never doubles as a
			// second welcome and the affirmative line is printed exactly once.
			switch {
			case accepted:
				fmt.Fprintln(stderr, "watch: reconnected")
			case out.total > 0:
				fmt.Fprintf(stderr, "connected · %d events from history\n", out.total)
			default:
				fmt.Fprintln(stderr, "connected · waiting for events")
			}
			accepted = true
		})
		if ctx.Err() != nil {
			return 0
		}
		if eventSeen {
			backoff.reset()
		}

		var fatal *fatalError
		if errors.As(err, &fatal) {
			fmt.Fprintf(stderr, "watch: %v\n", fatal.err)
			return 1
		}
		if err == nil {
			err = errors.New("connection closed")
		}

		delay := backoff.delay()
		fmt.Fprintf(stderr, "watch: reconnecting in %s: %v\n", delay, err)

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0
		case <-timer.C:
		}
	}
}

// stream holds one connection until it ends, the relay goes silent, or the
// context is cancelled. It reports whether a valid event arrived, which is
// what resets the backoff even when the filter discards it, and calls connected
// once the response is accepted.
func stream(ctx context.Context, target string, out *emitter, connected func()) (eventSeen bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return false, &fatalError{err}
	}
	resp, err := relayClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("%s answered %s", target, printable(resp.Status))
	}
	if connected != nil {
		connected()
	}

	reader := newIdleReader(resp.Body, idleTimeout())
	defer reader.stop()
	frames := sse.NewReader(reader)

	for {
		payload, err := frames.Next()
		if err != nil {
			switch {
			case reader.expired():
				return eventSeen, fmt.Errorf("no bytes for %s", reader.timeout)
			case ctx.Err() != nil:
				return eventSeen, ctx.Err()
			case errors.Is(err, io.EOF):
				return eventSeen, nil
			default:
				return eventSeen, err
			}
		}

		event, valid, shown := out.accept(payload)
		if valid {
			// A valid event proves the connection is healthy even when the
			// filter discards it, so it still resets the backoff.
			eventSeen = true
		}
		if !shown {
			continue
		}
		if err := out.paint(payload, event); err != nil {
			return eventSeen, &fatalError{err}
		}
	}
}

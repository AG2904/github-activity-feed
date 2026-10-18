package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
	"github.com/AG2904/github-activity-feed/client/internal/render"
	"github.com/AG2904/github-activity-feed/client/internal/sse"
)

const (
	defaultIdleTimeout = 5 * time.Minute
	minBackoff         = time.Second
	maxBackoff         = 30 * time.Second
)

// idleTimeoutSetting is empty in a normal build. The test harness injects a
// shorter duration through -ldflags, so the five-minute rule can be exercised
// without making every test wait five minutes.
var idleTimeoutSetting = ""

func idleTimeout() time.Duration {
	if idleTimeoutSetting == "" {
		return defaultIdleTimeout
	}
	timeout, err := time.ParseDuration(idleTimeoutSetting)
	if err != nil {
		return defaultIdleTimeout
	}
	return timeout
}

// fatalError marks a failure the CLI cannot retry: an unusable relay URL or a
// closed stdout. Connection failures are retryable by design; these are not.
type fatalError struct{ err error }

func (e *fatalError) Error() string { return e.err.Error() }
func (e *fatalError) Unwrap() error { return e.err }

// watch holds the relay's stream open and paints each frame as it arrives,
// reopening it whenever it drops or falls silent. A relay that is unreachable
// at startup is retried indefinitely: this is a watch, and dying because the
// relay was restarting is worse than waiting.
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
	fmt.Fprintf(stderr, "watching %s\n", target)

	out := newEmitter(stdout, *noColor, activity.Filter{User: *user, Repository: *repository})
	// The live feed is the one place --json changes how a single event is
	// written, so it is the one place the emitter carries that flag.
	out.json = *jsonOut
	backoff := &backoff{}
	attempts := 0
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
			// The first connection is the banner's job; every later one is a
			// recovery worth naming, so silence and a dead relay differ.
			if attempts > 0 {
				fmt.Fprintln(stderr, "watch: reconnected")
			}
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
		attempts++

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
func stream(ctx context.Context, target string, out emitter, connected func()) (eventSeen bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return false, &fatalError{err}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("%s answered %s", target, resp.Status)
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

// emitter is the output policy in one value: what this client shows, and how it
// is written. It never decides what the relay sends; only what reaches stdout.
type emitter struct {
	out     io.Writer
	palette render.Palette
	json    bool
	filter  activity.Filter
}

// newEmitter builds the output policy both commands hand their events to: the
// same stdout, the same colour rule, the same filter, so `--no-color` and
// `--user` cannot come to mean different things in the two of them.
func newEmitter(stdout io.Writer, noColor bool, filter activity.Filter) emitter {
	return emitter{
		out:     stdout,
		palette: render.Palette{Color: colorEnabled(stdout, noColor)},
		filter:  filter,
	}
}

// accept parses one payload and answers what it is to this client: valid when
// the CLI could read it as an event at all — an event the filter drops is still
// valid, and that is what resets the backoff — and shown when this client shows
// it. It is the one place the acceptance rule is written down, so the live
// stream and the backfill cannot disagree about it.
func (e emitter) accept(payload string) (event activity.Event, valid, shown bool) {
	event, err := activity.Parse([]byte(payload))
	if err != nil {
		return activity.Event{}, false, false
	}
	return event, true, e.filter.Match(event)
}

// paint writes one event of the feed. It is the only place the feed reaches
// stdout as feed lines, so the backfill and the live stream cannot disagree
// about --json, about colour, or about the rendered form. The report shapes —
// the raw array and the grouped report — are their own, because neither is a
// line of the feed.
func (e emitter) paint(payload string, event activity.Event) error {
	if e.json {
		// Re-marshalling would reorder keys and could reshape null or the
		// millisecond timestamp.
		_, err := io.WriteString(e.out, payload+"\n")
		return err
	}
	return e.palette.Event(e.out, event, time.Now())
}

// idleReader treats a read gap longer than the timeout as death. The relay
// heartbeats every fifteen seconds, so a healthy stream resets the timer long
// before five minutes; the timer is a backstop for a connection that dies
// inside a heartbeat interval, or a relay that stops writing without closing
// the socket.
type idleReader struct {
	r       io.Reader
	timeout time.Duration
	timer   *time.Timer
	stale   atomic.Bool
}

func newIdleReader(r io.Reader, timeout time.Duration) *idleReader {
	reader := &idleReader{r: r, timeout: timeout}
	reader.timer = time.AfterFunc(timeout, func() {
		reader.stale.Store(true)
		if closer, ok := r.(io.Closer); ok {
			_ = closer.Close()
		}
	})
	return reader
}

func (r *idleReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		r.stale.Store(false)
		r.timer.Reset(r.timeout)
	}
	return n, err
}

func (r *idleReader) expired() bool {
	return r.stale.Load()
}

func (r *idleReader) stop() {
	r.timer.Stop()
}

// backoff grows exponentially from one second to a thirty-second ceiling. Each
// delay is jittered upward by up to a quarter of the step, so a relay restart
// does not gather every client in the same instant. The ceiling applies to the
// step; a reset returns to the one-second base.
type backoff struct {
	step time.Duration
}

func (b *backoff) reset() {
	b.step = 0
}

func (b *backoff) delay() time.Duration {
	step := b.step
	if step == 0 {
		step = minBackoff
	}

	delay := step + time.Duration(rand.Int64N(int64(step/4)))
	if delay > maxBackoff {
		delay = maxBackoff
	}

	if next := step * 2; next < maxBackoff {
		b.step = next
	} else {
		b.step = maxBackoff
	}
	return delay
}

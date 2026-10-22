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

	out := newEmitter(stdout, *noColor, activity.Filter{User: *user, Repository: *repository})
	// The live feed is the one place --json changes how a single event is
	// written, so it is the one place the emitter carries that flag.
	out.json = *jsonOut
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
// It also remembers whether a card was already printed, because the blank line
// between cards belongs to the sequence and not to one card.
type emitter struct {
	out     io.Writer
	palette render.Palette
	json    bool
	filter  activity.Filter
	printed bool
	// The session's account of itself: what reached the reader, filter
	// included, split by the two types that carry a human word. The backfill
	// reads total as how much history printed; the stop line reads all three.
	total    int
	merged   int
	assigned int
}

// newEmitter builds the output policy both commands hand their events to: the
// same stdout, the same colour rule, the same filter, so `--no-color` and
// `--user` cannot come to mean different things in the two of them.
func newEmitter(stdout io.Writer, noColor bool, filter activity.Filter) *emitter {
	return &emitter{
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
func (e *emitter) accept(payload string) (event activity.Event, valid, shown bool) {
	event, err := activity.Parse([]byte(payload))
	if err != nil {
		return activity.Event{}, false, false
	}
	return event, true, e.filter.Match(event)
}

// paint writes one event of the feed. It is the only place the feed reaches
// stdout as feed lines, so the backfill and the live stream cannot disagree
// about --json, about colour, or about the rendered form. The blank line that
// separates cards is written here, before a card that is not the first, so the
// last event never leaves a gap under the prompt and `--json` never gets one.
func (e *emitter) paint(payload string, event activity.Event) error {
	if e.json {
		// Re-marshalling would reorder keys and could reshape null or the
		// millisecond timestamp.
		if _, err := io.WriteString(e.out, payload+"\n"); err != nil {
			return err
		}
		e.count(event)
		return nil
	}
	if e.printed {
		if _, err := io.WriteString(e.out, "\n"); err != nil {
			return err
		}
	}
	if err := e.palette.Event(e.out, event, time.Now()); err != nil {
		return err
	}
	e.count(event)
	e.printed = true
	return nil
}

// count records one event that reached the reader. Counting inside paint — the
// one place the backfill, the live stream and --json all cross into stdout —
// keeps a single account of what was shown; an event whose write failed is not
// counted, because nothing was shown of it.
func (e *emitter) count(event activity.Event) {
	e.total++
	switch event.Type {
	case activity.TypePRMerged:
		e.merged++
	case activity.TypeIssueAssigned:
		e.assigned++
	}
}

// summarize closes the session on stderr with one line of account: the events
// the reader saw, filter included, because the summary answers "what did I
// see", not "what did the relay send". watch() defers it, so every path out of
// the command — Ctrl-C, an unusable URL, a closed stdout — speaks once before
// going. It is plain text like the rest of the chrome: one sentence of state,
// no escape, never a heartbeat.
func (e *emitter) summarize(w io.Writer) {
	if e.total == 0 {
		fmt.Fprintln(w, "stopped · no events")
		return
	}
	fmt.Fprintf(w, "stopped · %d events · %d merged · %d assigned\n", e.total, e.merged, e.assigned)
}

// group writes the report shape: the same events, one section per repository.
// Raw output has no grouped form — the flags refuse the combination — so there
// is nothing to decide here, only to hand over, the same way paint hands over a
// feed line. Every byte the CLI puts on stdout starts here.
func (e *emitter) group(events []activity.Event, now time.Time) error {
	return e.palette.Grouped(e.out, events, now)
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

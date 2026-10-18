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
	fs := newFlagSet("watch", stderr, "usage: feed watch [--relay <url>] [--user <login>] [--repo <owner/name>] [--no-color] [--json]")
	relay := fs.String("relay", relayFallback(), "relay base URL")
	user := fs.String("user", "", "only events where this login is the actor or the assignee")
	repository := fs.String("repo", "", "only events in this owner/name repository")
	noColor := fs.Bool("no-color", false, "disable colour and hyperlinks")
	jsonOut := fs.Bool("json", false, "write each event's payload verbatim")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}

	target := endpoint(strings.TrimRight(*relay, "/"), "/stream")
	fmt.Fprintf(stderr, "watching %s\n", target)

	out := emitter{
		out:     stdout,
		palette: render.Palette{Color: colorEnabled(stdout, *noColor)},
		json:    *jsonOut,
		filter:  activity.Filter{User: *user, Repository: *repository},
	}
	backoff := &backoff{}
	attempts := 0

	for {
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

		valid, err := out.emit(payload)
		if err != nil {
			return eventSeen, &fatalError{err}
		}
		if valid {
			// A valid event proves the connection is healthy even when the
			// filter discards it, so it still resets the backoff.
			eventSeen = true
		}
	}
}

// emitter is the output policy in one value: rendered events, raw payloads, or
// nothing at all when the frame is not an event.
type emitter struct {
	out     io.Writer
	palette render.Palette
	json    bool
	filter  activity.Filter
}

// emit reports whether the payload was a valid event, not whether it was
// written: a filtered-out event still proves the connection is alive.
func (e emitter) emit(payload string) (valid bool, err error) {
	event, err := activity.Parse([]byte(payload))
	if err != nil {
		return false, nil
	}
	if !e.filter.Match(event) {
		return true, nil
	}

	if e.json {
		// Re-marshalling would reorder keys and could reshape null or the
		// millisecond timestamp.
		if _, err := io.WriteString(e.out, payload+"\n"); err != nil {
			return true, err
		}
		return true, nil
	}

	if err := e.palette.Event(e.out, event, time.Now()); err != nil {
		return true, err
	}
	return true, nil
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

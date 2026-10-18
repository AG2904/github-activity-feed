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

// watch holds the relay's stream open and paints each frame as it arrives,
// reopening it whenever it drops or falls silent. A relay that is unreachable
// at startup is retried indefinitely: this is a watch, and dying because the
// relay was restarting is worse than waiting.
func watch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("watch", stderr, "usage: feed watch [--relay <url>] [--no-color] [--json]")
	relay := fs.String("relay", relayFallback(), "relay base URL")
	noColor := fs.Bool("no-color", false, "disable colour and hyperlinks")
	jsonOut := fs.Bool("json", false, "write each event's payload verbatim")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}

	target := endpoint(strings.TrimRight(*relay, "/"), "/stream")
	fmt.Fprintf(stderr, "watching %s\n", target)

	palette := render.Palette{Color: colorEnabled(stdout, *noColor)}
	backoff := &backoff{}

	for {
		sawEvent, err := stream(ctx, target, stdout, palette, *jsonOut)
		if ctx.Err() != nil {
			return 0
		}
		if sawEvent {
			backoff.reset()
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
// context is cancelled. It reports whether an event reached stdout, which is
// what resets the backoff.
func stream(ctx context.Context, target string, stdout io.Writer, palette render.Palette, jsonOut bool) (sawEvent bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("%s answered %s", target, resp.Status)
	}

	reader := newIdleReader(resp.Body, idleTimeout())
	defer reader.stop()
	frames := sse.NewReader(reader)

	for {
		payload, err := frames.Next()
		if err != nil {
			switch {
			case reader.expired():
				return sawEvent, fmt.Errorf("no bytes for %s", reader.timeout)
			case ctx.Err() != nil:
				return sawEvent, ctx.Err()
			case errors.Is(err, io.EOF):
				return sawEvent, nil
			default:
				return sawEvent, err
			}
		}

		wrote, err := emit(stdout, palette, jsonOut, payload)
		if err != nil {
			return sawEvent, err
		}
		if wrote {
			sawEvent = true
		}
	}
}

// emit validates one frame's payload and writes it in the selected mode. A
// payload that does not parse is discarded even with --json: raw output passes
// the relay's bytes through untouched, but it still only passes events.
func emit(stdout io.Writer, palette render.Palette, jsonOut bool, payload string) (wrote bool, err error) {
	event, err := activity.Parse([]byte(payload))
	if err != nil {
		return false, nil
	}

	if jsonOut {
		// Re-marshalling would reorder keys and could reshape null or the
		// millisecond timestamp.
		if _, err := io.WriteString(stdout, payload+"\n"); err != nil {
			return false, err
		}
		return true, nil
	}

	if err := palette.Event(stdout, event, time.Now()); err != nil {
		return false, err
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
// delay is jittered in [d/2, d), so a relay restart does not gather every
// client in the same instant. The ceiling applies to the step; a reset returns
// to the one-second base.
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

	half := step / 2
	jittered := half + time.Duration(rand.Int64N(int64(half)))

	if next := step * 2; next < maxBackoff {
		b.step = next
	} else {
		b.step = maxBackoff
	}
	return jittered
}

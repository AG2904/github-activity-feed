package test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRelay is the test harness's counterpart at the process seam: a real HTTP
// server that answers the relay's two read endpoints, holds the stream open,
// and records what the binary asked for. A test writes frames into the stream
// after the CLI connected, and can drop the connection to exercise reconnect.
type fakeRelay struct {
	*httptest.Server

	mu             sync.Mutex
	request        []string
	lastEventID    []string
	frames         chan string
	drop           chan struct{}
	historyBody    string
	historyFailing bool
	done           chan struct{}
	doneOnce       sync.Once

	connected chan struct{}
}

func newFakeRelay(t *testing.T) *fakeRelay {
	t.Helper()

	relay := &fakeRelay{
		frames:      make(chan string, 16),
		drop:        make(chan struct{}),
		historyBody: "[]\n",
		done:        make(chan struct{}),
		connected:   make(chan struct{}),
	}
	relay.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		relay.record(r.URL.RequestURI())
		switch r.URL.Path {
		case "/stream":
			relay.recordEventID(r.Header.Get("Last-Event-ID"))
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming unsupported", http.StatusInternalServerError)
				return
			}
			flusher.Flush()
			// An unbuffered handshake, so a signal always belongs to this
			// connection and never to an earlier one.
			select {
			case relay.connected <- struct{}{}:
			case <-r.Context().Done():
				return
			}

			for {
				select {
				case frame := <-relay.frames:
					if _, err := io.WriteString(w, frame); err != nil {
						return
					}
					flusher.Flush()
				case <-relay.drop:
					return
				case <-relay.done:
					return
				case <-r.Context().Done():
					return
				}
			}
		case "/history":
			if relay.historyFails() {
				http.Error(w, "the relay is having a bad day", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(relay.history()))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(func() {
		relay.doneOnce.Do(func() { close(relay.done) })
		relay.Close()
	})
	return relay
}

// record keeps the whole request line, not just the path, so a query string the
// CLI should not be sending is visible to a test instead of hidden behind it.
func (f *fakeRelay) record(requestURI string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.request = append(f.request, requestURI)
}

// requests is a snapshot, so a test can assert both what the relay saw and
// what it did not see.
func (f *fakeRelay) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.request...)
}

func (f *fakeRelay) recordEventID(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastEventID = append(f.lastEventID, id)
}

// lastEventIDs is what each stream request offered as a resume position. The
// relay's `id:` counter restarts at zero, so the client must offer nothing.
func (f *fakeRelay) lastEventIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lastEventID...)
}

func (f *fakeRelay) setHistory(body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.historyBody = body
}

func (f *fakeRelay) history() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.historyBody
}

// failHistory makes /history answer 500, the way a relay that is up but unhappy
// would, until it is called again with false.
func (f *fakeRelay) failHistory(failing bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.historyFailing = failing
}

func (f *fakeRelay) historyFails() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.historyFailing
}

func (f *fakeRelay) hits(path string) int {
	hits := 0
	for _, seen := range f.requests() {
		if seen == path {
			hits++
		}
	}
	return hits
}

// wantHits asserts how many times the relay saw one request line. The count is
// never the point on its own, so every call says what it means.
func (f *fakeRelay) wantHits(t *testing.T, request string, want int, meaning string) {
	t.Helper()
	if got := f.hits(request); got != want {
		t.Errorf("relay saw %d %s requests, want %d: %s", got, request, want, meaning)
	}
}

// waitConnected blocks until the CLI's stream request has reached the handler,
// which is the deterministic barrier SSE needs: bytes on the wire mean the
// subscription exists. It is called once per connection, so reconnects consume
// another signal.
func (f *fakeRelay) waitConnected(t *testing.T) {
	t.Helper()
	select {
	case <-f.connected:
	case <-time.After(10 * time.Second):
		t.Fatalf("the CLI never connected to /stream")
	}
}

// send writes one frame into the held-open stream.
func (f *fakeRelay) send(t *testing.T, frame string) {
	t.Helper()
	select {
	case f.frames <- frame:
	case <-time.After(5 * time.Second):
		t.Fatalf("the stream never accepted a frame")
	}
}

// dropStream closes the current stream from the server side, which is what a
// relay restart or a killed connection looks like to the client.
func (f *fakeRelay) dropStream(t *testing.T) {
	t.Helper()
	select {
	case f.drop <- struct{}{}:
	case <-time.After(5 * time.Second):
		t.Fatalf("there was no open stream to drop")
	}
}

type result struct {
	stdout string
	stderr string
	code   int
}

// runFeed is for the one-shot commands: start the binary, wait for it, and
// keep everything a developer (or a script) could observe.
func runFeed(t *testing.T, env map[string]string, args ...string) result {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, feedBinary, args...)
	cmd.Env = environment(env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("feed %s did not finish: %v", strings.Join(args, " "), ctx.Err())
	}

	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("running feed %s: %v", strings.Join(args, " "), err)
		}
		code = exit.ExitCode()
	}

	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// runningFeed is the held-open counterpart: `watch` does not exit on its own,
// so a test starts it, observes it, and lets cleanup stop it.
type runningFeed struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	stdout *syncBuffer
	stderr *syncBuffer

	mu      sync.Mutex
	stopped bool
}

func startFeed(t *testing.T, env map[string]string, args ...string) *runningFeed {
	t.Helper()
	return startBinary(t, feedBinary, env, args...)
}

func startBinary(t *testing.T, binary string, env map[string]string, args ...string) *runningFeed {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = environment(env)
	feed := &runningFeed{cmd: cmd, cancel: cancel, stdout: &syncBuffer{}, stderr: &syncBuffer{}}
	cmd.Stdout = feed.stdout
	cmd.Stderr = feed.stderr

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("starting feed %s: %v", strings.Join(args, " "), err)
	}
	t.Cleanup(feed.stop)
	return feed
}

func (f *runningFeed) stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped {
		return
	}
	f.stopped = true
	f.cancel()
	_ = f.cmd.Wait()
}

func (f *runningFeed) waitStdout(t *testing.T, want string) {
	t.Helper()
	f.waitFor(t, "stdout", want, f.stdout.String)
}

func (f *runningFeed) waitStderr(t *testing.T, want string) {
	t.Helper()
	f.waitFor(t, "stderr", want, f.stderr.String)
}

func (f *runningFeed) waitFor(t *testing.T, stream, want string, read func() string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(read(), want) {
		if time.Now().After(deadline) {
			t.Fatalf("%s never contained %q\nstdout: %q\nstderr: %q", stream, want, f.stdout.String(), f.stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// reconnectPattern pulls the delay out of each reconnect notice. The line is
// the only thing an attempt writes, so its count is also the attempt count.
var reconnectPattern = regexp.MustCompile(`watch: reconnecting in ([0-9.]+(?:ns|µs|ms|s))`)

func (f *runningFeed) waitReconnects(t *testing.T, want int) []time.Duration {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		delays := parseReconnects(f.stderr.String())
		if len(delays) >= want {
			return delays
		}
		if time.Now().After(deadline) {
			t.Fatalf("wanted %d reconnect notices, got %d\nstderr: %q", want, len(delays), f.stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func nonEmptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseReconnects(stderr string) []time.Duration {
	matches := reconnectPattern.FindAllStringSubmatch(stderr, -1)
	delays := make([]time.Duration, 0, len(matches))
	for _, match := range matches {
		delay, err := time.ParseDuration(match[1])
		if err != nil {
			continue
		}
		delays = append(delays, delay)
	}
	return delays
}

// firstWatchBanner starts `feed watch` with no relay override and returns the
// banner it writes before connecting. Reading the banner is how the default
// relay is observed without depending on what is listening on port 3000.
func firstWatchBanner(t *testing.T) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, feedBinary, "watch")
	cmd.Env = environment(nil)
	var stderr syncBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting feed watch: %v", err)
	}

	const want = "watching http://localhost:3000/stream"
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(stderr.String(), want) {
		if time.Now().After(deadline) {
			cancel()
			_ = cmd.Wait()
			t.Fatalf("banner %q never appeared; stderr was %q", want, stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	_ = cmd.Wait()
	return stderr.String()
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// environment starts from the real environment but removes RELAY_URL, so a
// developer's own setting cannot leak into a test, then applies the overrides.
func environment(overrides map[string]string) []string {
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		name := entry
		if i := strings.IndexByte(entry, '='); i >= 0 {
			name = entry[:i]
		}
		if strings.EqualFold(name, "RELAY_URL") {
			continue
		}
		env = append(env, entry)
	}
	for name, value := range overrides {
		env = append(env, name+"="+value)
	}
	return env
}

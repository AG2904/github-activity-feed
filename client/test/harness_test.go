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
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRelay is the test harness's counterpart at the process seam: a real HTTP
// server that answers the relay's two read endpoints, holds the stream open,
// and records what the binary asked for. A test writes frames into the stream
// after the CLI connected, so the stream is exercised rather than inspected.
type fakeRelay struct {
	*httptest.Server

	mu          sync.Mutex
	request     []string
	frames      chan string
	historyBody string
	done        chan struct{}
	doneOnce    sync.Once

	connected     chan struct{}
	connectedOnce sync.Once
}

func newFakeRelay(t *testing.T) *fakeRelay {
	t.Helper()

	relay := &fakeRelay{
		frames:      make(chan string, 16),
		historyBody: "[]\n",
		done:        make(chan struct{}),
		connected:   make(chan struct{}),
	}
	relay.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		relay.record(r.URL.Path)
		switch r.URL.Path {
		case "/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming unsupported", http.StatusInternalServerError)
				return
			}
			flusher.Flush()
			relay.connectedOnce.Do(func() { close(relay.connected) })

			for {
				select {
				case frame := <-relay.frames:
					if _, err := io.WriteString(w, frame); err != nil {
						return
					}
					flusher.Flush()
				case <-relay.done:
					return
				case <-r.Context().Done():
					return
				}
			}
		case "/history":
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

func (f *fakeRelay) record(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.request = append(f.request, path)
}

// requests is a snapshot, so a test can assert both what the relay saw and
// what it did not see.
func (f *fakeRelay) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.request...)
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

func (f *fakeRelay) hits(path string) int {
	hits := 0
	for _, seen := range f.requests() {
		if seen == path {
			hits++
		}
	}
	return hits
}

// waitConnected blocks until the CLI's stream request has reached the handler,
// which is the deterministic barrier SSE needs: bytes on the wire mean the
// subscription exists.
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

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, feedBinary, args...)
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

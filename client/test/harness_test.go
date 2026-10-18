package test

import (
	"bytes"
	"context"
	"errors"
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
// server that answers the relay's two read endpoints and records what the
// binary asked for. Later tickets replace the canned responses with a real
// stream; the harness itself is the deliverable this ticket stands up.
type fakeRelay struct {
	*httptest.Server

	mu      sync.Mutex
	request []string
}

func newFakeRelay(t *testing.T) *fakeRelay {
	t.Helper()

	relay := &fakeRelay{}
	relay.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		relay.record(r.URL.Path)
		switch r.URL.Path {
		case "/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(
				`data: {"type":"pr.merged","repository":"AG2904/github-activity-feed","actor":"maria.perez","subject":"Fix memory leak in parser component","assignee":null,"labels":[],"url":"https://github.com/AG2904/github-activity-feed/pull/42","when":"2026-10-11T13:00:00.000Z"}` + "\n\n",
			))
		case "/history":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(relay.Close)
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

func (f *fakeRelay) hits(path string) int {
	hits := 0
	for _, seen := range f.requests() {
		if seen == path {
			hits++
		}
	}
	return hits
}

type result struct {
	stdout string
	stderr string
	code   int
}

// runFeed crosses the process seam: start the binary, wait for it, and keep
// everything a developer (or a script) could observe.
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

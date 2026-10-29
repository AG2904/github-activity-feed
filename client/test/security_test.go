package test

// Security contract tests for the three boundaries an audit confirmed at
// 324dbf5. Each test drives the built binary through the same harness as the
// rest of the suite and asserts the invariant a fix must enforce, not merely
// that the program still runs.

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// hostilePayload is a stored row whose text carries terminal control bytes the
// relay preserved verbatim. The JSON uses the escape forms because a raw ESC is
// not legal inside a JSON string; decoding restores the real bytes, which is
// the same path the renderer and the --json passthrough take.
const hostilePayload = `{"type":"pr.merged","repository":"AG2904/widgets","branch":"fix/tidy\u001b[2K\r\u001b[1A","actor":"mallory","subject":"Tidy \u001b]8;;https://evil.example/\u001b\\ now","assignee":null,"labels":["bug"],"url":"https://github.com/AG2904/widgets/pull/7","when":"2026-10-11T13:00:00.000Z"}`

// hostileRepoPayload carries the control bytes in the repository name instead,
// which is the one field the grouped report writes in its own header line.
const hostileRepoPayload = `{"type":"pr.merged","repository":"AG2904/widgets\u001b[2J","branch":"fix/tidy","actor":"mallory","subject":"Tidy the docs","assignee":null,"labels":["bug"],"url":"https://github.com/AG2904/widgets/pull/8","when":"2026-10-11T13:00:00.000Z"}`

// TestHistoryRendersRelayControlBytesInvisible pins the renderer's contract:
// the colour gate decides which escapes the program adds, never which the
// event brings, so a relay-supplied field must not reach the terminal as a
// control byte on the piped path either.
func TestHistoryRendersRelayControlBytesInvisible(t *testing.T) {
	relay := newFakeRelay(t)
	relay.setHistory(historyOf(hostilePayload))

	got := runFeed(t, nil, "history", "--relay", relay.URL, "--no-color")
	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	if got.stdout == "" {
		t.Fatal("stdout is empty, want the rendered card")
	}
	if bad := controlBytes(got.stdout); bad != "" {
		t.Errorf("stdout carries %s from the relay, want it rendered invisible\nstdout: %q", bad, got.stdout)
	}
	// The fix must drop the effect, not the text: the card still reads.
	for _, want := range []string{"mallory", "Tidy", "AG2904/widgets"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout = %q, want %q still present after cleaning", got.stdout, want)
		}
	}
}

// TestGroupedReportRendersRelayControlBytesInvisible is the same contract on
// the one writer that is not a card: the group header.
func TestGroupedReportRendersRelayControlBytesInvisible(t *testing.T) {
	relay := newFakeRelay(t)
	relay.setHistory(historyOf(hostileRepoPayload))

	got := runFeed(t, nil, "history", "--relay", relay.URL, "--group-by-repo")
	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	if got.stdout == "" {
		t.Fatal("stdout is empty, want the grouped report")
	}
	if bad := controlBytes(got.stdout); bad != "" {
		t.Errorf("stdout carries %s from the relay, want it rendered invisible\nstdout: %q", bad, got.stdout)
	}
	if !strings.Contains(got.stdout, "AG2904/widgets") {
		t.Errorf("stdout = %q, want the repository name still readable", got.stdout)
	}
}

// TestHistoryDoesNotFollowRelayRedirects pins the client's read boundary: the
// only origin it may request from is the one the operator configured, because
// the redirect target is chosen by the relay and the relay URL is chosen by
// the operator.
func TestHistoryDoesNotFollowRelayRedirects(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "[]")
	}))
	t.Cleanup(target.Close)

	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/internal-admin?reboot=1", http.StatusFound)
	}))
	t.Cleanup(relay.Close)

	got := runFeed(t, nil, "history", "--relay", relay.URL)
	if got.code == 0 {
		t.Errorf("exit code = 0, want the redirect refused as a non-200 (stderr: %q)", got.stderr)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("the client requested the relay-chosen target %d time(s), want 0 — it must read only the configured relay", n)
	}
	if !strings.Contains(got.stderr, "answered 302") {
		t.Errorf("stderr = %q, want the refusal to name the status it refused", got.stderr)
	}
}

// TestErrorsCarryNoControlBytesFromTheStatusLine pins the stderr contract: the
// status line is relay-authored text, and docs/cli-flow.md promises stderr
// carries narration and errors as plain text with no TTY gating.
func TestErrorsCarryNoControlBytesFromTheStatusLine(t *testing.T) {
	relay := rawRelay(t, "HTTP/1.1 500 \x1b]0;pwned\x07status\x1b[2J\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")

	t.Run("history", func(t *testing.T) {
		got := runFeed(t, nil, "history", "--relay", relay)
		if got.code == 0 {
			t.Errorf("exit code = 0, want the refusal (stderr: %q)", got.stderr)
		}
		if bad := controlBytes(got.stderr); bad != "" {
			t.Errorf("stderr carries %s from the status line: %q", bad, got.stderr)
		}
		if !strings.Contains(got.stderr, "500") {
			t.Errorf("stderr = %q, want the status code to survive the cleaning", got.stderr)
		}
	})

	t.Run("watch", func(t *testing.T) {
		feed := startFeed(t, nil, "watch", "--relay", relay)
		feed.waitStderr(t, "watch: reconnecting in")
		if bad := controlBytes(feed.stderr.String()); bad != "" {
			t.Errorf("stderr carries %s from the status line: %q", bad, feed.stderr.String())
		}
		if !strings.Contains(feed.stderr.String(), "500") {
			t.Errorf("stderr = %q, want the status code to survive the cleaning", feed.stderr.String())
		}
	})
}

// controlBytes reports the first byte a terminal would act on: anything below
// space except the line break the card itself writes, DEL, and the C1 range.
// Control bytes are reported as the rune they form, so a message reads the same
// for a UTF-8 lead pair as for an ASCII byte.
func controlBytes(text string) string {
	for _, r := range text {
		if (r < 0x20 && r != '\n') || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return fmt.Sprintf("U+%04X", r)
		}
	}
	return ""
}

// rawRelay answers with a literal response that a net/http handler cannot
// produce, keeping the status line's reason phrase exactly as written. It
// consumes a request first so the client sees a complete exchange rather than a
// reset, then writes the response and closes.
func rawRelay(t *testing.T, response string) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				reader := bufio.NewReader(c)
				for {
					line, err := reader.ReadString('\n')
					if err != nil || strings.TrimRight(line, "\r\n") == "" {
						break
					}
				}
				_, _ = c.Write([]byte(response))
			}(conn)
		}
	}()

	return "http://" + listener.Addr().String()
}

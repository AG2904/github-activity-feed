package notify

// Unit tests for the mode resolution and the sink: what a requested mode turns
// into for a terminal, and what the sink writes when an event is shown. The
// console write itself is one call whose failure is ignored by design, so the
// decision is pinned here and never through a terminal.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
)

// TestResolveTurnsAutoIntoWhatTheTerminalCanDo pins the auto selection as a
// pure function of the injected lookup: a terminal known to speak a frame gets
// one, anything else gets the bell. No case depends on the machine it runs on.
func TestResolveTurnsAutoIntoWhatTheTerminalCanDo(t *testing.T) {
	kitty := map[string]string{"KITTY_WINDOW_ID": "3"}
	unknown := map[string]string{}

	tests := []struct {
		name         string
		mode         Mode
		env          map[string]string
		wantMode     Mode
		wantProtocol Protocol
	}{
		{name: "auto frames on kitty", mode: Auto, env: kitty, wantMode: OSC, wantProtocol: OSC99},
		{name: "auto rings the bell on an unknown terminal", mode: Auto, env: unknown, wantMode: Bell, wantProtocol: NoProtocol},
		{name: "osc keeps the detected protocol", mode: OSC, env: kitty, wantMode: OSC, wantProtocol: OSC99},
		{name: "osc with no detected protocol stays silent", mode: OSC, env: unknown, wantMode: OSC, wantProtocol: NoProtocol},
		{name: "bell never detects", mode: Bell, env: kitty, wantMode: Bell, wantProtocol: NoProtocol},
		{name: "off stays off", mode: Off, env: kitty, wantMode: Off, wantProtocol: NoProtocol},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(name string) string { return tt.env[name] }
			mode, protocol := resolve(tt.mode, lookup)
			if mode != tt.wantMode || protocol != tt.wantProtocol {
				t.Fatalf("resolve() = (%v, %v), want (%v, %v)", mode, protocol, tt.wantMode, tt.wantProtocol)
			}
		})
	}
}

// TestSinkWritesTheResolvedSignal pins the whole decision as one value: the
// requested mode, the environment and the writer are all the sink knows, and
// what lands in the writer follows from them.
func TestSinkWritesTheResolvedSignal(t *testing.T) {
	event := activity.Event{
		Type:       activity.TypePRMerged,
		Actor:      "maria.perez",
		Branch:     ptrTo("fix/memory-leak"),
		Subject:    "Tidy the parser",
		Repository: ptrTo("AG2904/github-activity-feed"),
	}
	kitty := map[string]string{"KITTY_WINDOW_ID": "3"}
	unknown := map[string]string{}

	tests := []struct {
		name  string
		mode  Mode
		env   map[string]string
		want  string
		exact bool
	}{
		{name: "off writes nothing", mode: Off, env: kitty, want: "", exact: true},
		{name: "bell writes the bell", mode: Bell, env: unknown, want: "\a", exact: true},
		{name: "osc writes the kitty frame", mode: OSC, env: kitty, want: "\x1b]99;;maria.perez merged fix/memory-leak into main"},
		{name: "osc falls back to the widest sequence", mode: OSC, env: unknown, want: "\x1b]9;maria.perez merged fix/memory-leak into main"},
		{name: "auto writes the frame the terminal speaks", mode: Auto, env: kitty, want: "\x1b]99;;maria.perez merged fix/memory-leak into main"},
		{name: "auto falls back to the bell", mode: Auto, env: unknown, want: "\a", exact: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			lookup := func(name string) string { return tt.env[name] }
			New(tt.mode, &out, true, lookup).Notify(event)

			got := out.String()
			if tt.exact && got != tt.want {
				t.Fatalf("written = %q, want %q", got, tt.want)
			}
			if !tt.exact && !strings.HasPrefix(got, tt.want) {
				t.Fatalf("written = %q, want it to start with %q", got, tt.want)
			}
			if !tt.exact && strings.ContainsRune(got, '\a') != strings.HasSuffix(got, "\a") {
				t.Fatalf("written = %q, want exactly one terminator at the end", got)
			}
		})
	}
}

// TestSinkNeverWritesAFrameWhereEscapesAreNotUnderstood pins the console
// capability: a console that would print the sequence literally gets no frame,
// while the bell — a character, not an escape — still works. Auto falls back to
// the bell because a signal it can deliver beats one it cannot.
func TestSinkNeverWritesAFrameWhereEscapesAreNotUnderstood(t *testing.T) {
	event := activity.Event{Type: activity.TypeIssueClosed, Actor: "lucia.gomez", Subject: "done"}
	kitty := map[string]string{"KITTY_WINDOW_ID": "3"}

	tests := []struct {
		name string
		mode Mode
		want string
	}{
		{name: "osc writes nothing", mode: OSC, want: ""},
		{name: "auto falls back to the bell", mode: Auto, want: "\a"},
		{name: "bell still rings", mode: Bell, want: "\a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			lookup := func(name string) string { return kitty[name] }
			New(tt.mode, &out, false, lookup).Notify(event)

			if got := out.String(); got != tt.want {
				t.Fatalf("written = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSinkSwallowsAFailedWrite pins the ADR's degradation: a signal that cannot
// be delivered never becomes the session's error, so Notify must not panic and
// must not report anything.
func TestSinkSwallowsAFailedWrite(_ *testing.T) {
	event := activity.Event{Type: activity.TypeIssueClosed, Actor: "lucia.gomez", Subject: "done"}
	New(Bell, failingWriter{}, true, func(string) string { return "" }).Notify(event)
}

// failingWriter rejects every write, standing in for a console that is gone.
type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, errWriteFailed
}

// errWriteFailed is the failure the failingWriter reports.
var errWriteFailed = &writeError{}

// writeError is a minimal error for the failing writer.
type writeError struct{}

func (*writeError) Error() string { return "write failed" }

package test

import (
	"strings"
	"testing"
	"time"
)

// The accepted set is closed: a typo is a usage error, not a watch that holds
// the terminal while raising nothing. The message names the four values, so the
// refusal teaches the flag it refused (docs/cli-testing.md, paso 2).
func TestWatchRefusesAnUnknownNotifyValue(t *testing.T) {
	got := runFeed(t, nil, "watch", "--notify", "loud")

	if got.code != 2 {
		t.Fatalf("exit code = %d, want 2 (stderr: %q)", got.code, got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
	for _, want := range []string{
		`feed watch: --notify "loud" is not one of off, bell, osc or auto`,
		"usage: feed watch",
		"--notify <off|bell|osc|auto>",
	} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("stderr = %q, want it to mention %q", got.stderr, want)
		}
	}
}

// Every value the flag accepts is a value the usage names, and each one parses
// before anything else can fail. The relay URL is unusable on purpose: a code of
// 1 proves the watch got past the flag and failed on the URL, while a code of 2
// would prove the mode never parsed.
func TestWatchAcceptsEveryNotifyMode(t *testing.T) {
	for _, mode := range []string{"off", "bell", "osc", "auto"} {
		t.Run(mode, func(t *testing.T) {
			got := runFeed(t, nil, "watch", "--notify", mode, "--relay", "://bad")

			if got.code != 1 {
				t.Fatalf("exit code = %d, want 1 (stderr: %q)", got.code, got.stderr)
			}
			if got.stdout != "" {
				t.Errorf("stdout = %q, want empty", got.stdout)
			}
		})
	}
}

func TestWatchUsageNamesEveryNotifyMode(t *testing.T) {
	got := runFeed(t, nil, "watch", "--help")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	if !strings.Contains(got.stderr, "--notify <off|bell|osc|auto>") {
		t.Errorf("stderr = %q, want the usage to name the flag and its values", got.stderr)
	}
}

// The signal is not output: with the same event on the wire, a watch that
// notifies writes the same card as one that does not, and neither of the two
// streams ever carries the bell. The console is where the signal goes, which is
// exactly why its presence cannot be asserted here — the by-hand case lives in
// docs/cli-testing.md, paso 13.
func TestWatchNotificationsLeaveTheStreamsAlone(t *testing.T) {
	plain := watchOneCard(t)
	signalled := watchOneCard(t, "--notify", "bell")

	if signalled != plain {
		t.Errorf("the card with --notify = %q, want it identical to %q", signalled, plain)
	}
	for _, stream := range []string{plain, signalled} {
		if strings.ContainsRune(stream, '\a') || hasEscape(stream) {
			t.Errorf("a control byte reached the feed: %q", stream)
		}
	}
}

// watchOneCard runs one watch to the point where its only card has crossed
// stdout, then ends the session the way this platform supports. The card it
// returns is every byte the feed wrote, so two runs can be compared whole.
func watchOneCard(t *testing.T, extra ...string) string {
	t.Helper()

	relay := newFakeRelay(t)
	args := append([]string{"watch", "--relay", relay.URL, "--no-color"}, extra...)
	feed, stdout := startFeedWithPipeStdout(t, args...)
	relay.waitConnected(t)

	relay.send(t, mergedFrame(time.Now().Add(-2*time.Hour-30*time.Minute)))
	seen := readUntil(t, stdout, "https://github.com/AG2904/github-activity-feed/pull/42\n")

	endSession(t, feed, stdout, relay, assignedFrame(time.Now().Add(-2*time.Hour)))
	return seen
}

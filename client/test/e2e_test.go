package test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestVersionIsInjectedAtBuildTime(t *testing.T) {
	got := runFeed(t, nil, "--version")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	if got.stdout != testVersion+"\n" {
		t.Errorf("stdout = %q, want %q", got.stdout, testVersion+"\n")
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want empty", got.stderr)
	}
}

func TestNoSubcommandPrintsUsageAndExitsTwo(t *testing.T) {
	got := runFeed(t, nil)

	if got.code != 2 {
		t.Fatalf("exit code = %d, want 2", got.code)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
	for _, want := range []string{"Usage:", "watch", "history"} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("stderr = %q, want it to mention %q", got.stderr, want)
		}
	}
}

func TestUnknownSubcommandPrintsUsageAndExitsTwo(t *testing.T) {
	got := runFeed(t, nil, "bogus")

	if got.code != 2 {
		t.Fatalf("exit code = %d, want 2", got.code)
	}
	if !strings.Contains(got.stderr, `unknown command "bogus"`) {
		t.Errorf("stderr = %q, want it to name the unknown command", got.stderr)
	}
	if !strings.Contains(got.stderr, "Usage:") {
		t.Errorf("stderr = %q, want usage", got.stderr)
	}
}

func TestWatchUsesFlagOverEnvironment(t *testing.T) {
	fromEnvironment := newFakeRelay(t)
	fromFlag := newFakeRelay(t)

	feed := startFeed(t, map[string]string{"RELAY_URL": fromEnvironment.URL}, "watch", "--relay", fromFlag.URL)
	fromFlag.waitConnected(t)
	feed.waitStderr(t, "watching "+fromFlag.URL+"/stream")

	if hits := fromFlag.hits("/stream"); hits != 1 {
		t.Errorf("the --relay relay saw %d /stream requests, want 1", hits)
	}
	if seen := fromEnvironment.requests(); len(seen) != 0 {
		t.Errorf("RELAY_URL relay saw %v, want nothing", seen)
	}
	if got := feed.stdout.String(); got != "" {
		t.Errorf("stdout = %q, want empty", got)
	}
}

func TestWatchUsesEnvironmentWhenNoFlagIsGiven(t *testing.T) {
	relay := newFakeRelay(t)

	startFeed(t, map[string]string{"RELAY_URL": relay.URL}, "watch")
	relay.waitConnected(t)

	if hits := relay.hits("/stream"); hits != 1 {
		t.Errorf("RELAY_URL relay saw %d /stream requests, want 1", hits)
	}
}

func TestWatchDefaultsToLocalhost3000(t *testing.T) {
	banner := firstWatchBanner(t)

	if !strings.Contains(banner, "watching http://localhost:3000/stream") {
		t.Errorf("banner = %q, want the default relay", banner)
	}
}

func TestHistoryReachesTheRelay(t *testing.T) {
	relay := newFakeRelay(t)

	got := runFeed(t, nil, "history", "--relay", relay.URL)

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	if hits := relay.hits("/history"); hits != 1 {
		t.Errorf("relay saw %d /history requests, want 1", hits)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
}

func TestWatchPaintsAMergedPullRequest(t *testing.T) {
	relay := newFakeRelay(t)
	// NO_COLOR is one of the two ways to turn escapes off; the other is
	// --no-color, exercised by the assignment case below.
	feed := startFeed(t, map[string]string{"NO_COLOR": "1"}, "watch", "--relay", relay.URL)
	relay.waitConnected(t)

	relay.send(t, mergedFrame(time.Now().Add(-2*time.Hour-30*time.Minute)))

	want := "[pr.merged] Fix memory leak in parser component\n" +
		"AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/pull/42\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if strings.ContainsRune(feed.stdout.String(), '\x1b') {
		t.Errorf("stdout contains an escape sequence while piped: %q", feed.stdout.String())
	}
}

func TestWatchPaintsTheAssigneeOnAnAssignment(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	relay.send(t, assignedFrame(time.Now().Add(-2*time.Hour-30*time.Minute)))

	want := "[issue.assigned] lucia.gomez Ship the onboarding checklist\n" +
		"AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/issues/7\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if strings.ContainsRune(feed.stdout.String(), '\x1b') {
		t.Errorf("stdout contains an escape sequence while piped: %q", feed.stdout.String())
	}
}

func TestWatchDiscardsFramesItCannotUseAndKeepsReading(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL)
	relay.waitConnected(t)

	when := time.Now().Add(-2 * time.Hour)
	junk := "id: 1\n" +
		": heartbeat\n\n" +
		"event: pr.merged\n\n" +
		"retry: 1500\n\n" +
		`data: {"type":"pr.merged"` + "\n\n" +
		"data: this is not json\n\n"
	relay.send(t, junk)

	relay.send(t, mergedFrame(when))
	relay.send(t, assignedFrame(when))

	want := "[pr.merged] Fix memory leak in parser component\n" +
		"AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/pull/42\n" +
		"[issue.assigned] lucia.gomez Ship the onboarding checklist\n" +
		"AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/issues/7\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want exactly the two parseable events %q", got, want)
	}
}

func mergedFrame(when time.Time) string {
	return "data: " + fmt.Sprintf(
		`{"type":"pr.merged","repository":"AG2904/github-activity-feed","actor":"maria.perez","subject":"Fix memory leak in parser component","assignee":null,"labels":[],"url":"https://github.com/AG2904/github-activity-feed/pull/42","when":"%s"}`,
		stamp(when),
	) + "\n\n"
}

func assignedFrame(when time.Time) string {
	return "data: " + fmt.Sprintf(
		`{"type":"issue.assigned","repository":"AG2904/github-activity-feed","actor":"maria.perez","subject":"Ship the onboarding checklist","assignee":"lucia.gomez","labels":["bug"],"url":"https://github.com/AG2904/github-activity-feed/issues/7","when":"%s"}`,
		stamp(when),
	) + "\n\n"
}

// stamp is the relay's canonical form: UTC, ISO-8601, milliseconds.
func stamp(when time.Time) string {
	return when.UTC().Format("2006-01-02T15:04:05.000Z")
}

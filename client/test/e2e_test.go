package test

import (
	"fmt"
	"net"
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

func TestWatchJSONWritesTheFramePayloadVerbatim(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--json")
	relay.waitConnected(t)

	// The junk frames must still be discarded: --json changes the shape of the
	// output, not the rule about what counts as an event.
	junk := "id: 1\n" +
		": heartbeat\n\n" +
		"event: pr.merged\n\n" +
		"retry: 1500\n\n" +
		`data: {"type":"pr.merged"` + "\n\n" +
		"data: this is not json\n\n"
	relay.send(t, junk)
	relay.send(t, "data: "+rawPayload+"\n\n")

	feed.waitStdout(t, rawPayload+"\n")

	if got := feed.stdout.String(); got != rawPayload+"\n" {
		t.Errorf("stdout = %q, want the payload byte for byte plus one newline", got)
	}
	if strings.ContainsRune(feed.stdout.String(), '\x1b') {
		t.Errorf("stdout contains an escape sequence in --json mode: %q", feed.stdout.String())
	}
}

func TestHistoryJSONWritesTheResponseBodyVerbatim(t *testing.T) {
	relay := newFakeRelay(t)
	relay.setHistory(historyBody)

	got := runFeed(t, nil, "history", "--relay", relay.URL, "--json")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	if got.stdout != historyBody {
		t.Errorf("stdout = %q, want the relay's body byte for byte %q", got.stdout, historyBody)
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want empty", got.stderr)
	}
}

func TestHistoryJSONRejectsGrouping(t *testing.T) {
	relay := newFakeRelay(t)

	got := runFeed(t, nil, "history", "--relay", relay.URL, "--json", "--group-by-repo")

	if got.code != 2 {
		t.Fatalf("exit code = %d, want 2", got.code)
	}
	if !strings.Contains(got.stderr, "grouping is presentation") {
		t.Errorf("stderr = %q, want it to explain why grouping and --json cannot combine", got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
	if hits := relay.hits("/history"); hits != 0 {
		t.Errorf("relay saw %d /history requests, want 0: the conflict is a flag error", hits)
	}
}

func TestWatchRetriesAnUnreachableRelay(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a closed port: %v", err)
	}
	relay := "http://" + listener.Addr().String()
	listener.Close()

	feed := startFeed(t, nil, "watch", "--relay", relay)
	delays := feed.waitReconnects(t, 2)

	if len(delays) != 2 {
		t.Fatalf("reconnect notices = %d, want exactly 2", len(delays))
	}
	// The banner plus one line per attempt: nothing else may reach stderr.
	if lines := nonEmptyLines(feed.stderr.String()); len(lines) != 3 {
		t.Errorf("stderr lines = %d, want the banner plus one line per attempt\n%v", len(lines), lines)
	}
}

func TestWatchFailsOnAnUnusableRelayURL(t *testing.T) {
	got := runFeed(t, nil, "watch", "--relay", "://bad")

	if got.code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr: %q)", got.code, got.stderr)
	}
	if strings.Contains(got.stderr, "reconnecting") {
		t.Errorf("stderr = %q, want an unusable URL to be fatal, not retried", got.stderr)
	}
}

func TestReconnectBackoffGrowsAndResetsAfterAnEvent(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL)
	relay.waitConnected(t)

	relay.dropStream(t)
	feed.waitReconnects(t, 1)
	relay.waitConnected(t)
	feed.waitStderr(t, "watch: reconnected")

	relay.dropStream(t)
	delays := feed.waitReconnects(t, 2)
	relay.waitConnected(t)

	// The event is what resets the backoff; the drop comes after the event.
	relay.send(t, mergedFrame(time.Now().Add(-2*time.Hour-30*time.Minute)))
	feed.waitStdout(t, "Fix memory leak in parser component")

	relay.dropStream(t)
	delays = feed.waitReconnects(t, 3)
	relay.waitConnected(t)

	if delays[0] < time.Second || delays[0] >= 1500*time.Millisecond {
		t.Errorf("first delay = %s, want the one-second base plus its jitter", delays[0])
	}
	if delays[1] < 2*time.Second {
		t.Errorf("second delay = %s, want the backoff to have grown", delays[1])
	}
	if delays[2] >= 1500*time.Millisecond {
		t.Errorf("third delay = %s, want the event to have reset the backoff", delays[2])
	}
	if delays[2] >= delays[1] {
		t.Errorf("delays = %v, want the third shorter than the second", delays)
	}
}

func TestReconnectDoesNotReplayPrintedEvents(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL)
	relay.waitConnected(t)

	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.send(t, mergedFrame(when))
	feed.waitStdout(t, "Fix memory leak in parser component")

	relay.dropStream(t)
	feed.waitReconnects(t, 1)
	relay.waitConnected(t)
	feed.waitStderr(t, "watch: reconnected")

	for _, id := range relay.lastEventIDs() {
		if id != "" {
			t.Errorf("a stream request offered Last-Event-ID %q, want none: the relay's counter is not a position", id)
		}
	}

	relay.send(t, assignedFrame(when))
	want := "[pr.merged] Fix memory leak in parser component\n" +
		"AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/pull/42\n" +
		"[issue.assigned] lucia.gomez Ship the onboarding checklist\n" +
		"AG2904/github-activity-feed · maria.perez · 2h ago · https://github.com/AG2904/github-activity-feed/issues/7\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want each event exactly once %q", got, want)
	}
}

func TestWatchReopensAStreamThatGoesSilent(t *testing.T) {
	relay := newFakeRelay(t)
	// This build carries a 300ms idle timeout through -ldflags; the shipped
	// default stays at five minutes.
	feed := startBinary(t, idleFeedBinary, nil, "watch", "--relay", relay.URL)
	relay.waitConnected(t)

	feed.waitReconnects(t, 1)
	relay.waitConnected(t)

	if !strings.Contains(feed.stderr.String(), "no bytes") {
		t.Errorf("stderr = %q, want the idle timeout named as the reason", feed.stderr.String())
	}
	if hits := relay.hits("/stream"); hits < 2 {
		t.Errorf("relay saw %d /stream requests, want the silent stream reopened", hits)
	}
}

func TestHeartbeatsKeepASilentStreamOpen(t *testing.T) {
	relay := newFakeRelay(t)
	// The idle build's timeout is 300ms, so a comment every 100ms is the
	// relay's heartbeat and must keep the connection alive.
	feed := startBinary(t, idleFeedBinary, nil, "watch", "--relay", relay.URL)
	relay.waitConnected(t)

	for i := 0; i < 6; i++ {
		relay.send(t, ": heartbeat\n\n")
		time.Sleep(100 * time.Millisecond)
	}

	if delays := parseReconnects(feed.stderr.String()); len(delays) != 0 {
		t.Errorf("stderr shows %v, want a heartbeat to keep the stream open", delays)
	}
	if hits := relay.hits("/stream"); hits != 1 {
		t.Errorf("relay saw %d /stream requests, want 1", hits)
	}
}

// rawPayload is the relay's own bytes, with an ampersand that Go's encoder
// would escape as \u0026 if the CLI re-marshalled instead of passing it through.
const rawPayload = `{"type":"pr.merged","repository":"AG2904/github-activity-feed","actor":"maria.perez","subject":"Fix parser & renderer","assignee":null,"labels":[],"url":"https://github.com/AG2904/github-activity-feed/pull/42","when":"2026-10-11T13:00:00.000Z"}`

// historyBody is an audit-history array with both event shapes, again carrying
// an ampersand to make re-serialization visible.
const historyBody = `[{"type":"pr.merged","repository":"AG2904/github-activity-feed","actor":"maria.perez","subject":"Fix parser & renderer","assignee":null,"labels":[],"url":"https://github.com/AG2904/github-activity-feed/pull/42","when":"2026-10-11T13:00:00.000Z"},{"type":"issue.assigned","repository":"AG2904/github-activity-feed","actor":"maria.perez","subject":"Ship the onboarding checklist","assignee":"lucia.gomez","labels":["bug"],"url":"https://github.com/AG2904/github-activity-feed/issues/7","when":"2026-10-11T14:00:00.000Z"}]`

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

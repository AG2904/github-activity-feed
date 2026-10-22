package test

import (
	"fmt"
	"net"
	"os"
	"runtime"
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

	if !strings.Contains(banner, "watching http://localhost:3000/stream · backfill 10\n") {
		t.Errorf("banner = %q, want the default relay with its default backfill", banner)
	}
}

func TestWatchBannerNamesTheBackfillAndFilters(t *testing.T) {
	relay := newFakeRelay(t)

	feed := startFeed(t, nil, "watch", "--relay", relay.URL,
		"--backfill", "3", "--user", "maria.perez", "--repo", "AG2904/github-activity-feed")
	relay.waitConnected(t)

	want := "watching " + relay.URL + "/stream · backfill 3 · user maria.perez · repo AG2904/github-activity-feed\n"
	feed.waitStderr(t, want)

	// The banner is messaging: it never reaches stdout and never carries an
	// escape, --json or not, terminal or pipe.
	if got := feed.stdout.String(); got != "" {
		t.Errorf("stdout = %q, want empty while only the chrome has spoken", got)
	}
	if hasEscape(feed.stderr.String()) {
		t.Errorf("stderr contains an escape sequence in the chrome: %q", feed.stderr.String())
	}
}

func TestWatchSaysConnectedAfterTheFirstConnection(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.setHistory(historyOf(payloadOf(mergedFrame(when)), payloadOf(assignedFrame(when))))

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	want := cards(assignedOutput, mergedOutput)
	feed.waitStdout(t, want)
	feed.waitStderr(t, "connected · 2 events from history\n")

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want the cards untouched by the chrome %q", got, want)
	}
	// Exactly one affirmative line: the count is of what the backfill printed,
	// after the filter, not of what the relay stored.
	if got := connectedLines(feed.stderr.String()); got != 1 {
		t.Errorf("stderr carries %d connection lines, want exactly 1\nstderr: %q", got, feed.stderr.String())
	}
	if hasEscape(feed.stderr.String()) {
		t.Errorf("stderr contains an escape sequence in the chrome: %q", feed.stderr.String())
	}
}

func TestWatchSaysWaitingWhenTheBackfillPrintedNothing(t *testing.T) {
	relay := newFakeRelay(t)

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)
	feed.waitStderr(t, "connected · waiting for events\n")

	// The line is about the history, not about the session: a live event later
	// does not turn it into a count.
	relay.send(t, mergedFrame(time.Now().Add(-2*time.Hour-30*time.Minute)))
	feed.waitStdout(t, mergedOutput)

	if got := connectedLines(feed.stderr.String()); got != 1 {
		t.Errorf("stderr carries %d connection lines, want exactly 1\nstderr: %q", got, feed.stderr.String())
	}
}

// A relay that refuses the very first attempt does not lose the session its
// welcome: the affirmative line belongs to the first connection the relay
// accepts, not to the first attempt.
func TestWatchSaysConnectedWhenTheFirstAttemptIsRefused(t *testing.T) {
	relay := newFakeRelay(t)
	relay.failStream(1)

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)
	feed.waitStderr(t, "connected · waiting for events\n")

	// The refusal before it is a retry worth naming, but never a recovery:
	// `watch: reconnected` may only follow a connection that was accepted once.
	stderr := feed.stderr.String()
	index := strings.Index(stderr, "connected · ")
	if index < 0 {
		t.Fatalf("stderr = %q, want the affirmative line", stderr)
	}
	if before := stderr[:index]; strings.Contains(before, "watch: reconnected") {
		t.Errorf("stderr = %q, want no reconnect notice before the first accepted connection", stderr)
	}
	if got := connectedLines(stderr); got != 1 {
		t.Errorf("stderr carries %d connection lines, want exactly 1\nstderr: %q", got, stderr)
	}
}

func TestWatchSaysConnectedOnceAcrossReconnects(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)
	feed.waitStderr(t, "connected · waiting for events\n")

	relay.dropStream(t)
	feed.waitReconnects(t, 1)
	relay.waitConnected(t)
	feed.waitStderr(t, "watch: reconnected\n")

	// A healthy quiet relay heartbeats in comments: the affirmative line
	// answered the only moment "is it alive?" had no other reply, so the
	// silence that follows stays silent.
	for i := 0; i < 3; i++ {
		relay.send(t, ": heartbeat\n\n")
		time.Sleep(100 * time.Millisecond)
	}

	// The banner, the one connection line, and the reconnecting/reconnected
	// pair of the drop: nothing else may reach stderr.
	if lines := nonEmptyLines(feed.stderr.String()); len(lines) != 4 {
		t.Errorf("stderr has %d lines, want banner, connected and the reconnect pair\n%v", len(lines), lines)
	}
	if got := connectedLines(feed.stderr.String()); got != 1 {
		t.Errorf("stderr carries %d connection lines, want exactly 1\nstderr: %q", got, feed.stderr.String())
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

	want := mergedOutput
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if hasEscape(feed.stdout.String()) {
		t.Errorf("stdout contains an escape sequence while piped: %q", feed.stdout.String())
	}
}

func TestWatchPaintsTheAssigneeOnAnAssignment(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	relay.send(t, assignedFrame(time.Now().Add(-2*time.Hour-30*time.Minute)))

	want := assignedOutput
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if hasEscape(feed.stdout.String()) {
		t.Errorf("stdout contains an escape sequence while piped: %q", feed.stdout.String())
	}
}

func TestWatchRendersAnAssignmentWithoutAnAssignee(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	relay.send(t, eventFrame(time.Now().Add(-2*time.Hour-30*time.Minute), "issue.assigned",
		"AG2904/github-activity-feed", "", "lucia.gomez", "Ship the onboarding checklist", nil,
		`["bug"]`, "https://github.com/AG2904/github-activity-feed/issues/7"))

	want := "lucia.gomez assigned\n" +
		"  [bug] Ship the onboarding checklist\n" +
		"  AG2904/github-activity-feed · 2h ago\n" +
		"  https://github.com/AG2904/github-activity-feed/issues/7\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want an assignment with no destination %q", got, want)
	}
}

func TestWatchRendersEveryLabel(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	relay.send(t, eventFrame(time.Now().Add(-2*time.Hour-30*time.Minute), "issue.assigned",
		"AG2904/github-activity-feed", "", "lucia.gomez", "Ship the onboarding checklist",
		ptr("tomas.ferreira"), `["bug","feedback"]`,
		"https://github.com/AG2904/github-activity-feed/issues/7"))

	want := "lucia.gomez assigned to tomas.ferreira\n" +
		"  [bug, feedback] Ship the onboarding checklist\n" +
		"  AG2904/github-activity-feed · 2h ago\n" +
		"  https://github.com/AG2904/github-activity-feed/issues/7\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want every label between the brackets %q", got, want)
	}
}

// The labels pin has two halves, and this is the second: a card whose event
// carries no labels starts line two at the title — no empty "[]", no leading
// space, no extra blank line. Its own event, because the merged fixture now
// carries the reason before the what.
func TestWatchStartsLineTwoAtTheTitleWhenTheEventHasNoLabels(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	relay.send(t, eventFrame(time.Now().Add(-2*time.Hour-30*time.Minute), "pr.merged",
		"AG2904/github-activity-feed", "", "maria.perez", "Fix memory leak in parser component",
		nil, "[]", "https://github.com/AG2904/github-activity-feed/pull/42"))

	want := "maria.perez merged into main\n" +
		"  Fix memory leak in parser component\n" +
		"  AG2904/github-activity-feed · 2h ago\n" +
		"  https://github.com/AG2904/github-activity-feed/pull/42\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want line two to start at the title, not at an empty [] %q", got, want)
	}
}

func TestWatchRendersALegacyRowWithoutARepository(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	// An empty repository is the null of a row stored before the column existed
	// (ADR-0010): the context line is the time alone, and the label rides with
	// the title instead of filling the repository's place.
	relay.send(t, eventFrame(time.Now().Add(-2*time.Hour-30*time.Minute), "pr.merged",
		"", "", "maria.perez", "Legacy merge", nil, `["bug"]`,
		"https://github.com/AG2904/github-activity-feed/pull/1"))

	want := "maria.perez merged into main\n" +
		"  [bug] Legacy merge\n" +
		"  2h ago\n" +
		"  https://github.com/AG2904/github-activity-feed/pull/1\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want a context line of just the time %q", got, want)
	}
}

// The branch is supplementary: a merge stored before the relay recorded one
// falls back to the type-derived phrase, with no empty segment and no doubled
// `into main`.
func TestWatchRendersAMergeWithoutABranch(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	relay.send(t, eventFrame(time.Now().Add(-2*time.Hour-30*time.Minute), "pr.merged",
		"AG2904/github-activity-feed", "", "maria.perez", "Legacy merge", nil, `["bug"]`,
		"https://github.com/AG2904/github-activity-feed/pull/1"))

	want := "maria.perez merged into main\n" +
		"  [bug] Legacy merge\n" +
		"  AG2904/github-activity-feed · 2h ago\n" +
		"  https://github.com/AG2904/github-activity-feed/pull/1\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want the type-derived phrase when the branch is null %q", got, want)
	}
}

func TestRelativeTimeSaysYesterday(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	relay.send(t, mergedFrame(time.Now().Add(-30*time.Hour)))
	relay.send(t, mergedFrame(time.Now().Add(-50*time.Hour)))

	want := "maria.perez merged fix/memory-leak into main\n" +
		"  [bug] Fix memory leak in parser component\n" +
		"  AG2904/github-activity-feed · yesterday\n" +
		"  https://github.com/AG2904/github-activity-feed/pull/42\n" +
		"\n" +
		"maria.perez merged fix/memory-leak into main\n" +
		"  [bug] Fix memory leak in parser component\n" +
		"  AG2904/github-activity-feed · 2d ago\n" +
		"  https://github.com/AG2904/github-activity-feed/pull/42\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want yesterday and then two days %q", got, want)
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

	want := cards(mergedOutput, assignedOutput)
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
	if hasEscape(feed.stdout.String()) {
		t.Errorf("stdout contains an escape sequence in --json mode: %q", feed.stdout.String())
	}
	// The chrome still narrates on stderr: --json changes the shape of stdout,
	// not stderr's job.
	feed.waitStderr(t, "connected · waiting for events")
}

func TestHistoryJSONHandsOverTheRelayBytes(t *testing.T) {
	relay := newFakeRelay(t)
	relay.setHistory(historyBody)

	got := runFeed(t, nil, "history", "--relay", relay.URL, "--json")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	// Without a filter the array the CLI frames is the array the relay served,
	// so the body comes through byte for byte, ampersand included.
	if got.stdout != historyBody {
		t.Errorf("stdout = %q, want the relay's body byte for byte %q", got.stdout, historyBody)
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want empty", got.stderr)
	}
}

func TestHistoryJSONFiltersBeforeFraming(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	assigned := payloadOf(assignedFrame(when))
	relay.setHistory(historyOf(payloadOf(mergedFrame(when)), assigned))

	got := runFeed(t, nil, "history", "--relay", relay.URL, "--json", "--user", "lucia.gomez")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	// Raw is a shape, not an exemption: the filter runs, and what survives keeps
	// the relay's own bytes.
	want := "[" + assigned + "]"
	if got.stdout != want {
		t.Errorf("stdout = %q, want only the matching event's bytes %q", got.stdout, want)
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

func TestWatchSaysStoppedWhenItLeavesThroughAFatalError(t *testing.T) {
	got := runFeed(t, nil, "watch", "--relay", "://bad")

	if got.code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr: %q)", got.code, got.stderr)
	}
	// The URL was unusable before a single event could be shown, so the
	// account says nothing was — and it is the last word on stderr.
	if !strings.HasSuffix(got.stderr, "stopped · no events\n") {
		t.Errorf("stderr = %q, want it to end with the account of a session that showed nothing", got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
}

func TestWatchSaysStoppedWithTheCountsItShowed(t *testing.T) {
	relay := newFakeRelay(t)
	feed, stdout := startFeedWithPipeStdout(t, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	// The first card crosses the pipe while the readers are still there...
	relay.send(t, mergedFrame(time.Now().Add(-2*time.Hour-30*time.Minute)))
	readUntil(t, stdout, "https://github.com/AG2904/github-activity-feed/pull/42\n")

	// ...and then the session ends. The assignment is the card the end cannot
	// show — Windows by breaking the pipe under its write — so it may never
	// enter the account: the session must still account for the card it did.
	endSession(t, feed, stdout, relay, assignedFrame(time.Now().Add(-2*time.Hour-30*time.Minute)))
	wantStopped(t, feed, "stopped · 1 events · 1 merged · 0 assigned\n")
}

// The account answers "what did I see": an event the filter dropped was never
// shown, so it may not appear in the counts — not in the total, not in a
// category.
func TestWatchStoppedCountsLeaveOutWhatTheFilterDiscarded(t *testing.T) {
	relay := newFakeRelay(t)
	feed, stdout := startFeedWithPipeStdout(t, "watch", "--relay", relay.URL,
		"--no-color", "--user", "maria.perez")
	relay.waitConnected(t)

	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	// Somebody else's assignment is valid but discarded by the filter before
	// it is ever shown...
	relay.send(t, eventFrame(when, "issue.assigned", "AG2904/github-activity-feed", "",
		"someone.else", "Not maria's assignment", ptr("tomas.ferreira"), `["bug"]`,
		"https://github.com/AG2904/github-activity-feed/issues/8"))
	// ...and maria's merge is the one card the session shows.
	relay.send(t, mergedFrame(when))
	readUntil(t, stdout, "https://github.com/AG2904/github-activity-feed/pull/42\n")

	endSession(t, feed, stdout, relay, assignedFrame(when))

	wantStopped(t, feed, "stopped · 1 events · 1 merged · 0 assigned\n")
}

// endSession ends a running watch with the exit path this platform supports,
// and asserts the exit code that path promises. Windows cannot deliver Ctrl-C
// to an arbitrary child — Process.Signal(os.Interrupt) is unimplemented there —
// so the session ends the way the CLI can be stopped from outside: the test
// breaks the stdout pipe and the failed write walks the closed-stdout fatal
// path; stopFrame is the event whose write is that trigger, and it is never
// shown. Everywhere else the interrupt main.go listens for cancels the context,
// watch returns 0, and the deferred account speaks on the way out.
func endSession(t *testing.T, feed *runningFeed, stdout *os.File, relay *fakeRelay, stopFrame string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		stdout.Close()
		relay.send(t, stopFrame)
		if code := feed.waitExit(t); code != 1 {
			t.Errorf("exit code = %d, want 1 through the closed-stdout fatal path (stderr: %q)",
				code, feed.stderr.String())
		}
		return
	}
	if err := feed.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("sending Ctrl-C to the feed: %v", err)
	}
	if code := feed.waitExit(t); code != 0 {
		t.Errorf("exit code = %d, want 0 through the cancelled context (stderr: %q)",
			code, feed.stderr.String())
	}
}

// wantStopped asserts the session's closing account: on stderr, carrying
// exactly the counts the test names.
func wantStopped(t *testing.T, feed *runningFeed, want string) {
	t.Helper()
	if got := feed.stderr.String(); !strings.Contains(got, want) {
		t.Errorf("stderr = %q, want it to carry %q", got, want)
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
	want := cards(mergedOutput, assignedOutput)
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

func TestWatchFiltersByActor(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--user", "maria.perez")
	relay.waitConnected(t)

	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.send(t, eventFrame(when, "pr.merged", "other/repo", "", "someone.else", "Not maria's merge", nil, "[]", "https://github.com/other/repo/pull/2"))
	relay.send(t, mergedFrame(when))
	relay.send(t, assignedFrame(when))

	want := cards(mergedOutput, assignedOutput)
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want only the events where maria is the actor %q", got, want)
	}
}

func TestWatchFiltersByAssignee(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--user", "lucia.gomez")
	relay.waitConnected(t)

	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.send(t, mergedFrame(when))
	relay.send(t, assignedFrame(when))

	want := assignedOutput
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want only the assignment where lucia is the assignee %q", got, want)
	}
}

func TestWatchFiltersByRepository(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--repo", "AG2904/github-activity-feed")
	relay.waitConnected(t)

	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.send(t, eventFrame(when, "pr.merged", "other/repo", "", "maria.perez", "Other repo merge", nil, "[]", "https://github.com/other/repo/pull/3"))
	// A row stored before the repository column existed (ADR-0010).
	relay.send(t, eventFrame(when, "pr.merged", "", "", "maria.perez", "Legacy merge", nil, "[]", "https://github.com/AG2904/github-activity-feed/pull/1"))
	relay.send(t, mergedFrame(when))

	want := mergedOutput
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want only the events in the filtered repository %q", got, want)
	}
}

func TestWatchFiltersCombineAsConjunction(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--user", "lucia.gomez", "--repo", "other/repo")
	relay.waitConnected(t)

	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.send(t, assignedFrame(when))
	relay.send(t, eventFrame(when, "issue.assigned", "other/repo", "", "someone.else", "Other repo assignment", ptr("lucia.gomez"), `["bug"]`, "https://github.com/other/repo/issues/9"))

	want := "someone.else assigned to lucia.gomez\n" +
		"  [bug] Other repo assignment\n" +
		"  other/repo · 2h ago\n" +
		"  https://github.com/other/repo/issues/9\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want user and repository to combine as a conjunction %q", got, want)
	}
}

func TestWatchFiltersAreCaseSensitive(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--user", "maria.perez")
	relay.waitConnected(t)

	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.send(t, eventFrame(when, "pr.merged", "other/repo", "", "Maria.Perez", "Cased merge", nil, "[]", "https://github.com/other/repo/pull/4"))
	relay.send(t, mergedFrame(when))

	want := mergedOutput
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want an exact login match, not a case-folded one %q", got, want)
	}
}

func TestWatchJSONFiltersBeforeWritingRaw(t *testing.T) {
	relay := newFakeRelay(t)
	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--json", "--user", "lucia.gomez")
	relay.waitConnected(t)

	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.send(t, mergedFrame(when))
	assigned := assignedFrame(when)
	relay.send(t, assigned)

	want := payloadOf(assigned) + "\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want only the matching payload %q", got, want)
	}
}

func TestWatchBackfillsTheFirstConnection(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	// The relay serves the audit history newest first; the backfill is the head
	// of a stream, so it is printed oldest first and the live feed continues
	// below it.
	relay.setHistory(historyOf(payloadOf(mergedFrame(when)), payloadOf(assignedFrame(when))))

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	want := cards(assignedOutput, mergedOutput)
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want the backfill oldest first %q", got, want)
	}
	relay.wantHits(t, "/history", 1, "the first connection reads it once")
}

func TestWatchBackfillLimitsToTheMostRecentEvents(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.setHistory(historyOf(
		payloadOf(assignedFrame(when)),
		payloadOf(mergedFrame(when)),
		payloadOf(eventFrame(when, "pr.merged", "other/repo", "", "someone.else", "Oldest merge", nil, "[]", "https://github.com/other/repo/pull/5")),
	))

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color", "--backfill", "1")
	relay.waitConnected(t)

	want := assignedOutput
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want only the newest event %q", got, want)
	}
}

func TestWatchBackfillZeroStartsBlank(t *testing.T) {
	relay := newFakeRelay(t)
	relay.setHistory(historyOf(payloadOf(mergedFrame(time.Now().Add(-2 * time.Hour)))))

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color", "--backfill", "0")
	relay.waitConnected(t)

	relay.send(t, assignedFrame(time.Now().Add(-2*time.Hour-30*time.Minute)))

	want := assignedOutput
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want nothing until a live event arrives %q", got, want)
	}
	relay.wantHits(t, "/history", 0, "a disabled backfill asks nothing")
}

func TestWatchBackfillHonoursTheFiltersBeforeTheLimit(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	// The newest stored event is in another repository, so a limit applied
	// before the filter would print nothing at all.
	relay.setHistory(historyOf(
		payloadOf(eventFrame(when, "pr.merged", "other/repo", "", "maria.perez", "Newer merge elsewhere", nil, "[]", "https://github.com/other/repo/pull/6")),
		payloadOf(mergedFrame(when)),
	))

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color",
		"--user", "maria.perez", "--repo", "AG2904/github-activity-feed", "--backfill", "1")
	relay.waitConnected(t)

	want := mergedOutput
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want the most recent event of mine %q", got, want)
	}
	// Filtering never narrows what the relay sends, and the history is read
	// before the stream opens, so an event that arrives during the read is not
	// printed twice.
	if got := strings.Join(relay.requests(), " "); got != "/history /stream" {
		t.Errorf("relay saw %q, want an unfiltered history first", got)
	}
}

func TestWatchBackfillsTenEventsByDefault(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)

	// Twelve stored events, newest first, as the relay serves them: the last to
	// happen carries the highest number, so it comes first.
	payloads := make([]string, 0, 12)
	for i := 11; i >= 0; i-- {
		payloads = append(payloads, payloadOf(eventFrame(when, "pr.merged", "AG2904/github-activity-feed", "",
			"maria.perez", fmt.Sprintf("Merge %d", i), nil, "[]",
			fmt.Sprintf("https://github.com/AG2904/github-activity-feed/pull/%d", 100+i))))
	}
	relay.setHistory(historyOf(payloads...))

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	// Merge 11 is the newest of the ten the default prints, and therefore the
	// last of the backfill to reach stdout.
	feed.waitStdout(t, "Merge 11\n")

	// Ten events, four lines each without colour: the default is ten and not
	// twelve.
	if lines := nonEmptyLines(feed.stdout.String()); len(lines) != 40 {
		t.Errorf("stdout has %d lines, want 10 cards on four lines each\n%v", len(lines), lines)
	}
	for _, want := range []string{"Merge 11\n", "Merge 2\n"} {
		if !strings.Contains(feed.stdout.String(), want) {
			t.Errorf("stdout = %q, want it to contain %q", feed.stdout.String(), want)
		}
	}
	for _, unwanted := range []string{"Merge 1\n", "Merge 0\n"} {
		if strings.Contains(feed.stdout.String(), unwanted) {
			t.Errorf("stdout = %q, want the older events left out: found %q", feed.stdout.String(), unwanted)
		}
	}
}

func TestReconnectNeverRepeatsTheBackfill(t *testing.T) {
	relay := newFakeRelay(t)
	relay.setHistory(historyOf(payloadOf(mergedFrame(time.Now().Add(-2*time.Hour - 30*time.Minute)))))

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)
	feed.waitStdout(t, mergedOutput)

	relay.dropStream(t)
	feed.waitReconnects(t, 1)
	relay.waitConnected(t)
	feed.waitStderr(t, "watch: reconnected")

	relay.wantHits(t, "/history", 1, "the backfill runs on the first connection only")
	if got := feed.stdout.String(); got != mergedOutput {
		t.Errorf("stdout = %q, want the backfilled event exactly once %q", got, mergedOutput)
	}
}

func TestWatchBackfillsNothingFromAnEmptyHistory(t *testing.T) {
	relay := newFakeRelay(t)

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)
	relay.send(t, mergedFrame(time.Now().Add(-2*time.Hour-30*time.Minute)))
	feed.waitStdout(t, mergedOutput)

	if got := feed.stdout.String(); got != mergedOutput {
		t.Errorf("stdout = %q, want an empty history to print nothing and follow the stream", got)
	}
	relay.wantHits(t, "/history", 1, "the empty history was asked for")
}

func TestWatchFollowsTheStreamWhenTheHistoryIsUnreadable(t *testing.T) {
	relay := newFakeRelay(t)
	relay.setHistory("this is not the history the relay promised")

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)
	relay.send(t, mergedFrame(time.Now().Add(-2*time.Hour-30*time.Minute)))
	feed.waitStdout(t, mergedOutput)

	// The stream is what a watch was asked for; a history the CLI cannot read
	// is a duller terminal, not a broken watch.
	if got := feed.stdout.String(); got != mergedOutput {
		t.Errorf("stdout = %q, want the live feed despite the unreadable history", got)
	}
}

func TestWatchFollowsTheStreamWhenTheHistoryAnswersBadly(t *testing.T) {
	relay := newFakeRelay(t)
	relay.failHistory(true)

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)
	relay.send(t, mergedFrame(time.Now().Add(-2*time.Hour-30*time.Minute)))
	feed.waitStdout(t, mergedOutput)

	if got := feed.stdout.String(); got != mergedOutput {
		t.Errorf("stdout = %q, want the live feed despite the failing history", got)
	}
	if got := strings.Join(relay.requests(), " "); got != "/history /stream" {
		t.Errorf("relay saw %q, want the failing history asked for once, before the stream", got)
	}
}

func TestWatchRetriesTheBackfillUntilTheHistoryIsRead(t *testing.T) {
	relay := newFakeRelay(t)
	relay.setHistory(historyOf(payloadOf(mergedFrame(time.Now().Add(-2*time.Hour - 30*time.Minute)))))
	relay.failHistory(true)

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)
	relay.wantHits(t, "/history", 1, "the first connection asked and was refused")

	// The relay recovers and the stream drops: the next connection fills the
	// terminal, because a history that was never read is not a backfill that
	// happened.
	relay.failHistory(false)
	relay.dropStream(t)
	feed.waitReconnects(t, 1)
	relay.waitConnected(t)
	feed.waitStdout(t, mergedOutput)

	if got := feed.stdout.String(); got != mergedOutput {
		t.Errorf("stdout = %q, want the refused history backfilled once the relay answered", got)
	}
	relay.wantHits(t, "/history", 2, "a refused history is asked again")
}

func TestWatchJSONBackfillsOnePayloadPerLine(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	assigned := payloadOf(assignedFrame(when))
	// Newest first, as the relay serves it.
	relay.setHistory(historyOf(assigned, rawPayload))

	feed := startFeed(t, nil, "watch", "--relay", relay.URL, "--json")
	relay.waitConnected(t)

	// Oldest first, byte for byte, one event per line: the shape the live
	// stream continues in, so one parser reads both.
	want := rawPayload + "\n" + assigned + "\n"
	feed.waitStdout(t, want)

	if got := feed.stdout.String(); got != want {
		t.Errorf("stdout = %q, want the relay's own bytes one event per line %q", got, want)
	}
	if hasEscape(feed.stdout.String()) {
		t.Errorf("stdout contains an escape sequence in --json mode: %q", feed.stdout.String())
	}
}

func TestHistoryListsTheStoredEventsNewestFirst(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	// Newest first, the order the relay serves; the report keeps it.
	relay.setHistory(historyOf(payloadOf(assignedFrame(when)), payloadOf(mergedFrame(when))))

	got := runFeed(t, nil, "history", "--relay", relay.URL, "--no-color")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	want := cards(assignedOutput, mergedOutput)
	if got.stdout != want {
		t.Errorf("stdout = %q, want the relay's own order %q", got.stdout, want)
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want empty", got.stderr)
	}
	relay.wantHits(t, "/history", 1, "the report is one query")
}

func TestHistoryFiltersByInvolvement(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.setHistory(historyOf(
		payloadOf(eventFrame(when, "pr.merged", "AG2904/github-activity-feed", "", "someone.else", "Not maria's merge", nil, "[]", "https://github.com/AG2904/github-activity-feed/pull/8")),
		payloadOf(assignedFrame(when)),
		payloadOf(mergedFrame(when)),
	))

	got := runFeed(t, nil, "history", "--relay", relay.URL, "--no-color", "--user", "maria.perez")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	// Maria is the actor of both — she merged, and she assigned — and she has
	// nothing to do with someone else's merge.
	want := cards(assignedOutput, mergedOutput)
	if got.stdout != want {
		t.Errorf("stdout = %q, want only maria's involvement %q", got.stdout, want)
	}
}

func TestHistoryFiltersByRepository(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.setHistory(historyOf(
		payloadOf(eventFrame(when, "pr.merged", "other/repo", "", "maria.perez", "Merge elsewhere", nil, "[]", "https://github.com/other/repo/pull/9")),
		payloadOf(mergedFrame(when)),
		// A row stored before the repository column existed (ADR-0010).
		payloadOf(eventFrame(when, "pr.merged", "", "", "maria.perez", "Legacy merge", nil, "[]", "https://github.com/AG2904/github-activity-feed/pull/1")),
	))

	got := runFeed(t, nil, "history", "--relay", relay.URL, "--no-color", "--repo", "AG2904/github-activity-feed")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	// A row with no repository matches no repository filter.
	want := mergedOutput
	if got.stdout != want {
		t.Errorf("stdout = %q, want only the events in that repository %q", got.stdout, want)
	}
}

func TestHistoryFiltersCombineAsConjunction(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.setHistory(historyOf(
		payloadOf(assignedFrame(when)),
		payloadOf(eventFrame(when, "issue.assigned", "other/repo", "", "someone.else", "Other repo assignment", ptr("lucia.gomez"), `["bug"]`, "https://github.com/other/repo/issues/9")),
	))

	got := runFeed(t, nil, "history", "--relay", relay.URL, "--no-color",
		"--user", "lucia.gomez", "--repo", "other/repo")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	want := "someone.else assigned to lucia.gomez\n" +
		"  [bug] Other repo assignment\n" +
		"  other/repo · 2h ago\n" +
		"  https://github.com/other/repo/issues/9\n"
	if got.stdout != want {
		t.Errorf("stdout = %q, want user and repository to combine as a conjunction %q", got.stdout, want)
	}
}

func TestHistoryGroupsByRepository(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	// other/repo comes first in the relay's order, so alphabetical order is not
	// the order these groups were first seen in; grouping has to reorder them,
	// and the order inside each group still has to survive.
	relay.setHistory(historyOf(
		payloadOf(eventFrame(when, "pr.merged", "other/repo", "", "someone.else", "Other repo merge", nil, "[]", "https://github.com/other/repo/pull/9")),
		payloadOf(mergedFrame(when)),
		payloadOf(assignedFrame(when)),
	))

	got := runFeed(t, nil, "history", "--relay", relay.URL, "--no-color", "--group-by-repo")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	// Alphabetical, the relay's order inside each group, and no repository
	// repeated on the per-event lines.
	want := "AG2904/github-activity-feed\n\n" + mergedGrouped + "\n" + assignedGrouped +
		"\n" +
		"other/repo\n\n" +
		"someone.else merged into main\n" +
		"  Other repo merge\n" +
		"  2h ago\n" +
		"  https://github.com/other/repo/pull/9\n"
	if got.stdout != want {
		t.Errorf("stdout = %q, want the grouped report %q", got.stdout, want)
	}
}

func TestHistoryGroupsTheEventsWithoutARepositoryLast(t *testing.T) {
	relay := newFakeRelay(t)
	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.setHistory(historyOf(
		// A row stored before the column existed (ADR-0010).
		payloadOf(eventFrame(when, "pr.merged", "", "", "maria.perez", "Legacy merge", nil, "[]", "https://github.com/AG2904/github-activity-feed/pull/1")),
		payloadOf(mergedFrame(when)),
	))

	got := runFeed(t, nil, "history", "--relay", relay.URL, "--no-color", "--group-by-repo")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	// Nothing to group it under and no name worth inventing, so it is written
	// last without a header.
	want := "AG2904/github-activity-feed\n\n" + mergedGrouped +
		"\n" +
		"maria.perez merged into main\n" +
		"  Legacy merge\n" +
		"  2h ago\n" +
		"  https://github.com/AG2904/github-activity-feed/pull/1\n"
	if got.stdout != want {
		t.Errorf("stdout = %q, want the ungrouped event last and without a header %q", got.stdout, want)
	}
}

func TestHistoryPrintsNothingForAnEmptyHistory(t *testing.T) {
	relay := newFakeRelay(t)

	got := runFeed(t, nil, "history", "--relay", relay.URL)

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want nothing", got.stdout)
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want empty", got.stderr)
	}
}

func TestHistoryFailsOnAnUnreachableRelay(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a closed port: %v", err)
	}
	relay := "http://" + listener.Addr().String()
	listener.Close()

	got := runFeed(t, nil, "history", "--relay", relay)

	if got.code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr: %q)", got.code, got.stderr)
	}
	if !strings.Contains(got.stderr, "history: ") {
		t.Errorf("stderr = %q, want it to name what failed", got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
}

func TestHistoryFailsOnAResponseItCannotRead(t *testing.T) {
	relay := newFakeRelay(t)
	relay.setHistory("this is not the history the relay promised")

	got := runFeed(t, nil, "history", "--relay", relay.URL)

	if got.code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr: %q)", got.code, got.stderr)
	}
	if !strings.Contains(got.stderr, "history: ") {
		t.Errorf("stderr = %q, want it to name what failed", got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
}

// rawPayload is the relay's own bytes, with an ampersand that Go's encoder
// would escape as \u0026 if the CLI re-marshalled instead of passing it through.
const rawPayload = `{"type":"pr.merged","repository":"AG2904/github-activity-feed","branch":"fix/memory-leak","actor":"maria.perez","subject":"Fix parser & renderer","assignee":null,"labels":[],"url":"https://github.com/AG2904/github-activity-feed/pull/42","when":"2026-10-11T13:00:00.000Z"}`

// historyBody is an audit-history array with both event shapes, again carrying
// an ampersand to make re-serialization visible.
const historyBody = `[{"type":"pr.merged","repository":"AG2904/github-activity-feed","branch":"fix/memory-leak","actor":"maria.perez","subject":"Fix parser & renderer","assignee":null,"labels":[],"url":"https://github.com/AG2904/github-activity-feed/pull/42","when":"2026-10-11T13:00:00.000Z"},{"type":"issue.assigned","repository":"AG2904/github-activity-feed","branch":null,"actor":"maria.perez","subject":"Ship the onboarding checklist","assignee":"lucia.gomez","labels":["bug"],"url":"https://github.com/AG2904/github-activity-feed/issues/7","when":"2026-10-11T14:00:00.000Z"}]`

// The two rendered forms the fixtures below produce, all at "2h ago". A card is
// the sentence, the title and the context; without colour the URL is a fourth
// line, because there is no hyperlink to carry it. The merged event carries the
// bug label, so its line two is the reason before the what — the pin for a card
// with no labels lives in its own test, on its own event.
const (
	mergedOutput = "maria.perez merged fix/memory-leak into main\n" +
		"  [bug] Fix memory leak in parser component\n" +
		"  AG2904/github-activity-feed · 2h ago\n" +
		"  https://github.com/AG2904/github-activity-feed/pull/42\n"
	assignedOutput = "maria.perez assigned to lucia.gomez\n" +
		"  [bug] Ship the onboarding checklist\n" +
		"  AG2904/github-activity-feed · 2h ago\n" +
		"  https://github.com/AG2904/github-activity-feed/issues/7\n"
)

// The same two events inside a grouped report, where the header already named
// the repository and the context line therefore does not. The labels still ride
// with the title: the reason is not repository-scoped.
const (
	mergedGrouped = "maria.perez merged fix/memory-leak into main\n" +
		"  [bug] Fix memory leak in parser component\n" +
		"  2h ago\n" +
		"  https://github.com/AG2904/github-activity-feed/pull/42\n"
	assignedGrouped = "maria.perez assigned to lucia.gomez\n" +
		"  [bug] Ship the onboarding checklist\n" +
		"  2h ago\n" +
		"  https://github.com/AG2904/github-activity-feed/issues/7\n"
)

// cards joins rendered cards with the blank line the feed puts between them.
func cards(rendered ...string) string {
	return strings.Join(rendered, "\n")
}

// hasEscape reports whether a stream carries an ANSI escape: the chrome and the
// piped feed must never do, and every assertion of that goes through here.
func hasEscape(text string) bool {
	return strings.ContainsRune(text, '\x1b')
}

// connectedLines counts the affirmative connection lines on stderr: exactly one
// per session, the first accepted connection's.
func connectedLines(stderr string) int {
	return strings.Count(stderr, "connected · ")
}

func mergedFrame(when time.Time) string {
	return eventFrame(when, "pr.merged", "AG2904/github-activity-feed", "fix/memory-leak",
		"maria.perez", "Fix memory leak in parser component", nil, `["bug"]`,
		"https://github.com/AG2904/github-activity-feed/pull/42")
}

func assignedFrame(when time.Time) string {
	return eventFrame(when, "issue.assigned", "AG2904/github-activity-feed", "",
		"maria.perez", "Ship the onboarding checklist", ptr("lucia.gomez"), `["bug"]`,
		"https://github.com/AG2904/github-activity-feed/issues/7")
}

// eventFrame builds the relay's frame for one event. An empty repository or
// branch is the null of a row stored before that column existed; a nil assignee
// is null.
func eventFrame(when time.Time, kind, repository, branch, actor, subject string, assignee *string, labels, url string) string {
	repo := "null"
	if repository != "" {
		repo = fmt.Sprintf("%q", repository)
	}
	ref := "null"
	if branch != "" {
		ref = fmt.Sprintf("%q", branch)
	}
	assigned := "null"
	if assignee != nil {
		assigned = fmt.Sprintf("%q", *assignee)
	}

	return "data: " + fmt.Sprintf(
		`{"type":%q,"repository":%s,"branch":%s,"actor":%q,"subject":%q,"assignee":%s,"labels":%s,"url":%q,"when":"%s"}`+"\n\n",
		kind, repo, ref, actor, subject, assigned, labels, url, stamp(when),
	)
}

func payloadOf(frame string) string {
	return strings.TrimSuffix(strings.TrimPrefix(frame, "data: "), "\n\n")
}

// historyOf builds the relay's /history body out of the payloads a test would
// otherwise send on the stream: the same shape either way (story 18), so the
// backfill and the live feed are fed from identical bytes.
func historyOf(payloads ...string) string {
	return "[" + strings.Join(payloads, ",") + "]"
}

func ptr[T any](value T) *T {
	return &value
}

// stamp is the relay's canonical form: UTC, ISO-8601, milliseconds.
func stamp(when time.Time) string {
	return when.UTC().Format("2006-01-02T15:04:05.000Z")
}

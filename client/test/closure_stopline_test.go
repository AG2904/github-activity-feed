package test

import (
	"testing"
	"time"
)

// A session that showed only closures accounts for both outcomes and for none
// of the types it did not show. It pins the stop line against a counter being
// added without its zero, and against the two closures being folded together.
func TestWatchStoppedCountsClosuresSeparately(t *testing.T) {
	relay := newFakeRelay(t)
	feed, stdout := startFeedWithPipeStdout(t, "watch", "--relay", relay.URL, "--no-color")
	relay.waitConnected(t)

	when := time.Now().Add(-2*time.Hour - 30*time.Minute)
	relay.send(t, closedFrame(when))
	readUntil(t, stdout, closedOutput)
	relay.send(t, dismissedFrame(when))
	readUntil(t, stdout, dismissedOutput)

	endSession(t, feed, stdout, relay, mergedFrame(when))
	wantStopped(t, feed, "stopped · 2 events · 0 merged · 0 assigned · 1 closed · 1 dismissed\n")
}

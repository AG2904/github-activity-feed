package test

import (
	"strings"
	"testing"
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

	got := runFeed(t, map[string]string{"RELAY_URL": fromEnvironment.URL}, "watch", "--relay", fromFlag.URL)

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
	if hits := fromFlag.hits("/stream"); hits != 1 {
		t.Errorf("the --relay relay saw %d /stream requests, want 1", hits)
	}
	if seen := fromEnvironment.requests(); len(seen) != 0 {
		t.Errorf("RELAY_URL relay saw %v, want nothing", seen)
	}
	if !strings.Contains(got.stderr, "watching "+fromFlag.URL+"/stream") {
		t.Errorf("stderr = %q, want the watch banner naming %s", got.stderr, fromFlag.URL+"/stream")
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
}

func TestWatchUsesEnvironmentWhenNoFlagIsGiven(t *testing.T) {
	relay := newFakeRelay(t)

	got := runFeed(t, map[string]string{"RELAY_URL": relay.URL}, "watch")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", got.code, got.stderr)
	}
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

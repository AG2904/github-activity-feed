package cli

import (
	"testing"
	"time"
)

// TestBackoffDelay is the reconnection policy as a table: every step the
// doubling growth can produce, the jitter window each delay must land in, and
// the step the next attempt starts from.
func TestBackoffDelay(t *testing.T) {
	tests := []struct {
		name     string
		step     time.Duration // the step the backoff starts from
		wantMin  time.Duration // inclusive lower bound of every delay
		wantMax  time.Duration // exclusive upper bound of every delay
		wantNext time.Duration // the step after one delay was served
	}{
		{"zero value starts at the one-second base", 0,
			time.Second, time.Second + time.Second/4, 2 * time.Second},
		{"the base step doubles", time.Second,
			time.Second, time.Second + time.Second/4, 2 * time.Second},
		{"the second step doubles", 2 * time.Second,
			2 * time.Second, 2*time.Second + 2*time.Second/4, 4 * time.Second},
		{"the third step doubles", 4 * time.Second,
			4 * time.Second, 4*time.Second + 4*time.Second/4, 8 * time.Second},
		{"the fourth step doubles", 8 * time.Second,
			8 * time.Second, 8*time.Second + 8*time.Second/4, 16 * time.Second},
		{"the fifth step doubles but clamps at the ceiling", 16 * time.Second,
			16 * time.Second, 16*time.Second + 16*time.Second/4, maxBackoff},
		{"the ceiling step clamps and jitters below the ceiling", maxBackoff,
			maxBackoff - maxBackoff/4, maxBackoff, maxBackoff},
	}

	const samples = 128
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < samples; i++ {
				b := &backoff{step: tt.step}
				got := b.delay()
				if got < tt.wantMin || got >= tt.wantMax {
					t.Fatalf("delay() = %s, want in [%s, %s)", got, tt.wantMin, tt.wantMax)
				}
				if b.step != tt.wantNext {
					t.Fatalf("step after delay() = %s, want %s", b.step, tt.wantNext)
				}
			}
		})
	}
}

// TestBackoffCeilingJitterVaries pins the fix itself: at the ceiling every
// delay must be a different draw, not the deterministic maximum the clipped
// jitter used to produce where reconnect lockstep matters most.
func TestBackoffCeilingJitterVaries(t *testing.T) {
	seen := make(map[time.Duration]bool)
	const samples = 64
	for i := 0; i < samples; i++ {
		b := &backoff{step: maxBackoff}
		seen[b.delay()] = true
	}
	if len(seen) < 2 {
		t.Errorf("distinct ceiling delays = %d over %d draws, want jittered draws that are not all %s",
			len(seen), samples, maxBackoff)
	}
}

// TestBackoffReset returns to the one-second base after any step, ceiling
// included — the reset contract the reconnect loop relies on after an event.
func TestBackoffReset(t *testing.T) {
	b := &backoff{step: maxBackoff}
	b.reset()

	got := b.delay()
	if got < time.Second || got >= time.Second+time.Second/4 {
		t.Errorf("delay() after reset = %s, want in [%s, %s)", got, time.Second, time.Second+time.Second/4)
	}
}

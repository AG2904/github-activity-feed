package cli

import (
	"math/rand/v2"
	"time"
)

// The reconnection policy: how long to wait before trying again, and which
// failures are never retried. The two live together because a fatal error is
// the exception to the backoff, and reading them apart invites a call site that
// retries what was meant to end the command.
const (
	minBackoff = time.Second
	maxBackoff = 30 * time.Second
)

// fatalError marks a failure the CLI cannot retry: an unusable relay URL or a
// closed stdout. Connection failures are retryable by design; these are not.
type fatalError struct{ err error }

func (e *fatalError) Error() string { return e.err.Error() }
func (e *fatalError) Unwrap() error { return e.err }

// backoff grows exponentially from one second to a thirty-second ceiling. Each
// delay is jittered — upward by up to a quarter of the step below the ceiling,
// and within the quarter below the ceiling itself at the ceiling — so a relay
// restart does not gather every client in the same instant. The ceiling
// applies to the step; a reset returns to the one-second base.
type backoff struct {
	step time.Duration
}

func (b *backoff) reset() {
	b.step = 0
}

func (b *backoff) delay() time.Duration {
	step := b.step
	if step == 0 {
		step = minBackoff
	}

	if step > maxBackoff {
		step = maxBackoff
	}

	// Cap first, then jitter. Below the ceiling the delay is the step plus up
	// to a quarter of it; at the ceiling there is no headroom for an upward
	// draw, so the delay comes from the quarter below the cap —
	// [3/4·maxBackoff, maxBackoff) — instead of clipping to the
	// deterministic maximum, which is exactly where reconnect lockstep
	// matters most.
	var delay time.Duration
	if step >= maxBackoff {
		quarter := maxBackoff / 4
		delay = maxBackoff - quarter + time.Duration(rand.Int64N(int64(quarter)))
	} else {
		delay = step + time.Duration(rand.Int64N(int64(step/4)))
	}

	if next := step * 2; next < maxBackoff {
		b.step = next
	} else {
		b.step = maxBackoff
	}
	return delay
}

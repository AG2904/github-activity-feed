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
// delay is jittered upward by up to a quarter of the step, so a relay restart
// does not gather every client in the same instant. The ceiling applies to the
// step; a reset returns to the one-second base.
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

	delay := step + time.Duration(rand.Int64N(int64(step/4)))
	if delay > maxBackoff {
		delay = maxBackoff
	}

	if next := step * 2; next < maxBackoff {
		b.step = next
	} else {
		b.step = maxBackoff
	}
	return delay
}

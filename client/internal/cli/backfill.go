package cli

import (
	"context"
	"slices"
)

// backfill fills the terminal from the audit history, before the stream opens,
// so a developer can tell the CLI works before the next critical event happens.
// A limit of zero or less disables it and asks the relay nothing.
//
// It reports whether a history was read. A relay that will not answer is not a
// broken watch: the terminal stays as it is and the next connection asks again,
// which is how a CLI started before its relay still gets a backfill, and a
// history that was read is never asked for a second time. The one error it
// returns is a failure to write the feed, because a closed stdout ends the
// watch wherever it happens.
func backfill(ctx context.Context, url string, out emitter, limit int) (filled bool, err error) {
	if limit <= 0 {
		return true, nil
	}

	payloads, err := storedHistory(ctx, url)
	if err != nil {
		// The stream is what a `watch` was asked for; a history the relay will
		// not serve is a duller terminal, not a broken watch.
		return false, nil
	}

	// The relay serves its history newest first, so the head of the stream is
	// the first rows it offered.
	head := out.shown(payloads)
	if len(head) > limit {
		head = head[:limit]
	}
	// Printed oldest first, the live events carry on below it.
	slices.Reverse(head)
	for _, stored := range head {
		if err := out.paint(stored.payload, stored.event); err != nil {
			return true, err
		}
	}
	return true, nil
}

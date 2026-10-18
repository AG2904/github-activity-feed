package cli

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
)

// storedEvent is one row of the audit history: the relay's own bytes and the
// event they carry, so the filter and the renderer read the same thing the wire
// did.
type storedEvent struct {
	payload string
	event   activity.Event
}

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

	head := out.head(payloads, limit)
	// The relay serves its history newest first, and this is the head of a
	// stream: printed oldest first, the live events carry on below it.
	slices.Reverse(head)
	for _, stored := range head {
		if err := out.paint(stored.payload, stored.event); err != nil {
			return true, err
		}
	}
	return true, nil
}

// storedHistory reads the audit history once. The request carries no filter:
// filtering is local, exactly as it is on the stream (ADR-0001), so the CLI
// never narrows what the relay sends.
func storedHistory(ctx context.Context, url string) ([]json.RawMessage, error) {
	resp, err := fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Raw messages, so each stored event keeps the relay's bytes: re-marshalling
	// a parsed event would reorder its keys and reshape null and the millisecond
	// timestamp, which is the contract --json hands over untouched.
	var payloads []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&payloads); err != nil {
		return nil, err
	}
	return payloads, nil
}

// head keeps the newest stored rows this client shows, newest first, up to
// limit. It is the emitter's acceptance rule asked over rows instead of frames,
// so it lives beside the backfill that reads them.
//
// The filter runs before the limit, so `--backfill 10 --user me` shows the ten
// most recent events of mine rather than whichever of everyone's last ten
// happened to be mine. A row the relay stored but the CLI cannot read is
// dropped, the same stance the stream takes on a frame it cannot use.
func (e emitter) head(payloads []json.RawMessage, limit int) []storedEvent {
	head := make([]storedEvent, 0, min(limit, len(payloads)))
	for _, payload := range payloads {
		event, _, shown := e.accept(string(payload))
		if !shown {
			continue
		}
		head = append(head, storedEvent{payload: string(payload), event: event})
		if len(head) == limit {
			break
		}
	}
	return head
}

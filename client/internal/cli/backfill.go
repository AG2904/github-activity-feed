package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
)

// row is one stored event: the relay's own bytes and the event they carry, so
// the filter and the renderer read the same thing the wire did.
type row struct {
	payload string
	event   activity.Event
}

// backfill fills the terminal from the audit history on the first connection,
// so a developer can tell the CLI works before the next critical event happens.
// A limit of zero or less disables it and asks the relay nothing.
//
// It is quiet about its own failures. The stream is what a `watch` was asked
// for, so a history the relay will not serve is a duller terminal and not a
// broken watch — the same reason an empty history prints nothing and follows
// the stream. The one error it reports is a failure to write the feed, because
// a closed stdout ends the watch wherever it happens.
func backfill(ctx context.Context, url string, out emitter, limit int) error {
	if limit <= 0 {
		return nil
	}

	payloads, err := storedHistory(ctx, url)
	if err != nil {
		return nil
	}

	recent := mostRecent(payloads, out.filter, limit)
	// The relay serves the history newest first, and this is the head of a
	// stream: printed oldest first, the live events carry on below it.
	slices.Reverse(recent)
	for _, row := range recent {
		if err := out.paint(row.payload, row.event); err != nil {
			return err
		}
	}
	return nil
}

// storedHistory reads the audit history once. The request carries no filter:
// filtering is local, exactly as it is on the stream (ADR-0001), so the CLI
// never narrows what the relay sends.
func storedHistory(ctx context.Context, url string) ([]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", url, resp.Status)
	}

	// Raw messages, so each stored event keeps the relay's bytes: re-marshalling
	// a parsed event would reorder its keys and reshape null and the millisecond
	// timestamp, which is the contract --json hands over untouched.
	var payloads []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&payloads); err != nil {
		return nil, err
	}
	return payloads, nil
}

// mostRecent keeps the newest rows that pass the filter, newest first, up to
// limit. The filter runs before the limit, so `--backfill 10 --user me` shows
// the ten most recent events of mine rather than whatever of everyone's last
// ten happened to be mine. A row the relay stored but the CLI cannot read is
// dropped, the same stance the stream takes on a frame it cannot use.
func mostRecent(payloads []json.RawMessage, filter activity.Filter, limit int) []row {
	recent := make([]row, 0, min(limit, len(payloads)))
	for _, payload := range payloads {
		event, err := activity.Parse(payload)
		if err != nil || !filter.Match(event) {
			continue
		}
		recent = append(recent, row{payload: string(payload), event: event})
		if len(recent) == limit {
			break
		}
	}
	return recent
}

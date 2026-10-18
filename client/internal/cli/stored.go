package cli

import (
	"context"
	"encoding/json"
	"io"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
)

// storedEvent is one row of the audit history: the relay's own bytes and the
// event they carry, so the filter and the renderer read the same thing the wire
// did and --json can still hand the bytes over untouched.
type storedEvent struct {
	payload string
	event   activity.Event
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

	return storedRows(resp.Body)
}

// storedRows decodes the relay's history body into one raw message per event.
// Raw messages, so each stored event keeps the relay's bytes: re-marshalling a
// parsed event would reorder its keys and reshape null and the millisecond
// timestamp, which is the contract --json hands over untouched.
func storedRows(body io.Reader) ([]json.RawMessage, error) {
	var payloads []json.RawMessage
	if err := json.NewDecoder(body).Decode(&payloads); err != nil {
		return nil, err
	}
	return payloads, nil
}

// shown keeps the stored rows this client shows, in the order the relay served
// them. It is the emitter's acceptance rule asked over rows instead of frames,
// so the report and the live feed cannot disagree about --user, about --repo,
// or about a row the CLI cannot read.
func (e emitter) shown(payloads []json.RawMessage) []storedEvent {
	shown := make([]storedEvent, 0, len(payloads))
	for _, payload := range payloads {
		event, _, show := e.accept(string(payload))
		if !show {
			continue
		}
		shown = append(shown, storedEvent{payload: string(payload), event: event})
	}
	return shown
}

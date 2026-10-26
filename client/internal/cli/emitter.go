package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
	"github.com/AG2904/github-activity-feed/client/internal/render"
)

// emitter is the output policy in one value: what this client shows, and how it
// is written. It never decides what the relay sends; only what reaches stdout.
// It also remembers whether a card was already printed, because the blank line
// between cards belongs to the sequence and not to one card.
type emitter struct {
	out     io.Writer
	palette render.Palette
	json    bool
	filter  activity.Filter
	printed bool
	// The session's account of itself: what reached the reader, filter
	// included, split by the two types that carry a human word. The backfill
	// reads total as how much history printed; the stop line reads all three.
	total    int
	merged   int
	assigned int
}

// newEmitter builds the output policy both commands hand their events to: the
// same stdout, the same colour rule, the same filter, so `--no-color` and
// `--user` cannot come to mean different things in the two of them.
func newEmitter(stdout io.Writer, noColor bool, filter activity.Filter) *emitter {
	return &emitter{
		out:     stdout,
		palette: render.Palette{Color: colorEnabled(stdout, noColor)},
		filter:  filter,
	}
}

// accept parses one payload and answers what it is to this client: valid when
// the CLI could read it as an event at all — an event the filter drops is still
// valid, and that is what resets the backoff — and shown when this client shows
// it. It is the one place the acceptance rule is written down, so the live
// stream and the backfill cannot disagree about it.
func (e *emitter) accept(payload string) (event activity.Event, valid, shown bool) {
	event, err := activity.Parse([]byte(payload))
	if err != nil {
		return activity.Event{}, false, false
	}
	return event, true, e.filter.Match(event)
}

// shown applies accept over stored rows instead of live frames: the report and
// the live feed cannot disagree about --user, about --repo, or about a row the
// CLI cannot read. The rows keep the order the relay served them in.
func (e *emitter) shown(payloads []json.RawMessage) []storedEvent {
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

// paint writes one event of the feed. It is the only place the feed reaches
// stdout as feed lines, so the backfill and the live stream cannot disagree
// about --json, about colour, or about the rendered form. The blank line that
// separates cards is written here, before a card that is not the first, so the
// last event never leaves a gap under the prompt and `--json` never gets one.
func (e *emitter) paint(payload string, event activity.Event) error {
	if e.json {
		// Re-marshalling would reorder keys and could reshape null or the
		// millisecond timestamp.
		if _, err := io.WriteString(e.out, payload+"\n"); err != nil {
			return err
		}
		e.count(event)
		return nil
	}
	if e.printed {
		if _, err := io.WriteString(e.out, "\n"); err != nil {
			return err
		}
	}
	if err := e.palette.Event(e.out, event, time.Now()); err != nil {
		return err
	}
	e.count(event)
	e.printed = true
	return nil
}

// count records one event that reached the reader. Counting inside paint — the
// one place the backfill, the live stream and --json all cross into stdout —
// keeps a single account of what was shown; an event whose write failed is not
// counted, because nothing was shown of it.
func (e *emitter) count(event activity.Event) {
	e.total++
	switch event.Type {
	case activity.TypePRMerged:
		e.merged++
	case activity.TypeIssueAssigned:
		e.assigned++
	}
}

// summarize closes the session on stderr with one line of account: the events
// the reader saw, filter included, because the summary answers "what did I
// see", not "what did the relay send". watch() defers it, so every path out of
// the command — Ctrl-C, an unusable URL, a closed stdout — speaks once before
// going. It is plain text like the rest of the chrome: one sentence of state,
// no escape, never a heartbeat.
func (e *emitter) summarize(w io.Writer) {
	if e.total == 0 {
		fmt.Fprintln(w, "stopped · no events")
		return
	}
	fmt.Fprintf(w, "stopped · %d events · %d merged · %d assigned\n", e.total, e.merged, e.assigned)
}

// group writes the report shape: the same events, one section per repository.
// Raw output has no grouped form — the flags refuse the combination — so there
// is nothing to decide here, only to hand over, the same way paint hands over a
// feed line. Every byte the CLI puts on stdout starts here.
func (e *emitter) group(events []activity.Event, now time.Time) error {
	return e.palette.Grouped(e.out, events, now)
}

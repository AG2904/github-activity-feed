// Package render turns an ActivityEvent into the terminal's card: a sentence,
// the thing that happened, and its context. It knows nothing about flags:
// whether colour is allowed is decided by the shell and handed in as a boolean.
package render

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
)

// The palette carries roles, not decoration: two states, the people, the place,
// and the reason. Sixteen-colour codes, because the binary ships to terminals
// that are not Windows Terminal.
const (
	reset = "\x1b[0m"
	bold  = "\x1b[1m"
	grey  = "\x1b[90m"
	cyan  = "\x1b[1;36m"
	green = "\x1b[1;32m"
	amber = "\x1b[1;33m"
	blue  = "\x1b[34m"
	red   = "\x1b[1;31m"
)

// Palette renders one event per call. Color enables ANSI colour, bold, and OSC
// 8 hyperlinks; with it off, the output carries no escape sequence and the URL
// is printed on its own line so the subject is still reachable.
type Palette struct {
	Color bool
}

// lineShape says whether the card names the repository in its context line. The
// feed does; a grouped report does not, because its header already named it.
type lineShape bool

const (
	withRepository    lineShape = true
	withoutRepository lineShape = false
)

// Event writes one card of the feed.
func (p Palette) Event(w io.Writer, event activity.Event, now time.Time) error {
	return p.card(w, event, now, withRepository)
}

// card writes one event: the sentence, the title, the context, and — only when
// there is no hyperlink — the URL. shape decides whether the context line
// repeats the repository.
func (p Palette) card(w io.Writer, event activity.Event, now time.Time, shape lineShape) error {
	var card strings.Builder
	card.WriteString(p.headline(event))
	card.WriteString("\n  ")
	card.WriteString(p.title(event))
	card.WriteString("\n  ")
	card.WriteString(p.context(event, now, shape))
	card.WriteString("\n")
	if !p.Color {
		card.WriteString("  ")
		card.WriteString(event.URL)
		card.WriteString("\n")
	}

	_, err := io.WriteString(w, card.String())
	return err
}

// headline is the sentence: who acted, what the verb was, and where it landed.
// The destination is derived from the event's type because the event carries no
// branch; a type that means something else gets its own phrase rather than
// reusing this one. An unknown type degrades to its raw value and no
// destination, so a new event type still reads as something.
func (p Palette) headline(event activity.Event) string {
	parts := []string{p.paint(cyan, event.Actor)}
	switch event.Type {
	case "pr.merged":
		parts = append(parts,
			p.paint(green, "merged"),
			p.paint(grey, "into"),
			p.paint(green, "main"),
		)
	case "issue.assigned":
		parts = append(parts, p.paint(amber, "assigned"))
		if event.Assignee != nil && *event.Assignee != "" {
			parts = append(parts, p.paint(grey, "to"), p.paint(cyan, *event.Assignee))
		}
	default:
		parts = append(parts, p.paint(grey, event.Type))
	}
	return strings.Join(parts, " ")
}

// title is the thing itself, on its own line and bold so it does not compete
// with the palette. With colour it is also the hyperlink, which is why the URL
// line disappears.
func (p Palette) title(event activity.Event) string {
	if !p.Color {
		return event.Subject
	}
	return bold + "\x1b]8;;" + event.URL + "\x1b\\" + event.Subject + "\x1b]8;;\x1b\\" + reset
}

// context is the repository, the labels and the freshness, in the order a
// reader asks for them. A segment the event does not carry disappears instead
// of printing an empty one.
func (p Palette) context(event activity.Event, now time.Time, shape lineShape) string {
	parts := make([]string, 0, 3)
	if shape == withRepository && hasRepository(event) {
		parts = append(parts, p.paint(blue, *event.Repository))
	}
	if len(event.Labels) > 0 {
		parts = append(parts, p.paint(red, "["+strings.Join(event.Labels, ", ")+"]"))
	}
	parts = append(parts, p.paint(grey, relative(now, event.When)))
	return strings.Join(parts, p.paint(grey, " · "))
}

// paint wraps text in an escape only when colour is allowed. Every flat coloured
// segment goes through here, so a pipe cannot receive one by accident; the title
// builds its own because it has to wrap the hyperlink around the text.
func (p Palette) paint(code, text string) string {
	if !p.Color {
		return text
	}
	return code + text + reset
}

// hasRepository reports whether the event carries a repository the renderer can
// name: a row stored before the column existed does not (ADR-0010), and neither
// does an empty one the relay should never send.
func hasRepository(event activity.Event) bool {
	return event.Repository != nil && *event.Repository != ""
}

// relative keeps the boundaries a person uses: "just now", minutes, hours,
// yesterday, and days. The 24-to-48-hour window is an approximation of a
// calendar day, not a computation against local midnight.
func relative(now, when time.Time) string {
	elapsed := now.Sub(when)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed/time.Minute))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed/time.Hour))
	case elapsed < 48*time.Hour:
		return "yesterday"
	default:
		return fmt.Sprintf("%dd ago", int(elapsed/(24*time.Hour)))
	}
}

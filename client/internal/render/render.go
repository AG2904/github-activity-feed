// Package render turns an ActivityEvent into the terminal's card: a sentence,
// the reason with the thing that happened, and its context. It knows nothing
// about flags: whether colour is allowed is decided by the shell and handed in
// as a boolean.
package render

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
)

// The palette carries roles, not decoration: four states, the people, the place,
// and the reason. Every role is a regular weight; bold is structure and belongs
// to the title and the grouped heading. Grey is context: the repository, the
// connectors and the time. The branch carries the location blue, the states keep
// their own slot — green for the integration, yellow for the assignment, white
// for the resolved closure and red for the dismissed one — and the reason is
// magenta — warm without competing with the states. Sixteen-colour codes, because
// the binary ships to terminals that are not Windows Terminal.
const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	grey    = "\x1b[90m"
	cyan    = "\x1b[36m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	blue    = "\x1b[34m"
	magenta = "\x1b[35m"
	white   = "\x1b[37m"
	red     = "\x1b[31m"
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
		card.WriteString(clean(event.URL))
		card.WriteString("\n")
	}

	_, err := io.WriteString(w, card.String())
	return err
}

// headline is the sentence: who acted, what the verb was, and where it landed.
// A merge names the branch that landed when the event carries one; a null
// branch — a legacy row or a payload without a usable head ref — falls back to
// the type-derived phrase, because pr.merged still means a merge into main. A
// type that means something else gets its own phrase rather than reusing this
// one. An unknown type degrades to its raw value and no destination, so a new
// event type still reads as something.
func (p Palette) headline(event activity.Event) string {
	parts := []string{p.paint(cyan, clean(event.Actor))}
	switch event.Type {
	case activity.TypePRMerged:
		parts = append(parts, p.paint(green, "merged"))
		if event.Branch != nil && *event.Branch != "" {
			parts = append(parts, p.paint(blue, clean(*event.Branch)))
		}
		parts = append(parts,
			p.paint(grey, "into"),
			p.paint(green, "main"),
		)
	case activity.TypeIssueAssigned:
		parts = append(parts, p.paint(yellow, "assigned"))
		if event.Assignee != nil && *event.Assignee != "" {
			parts = append(parts, p.paint(grey, "to"), p.paint(cyan, clean(*event.Assignee)))
		}
	case activity.TypeIssueClosed:
		parts = append(parts, p.paint(white, "closed"))
	case activity.TypeIssueDismissed:
		// The two closures are different news, so the reason rides in the same
		// sentence instead of being left to the type the feed does not print.
		parts = append(parts, p.paint(red, "dismissed"), p.paint(grey, "as not planned"))
	default:
		parts = append(parts, p.paint(grey, clean(event.Type)))
	}
	return strings.Join(parts, " ")
}

// title is the reason and the thing itself: the labels the event carries come
// first, in brackets and one space apart, so the why arrives before the what; a
// card with no labels starts at the title. The subject stays bold so it does
// not compete with the palette, and with colour it is also the hyperlink, which
// is why the URL line disappears.
func (p Palette) title(event activity.Event) string {
	labels := ""
	if len(event.Labels) > 0 {
		labels = p.paint(magenta, "["+clean(strings.Join(event.Labels, ", "))+"]") + " "
	}
	if !p.Color {
		return labels + clean(event.Subject)
	}
	// The subject is concatenated into the hyperlink, so an ST byte inside it
	// would close the link the palette opens and retarget the rest; cleaning
	// both halves keeps the sequence the palette's own.
	return labels + bold + "\x1b]8;;" + clean(event.URL) + "\x1b\\" + clean(event.Subject) + "\x1b]8;;\x1b\\" + reset
}

// context is the repository and the freshness, in the order a reader asks for
// them. A segment the event does not carry disappears instead of printing an
// empty one; the labels are not here — they ride with the title.
func (p Palette) context(event activity.Event, now time.Time, shape lineShape) string {
	parts := make([]string, 0, 2)
	if shape == withRepository && hasRepository(event) {
		parts = append(parts, p.paint(grey, clean(*event.Repository)))
	}
	parts = append(parts, p.paint(grey, relative(now, event.When)))
	return strings.Join(parts, p.paint(grey, " · "))
}

// clean renders event text as displayable characters. Every field in a card
// comes from the relay, which stored it verbatim, and a terminal acts on C0,
// DEL and C1 bytes the program would otherwise never emit: without this the
// colour gate — which only decides which escapes the program adds — would be
// mistaken for a filter of the ones the event brings. A line break or a tab
// inside a field would also split the card's own layout, so they become a space
// instead of disappearing.
func clean(text string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return ' '
		case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
			return -1
		}
		return r
	}, text)
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

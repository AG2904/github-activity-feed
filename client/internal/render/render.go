// Package render turns an ActivityEvent into the two-line terminal form. It
// knows nothing about flags: whether colour is allowed is decided by the shell
// and handed in as a boolean.
package render

import (
	"fmt"
	"io"
	"time"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
)

const (
	reset = "\x1b[0m"
	bold  = "\x1b[1m"
)

// Palette renders one event per call. Color enables ANSI colour, bold, and OSC
// 8 hyperlinks; with it off, the output carries no escape sequence and the URL
// is printed raw so the subject is still reachable.
type Palette struct {
	Color bool
}

// Event writes the two lines: badge, assignee when there is one, and subject;
// then repository, actor, and relative time. The raw URL replaces the terminal
// hyperlink when escapes are off.
func (p Palette) Event(w io.Writer, event activity.Event, now time.Time) error {
	first := p.badge(event.Type)
	if event.Assignee != nil && *event.Assignee != "" {
		assignee := "@" + *event.Assignee
		if p.Color {
			assignee = bold + assignee + reset
		}
		first += " " + assignee
	}
	first += " " + p.subject(event.URL, event.Subject)

	repository := "unknown repository"
	if event.Repository != nil && *event.Repository != "" {
		repository = *event.Repository
	}
	second := repository + " · " + event.Actor + " · " + relative(now, event.When)
	if !p.Color {
		second += " · " + event.URL
	}

	_, err := fmt.Fprintf(w, "%s\n%s\n", first, second)
	return err
}

func (p Palette) badge(kind string) string {
	text := "[" + kind + "]"
	if !p.Color {
		return text
	}

	color := "\x1b[36m"
	switch kind {
	case "pr.merged":
		color = "\x1b[32m"
	case "issue.assigned":
		color = "\x1b[33m"
	}
	return color + text + reset
}

// subject carries an OSC 8 hyperlink to the event's URL. The empty OSC target
// at the end closes the link, so the rest of the line is not clickable.
func (p Palette) subject(url, subject string) string {
	if !p.Color {
		return subject
	}
	return "\x1b]8;;" + url + "\x1b\\" + subject + "\x1b]8;;\x1b\\"
}

func relative(now, when time.Time) string {
	elapsed := now.Sub(when)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed/time.Minute))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed/time.Hour))
	case elapsed < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(elapsed/(24*time.Hour)))
	default:
		return when.UTC().Format("2006-01-02")
	}
}

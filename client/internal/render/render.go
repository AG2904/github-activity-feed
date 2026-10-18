// Package render turns an ActivityEvent into the two-line terminal form. It
// knows nothing about flags: whether colour is allowed is decided by the shell
// and handed in as a boolean.
package render

import (
	"fmt"
	"io"
	"slices"
	"strings"
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

// Event writes the two lines of the feed: badge, assignee when there is one,
// and subject; then repository, actor, and relative time. The raw URL replaces
// the terminal hyperlink when escapes are off.
func (p Palette) Event(w io.Writer, event activity.Event, now time.Time) error {
	return p.line(w, event, now, true)
}

// Grouped writes the same feed as a per-repository report: one header per
// repository in alphabetical order, each with the events that happened in it in
// the order they were given, and the repository left out of the per-event lines
// because the header already named it. Groups are separated by a blank line.
//
// An event with no repository — a row stored before the column existed
// (ADR-0010) — has nothing to be grouped under and no name worth inventing for
// it, so it is written last, after a blank line, without a header: its second
// line already says everything there is to say about where it happened.
func (p Palette) Grouped(w io.Writer, events []activity.Event, now time.Time) error {
	groups, unnamed := byRepository(events)

	for i, group := range groups {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if err := p.heading(w, group.repository); err != nil {
			return err
		}
		for _, event := range group.events {
			if err := p.line(w, event, now, false); err != nil {
				return err
			}
		}
	}

	if len(unnamed) > 0 && len(groups) > 0 {
		fmt.Fprintln(w)
	}
	for _, event := range unnamed {
		if err := p.line(w, event, now, true); err != nil {
			return err
		}
	}
	return nil
}

// repositoryGroup is one section of a report: the repository the header names
// and the events that happened in it.
type repositoryGroup struct {
	repository string
	events     []activity.Event
}

// byRepository splits the events into one group per repository, alphabetically,
// keeping the order it was given inside each group. The events with no
// repository come back separately, in the same order.
func byRepository(events []activity.Event) ([]repositoryGroup, []activity.Event) {
	index := make(map[string]int)
	var groups []repositoryGroup
	var unnamed []activity.Event

	for _, event := range events {
		if event.Repository == nil || *event.Repository == "" {
			unnamed = append(unnamed, event)
			continue
		}
		at, seen := index[*event.Repository]
		if !seen {
			at = len(groups)
			index[*event.Repository] = at
			groups = append(groups, repositoryGroup{repository: *event.Repository})
		}
		groups[at].events = append(groups[at].events, event)
	}

	slices.SortFunc(groups, func(a, b repositoryGroup) int {
		return strings.Compare(a.repository, b.repository)
	})
	return groups, unnamed
}

// heading names a group. The header is the only structure a grouped report has,
// so it is the one line worth weighting when escapes are in use.
func (p Palette) heading(w io.Writer, repository string) error {
	if p.Color {
		repository = bold + repository + reset
	}
	_, err := fmt.Fprintln(w, repository)
	return err
}

// line writes one event's two lines. repository decides whether the second line
// repeats it: a grouped report has already named it in the header.
func (p Palette) line(w io.Writer, event activity.Event, now time.Time, repository bool) error {
	first := p.badge(event.Type)
	if event.Assignee != nil && *event.Assignee != "" {
		assignee := *event.Assignee
		if p.Color {
			assignee = bold + assignee + reset
		}
		first += " " + assignee
	}
	first += " " + p.subject(event.URL, event.Subject)

	// A nil repository is a row stored before the field existed (ADR-0010);
	// the segment disappears rather than inventing a name for it.
	second := make([]string, 0, 4)
	if repository && event.Repository != nil && *event.Repository != "" {
		second = append(second, *event.Repository)
	}
	second = append(second, event.Actor, relative(now, event.When))
	if !p.Color {
		second = append(second, event.URL)
	}

	_, err := fmt.Fprintf(w, "%s\n%s\n", first, strings.Join(second, " · "))
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
	default:
		return fmt.Sprintf("%dd ago", int(elapsed/(24*time.Hour)))
	}
}

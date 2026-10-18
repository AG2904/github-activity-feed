package render

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
)

// Grouped writes the feed as a per-repository report: one header per repository
// in alphabetical order, each with the events that happened in it in the order
// they were given, and the repository left out of the per-event lines because
// the header already named it. Groups are separated by a blank line.
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
			if err := p.line(w, event, now, withoutRepository); err != nil {
				return err
			}
		}
	}

	if len(unnamed) > 0 && len(groups) > 0 {
		fmt.Fprintln(w)
	}
	for _, event := range unnamed {
		if err := p.line(w, event, now, withRepository); err != nil {
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
		if !hasRepository(event) {
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

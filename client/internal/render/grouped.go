package render

import (
	"io"
	"slices"
	"strings"
	"time"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
)

// Grouped writes the feed as a per-repository report: one header per repository
// in alphabetical order, each with the events that happened in it in the order
// they were given, and the repository left out of the per-event cards because
// the header already named it. A blank line separates the header from its first
// card, each card from the next, and one group from another.
//
// This is the one place that owns its own spacing: a report is a whole document
// rather than a sequence of cards, so the emitter that separates feed cards has
// nothing to interleave here.
//
// An event with no repository — a row stored before the column existed
// (ADR-0010) — has nothing to be grouped under and no name worth inventing for
// it, so it is written last, without a header: its context line already says
// everything there is to say about where it happened.
func (p Palette) Grouped(w io.Writer, events []activity.Event, now time.Time) error {
	groups, unnamed := byRepository(events)

	for i, group := range groups {
		if i > 0 {
			if err := blank(w); err != nil {
				return err
			}
		}
		if err := p.heading(w, group.repository); err != nil {
			return err
		}
		if err := blank(w); err != nil {
			return err
		}
		for j, event := range group.events {
			if j > 0 {
				if err := blank(w); err != nil {
					return err
				}
			}
			if err := p.card(w, event, now, withoutRepository); err != nil {
				return err
			}
		}
	}

	if len(unnamed) > 0 && len(groups) > 0 {
		if err := blank(w); err != nil {
			return err
		}
	}
	for i, event := range unnamed {
		if i > 0 {
			if err := blank(w); err != nil {
				return err
			}
		}
		if err := p.card(w, event, now, withRepository); err != nil {
			return err
		}
	}
	return nil
}

// blank writes the empty line a report uses to separate its parts, and reports
// a closed stdout like every other write does.
func blank(w io.Writer) error {
	_, err := io.WriteString(w, "\n")
	return err
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
// so it is the one line worth weighting when escapes are in use: bold for the
// structure and blue, the location role, as the branch carries it in the feed.
func (p Palette) heading(w io.Writer, repository string) error {
	repository = clean(repository)
	if p.Color {
		repository = p.paint(bold+blue, repository)
	}
	_, err := io.WriteString(w, repository+"\n")
	return err
}

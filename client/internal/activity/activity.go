// Package activity holds the normalized ActivityEvent the relay emits. It
// knows nothing about SSE, flags, or the terminal: the shape is the contract
// between the two contexts, so it lives on its own.
package activity

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// The event types the relay's contract defines: the wire vocabulary every part
// of the client switches on. The human words ("merged", "assigned") belong to
// the renderer; these belong to the domain, so a type added to the contract is
// named once here instead of hunted down as a bare string across packages.
const (
	// TypePRMerged is a pull request merged into main.
	TypePRMerged = "pr.merged"
	// TypeIssueAssigned is an issue gaining an assignee.
	TypeIssueAssigned = "issue.assigned"
)

// Event is one ActivityEvent from the relay, with the timestamp already
// parsed. Repository, Branch and Assignee are pointers because the contract
// makes them nullable, each for its own reason: Repository and Branch can be
// null on a row stored before the column existed; Branch is also null for an
// event with no branch to name (every issue.assigned event, and a merge whose
// payload carried no usable head ref); Assignee is null for every pr.merged
// event.
type Event struct {
	Type       string
	Repository *string
	Branch     *string
	Actor      string
	Subject    string
	Assignee   *string
	Labels     []string
	URL        string
	When       time.Time
}

// Filter narrows the relay's broadcast for one client. The relay sends every
// event to everyone (ADR-0001), so this is where a developer's own events are
// separated from the rest.
type Filter struct {
	User       string
	Repository string
}

// Match applies the same rule the relay's /history?user= applies: involvement
// is actor or assignee, so the two never disagree about what a user's events
// are. The match is exact and case-sensitive, and a nil repository matches no
// repository filter (ADR-0010).
func (f Filter) Match(event Event) bool {
	if f.User != "" && event.Actor != f.User {
		if event.Assignee == nil || *event.Assignee != f.User {
			return false
		}
	}
	if f.Repository != "" {
		if event.Repository == nil || *event.Repository != f.Repository {
			return false
		}
	}
	return true
}

// Parse turns one frame's data payload into an Event. A payload that is not
// JSON, or that is missing a field the contract always carries, is rejected
// rather than rendered as a half event.
func Parse(payload []byte) (Event, error) {
	var wire struct {
		Type       string   `json:"type"`
		Repository *string  `json:"repository"`
		Branch     *string  `json:"branch"`
		Actor      string   `json:"actor"`
		Subject    string   `json:"subject"`
		Assignee   *string  `json:"assignee"`
		Labels     []string `json:"labels"`
		URL        string   `json:"url"`
		When       string   `json:"when"`
	}
	if err := json.Unmarshal(payload, &wire); err != nil {
		return Event{}, fmt.Errorf("activity: %w", err)
	}
	if wire.Type == "" || wire.Actor == "" || wire.Subject == "" || wire.URL == "" || wire.When == "" {
		return Event{}, errors.New("activity: missing a field the contract always carries")
	}

	when, err := time.Parse(time.RFC3339, wire.When)
	if err != nil {
		return Event{}, fmt.Errorf("activity: when: %w", err)
	}

	return Event{
		Type:       wire.Type,
		Repository: wire.Repository,
		Branch:     wire.Branch,
		Actor:      wire.Actor,
		Subject:    wire.Subject,
		Assignee:   wire.Assignee,
		Labels:     wire.Labels,
		URL:        wire.URL,
		When:       when,
	}, nil
}

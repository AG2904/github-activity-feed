package activity

// In-package unit tests for the wire contract: Parse must accept exactly the
// ActivityEvents the relay emits and reject everything else, and Filter must
// apply the same involvement rule the relay's /history endpoint applies.

import (
	"strings"
	"testing"
	"time"
)

// rawPayload is the relay's own bytes for a pr.merged ActivityEvent, with an
// ampersand Go's encoder would escape and a fractional-second timestamp — the
// same shape client/test pins end to end.
const rawPayload = `{"type":"pr.merged","repository":"AG2904/github-activity-feed","branch":"fix/memory-leak","actor":"maria.perez","subject":"Fix parser & renderer","assignee":null,"labels":[],"url":"https://github.com/AG2904/github-activity-feed/pull/42","when":"2026-10-11T13:00:00.000Z"}`

// issuePayload is the issue.assigned shape: null branch, a non-null assignee,
// and a label.
const issuePayload = `{"type":"issue.assigned","repository":"AG2904/github-activity-feed","branch":null,"actor":"maria.perez","subject":"Ship the onboarding checklist","assignee":"lucia.gomez","labels":["bug"],"url":"https://github.com/AG2904/github-activity-feed/issues/7","when":"2026-10-11T14:00:00Z"}`

// The hostile payloads client/test/security_test.go pins by hand: stored rows
// whose text carries terminal control bytes the relay preserved verbatim. The
// JSON uses the escape forms (written as Go \\u001b) because a raw ESC is not
// legal inside a JSON string; decoding restores the real bytes.
const (
	hostilePayload     = "{\"type\":\"pr.merged\",\"repository\":\"AG2904/widgets\",\"branch\":\"fix/tidy\\u001b[2K\\r\\u001b[1A\",\"actor\":\"mallory\",\"subject\":\"Tidy \\u001b]8;;https://evil.example/\\u001b\\\\ now\",\"assignee\":null,\"labels\":[\"bug\"],\"url\":\"https://github.com/AG2904/widgets/pull/7\",\"when\":\"2026-10-11T13:00:00.000Z\"}"
	hostileRepoPayload = "{\"type\":\"pr.merged\",\"repository\":\"AG2904/widgets\\u001b[2J\",\"branch\":\"fix/tidy\",\"actor\":\"mallory\",\"subject\":\"Tidy the docs\",\"assignee\":null,\"labels\":[\"bug\"],\"url\":\"https://github.com/AG2904/widgets/pull/8\",\"when\":\"2026-10-11T13:00:00.000Z\"}"
)

// withoutField removes one JSON member from the valid payload, so each
// missing-field case names exactly the field it drops.
func withoutField(field string) string {
	return strings.Replace(rawPayload, field, "", 1)
}

// str returns a pointer to s, for the nullable contract fields of Event.
func str(s string) *string {
	return &s
}

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		give string
		// wantOK says the payload must be accepted; when it is, want is the
		// exact ActivityEvent it must decode to.
		wantOK bool
		want   Event
		// wantErr is a substring the error must carry; empty means only that
		// an error is required, with no wording pinned.
		wantErr string
	}{
		{
			name:   "valid pr.merged payload",
			give:   rawPayload,
			wantOK: true,
			want: Event{
				Type:       TypePRMerged,
				Repository: str("AG2904/github-activity-feed"),
				Branch:     str("fix/memory-leak"),
				Actor:      "maria.perez",
				Subject:    "Fix parser & renderer",
				Assignee:   nil,
				Labels:     []string{},
				URL:        "https://github.com/AG2904/github-activity-feed/pull/42",
				When:       time.Date(2026, 10, 11, 13, 0, 0, 0, time.UTC),
			},
		},
		{
			name:   "valid issue.assigned payload with null branch",
			give:   issuePayload,
			wantOK: true,
			want: Event{
				Type:       TypeIssueAssigned,
				Repository: str("AG2904/github-activity-feed"),
				Branch:     nil,
				Actor:      "maria.perez",
				Subject:    "Ship the onboarding checklist",
				Assignee:   str("lucia.gomez"),
				Labels:     []string{"bug"},
				URL:        "https://github.com/AG2904/github-activity-feed/issues/7",
				When:       time.Date(2026, 10, 11, 14, 0, 0, 0, time.UTC),
			},
		},
		{
			name:    "payload that is not json",
			give:    "data: this is not json",
			wantOK:  false,
			wantErr: "activity:",
		},
		{
			name:    "truncated json",
			give:    `{"type":"pr.merged"`,
			wantOK:  false,
			wantErr: "activity:",
		},
		{
			name:    "empty payload",
			give:    "",
			wantOK:  false,
			wantErr: "activity:",
		},
		{
			name:    "missing type",
			give:    withoutField(`"type":"pr.merged",`),
			wantOK:  false,
			wantErr: "missing a field the contract always carries",
		},
		{
			name:    "missing actor",
			give:    withoutField(`"actor":"maria.perez",`),
			wantOK:  false,
			wantErr: "missing a field the contract always carries",
		},
		{
			name:    "missing subject",
			give:    withoutField(`"subject":"Fix parser & renderer",`),
			wantOK:  false,
			wantErr: "missing a field the contract always carries",
		},
		{
			name:    "missing url",
			give:    withoutField(`"url":"https://github.com/AG2904/github-activity-feed/pull/42",`),
			wantOK:  false,
			wantErr: "missing a field the contract always carries",
		},
		{
			// when is the last member, so the leading comma goes with it.
			name:    "missing when",
			give:    withoutField(`,"when":"2026-10-11T13:00:00.000Z"`),
			wantOK:  false,
			wantErr: "missing a field the contract always carries",
		},
		{
			name: "when is not an rfc3339 timestamp",
			give: strings.Replace(rawPayload,
				`"when":"2026-10-11T13:00:00.000Z"`, `"when":"yesterday"`, 1),
			wantOK:  false,
			wantErr: "activity: when:",
		},
		{
			name:    "json array is not an event",
			give:    `[{"type":"pr.merged"}]`,
			wantOK:  false,
			wantErr: "activity:",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.give))
			if (err == nil) != tt.wantOK {
				t.Fatalf("Parse(%q) error = %v, wantOK %v", tt.give, err, tt.wantOK)
			}
			if err != nil {
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Parse(%q) error = %q, want it to contain %q", tt.give, err, tt.wantErr)
				}
				return
			}
			wantEvent(t, tt.give, got, tt.want)
		})
	}
}

// wantEvent compares a decoded Event field by field so a mismatch names the
// field; When compares with Equal because the wall clock representation of a
// parsed timestamp is an implementation detail.
func wantEvent(t *testing.T, payload string, got, want Event) {
	t.Helper()
	if got.Type != want.Type {
		t.Errorf("Parse(%q) Type = %q, want %q", payload, got.Type, want.Type)
	}
	if !sameStringPtr(got.Repository, want.Repository) {
		t.Errorf("Parse(%q) Repository = %v, want %v", payload, got.Repository, want.Repository)
	}
	if !sameStringPtr(got.Branch, want.Branch) {
		t.Errorf("Parse(%q) Branch = %v, want %v", payload, got.Branch, want.Branch)
	}
	if got.Actor != want.Actor {
		t.Errorf("Parse(%q) Actor = %q, want %q", payload, got.Actor, want.Actor)
	}
	if got.Subject != want.Subject {
		t.Errorf("Parse(%q) Subject = %q, want %q", payload, got.Subject, want.Subject)
	}
	if !sameStringPtr(got.Assignee, want.Assignee) {
		t.Errorf("Parse(%q) Assignee = %v, want %v", payload, got.Assignee, want.Assignee)
	}
	if len(got.Labels) != len(want.Labels) {
		t.Errorf("Parse(%q) Labels = %v, want %v", payload, got.Labels, want.Labels)
	} else {
		for i := range want.Labels {
			if got.Labels[i] != want.Labels[i] {
				t.Errorf("Parse(%q) Labels = %v, want %v", payload, got.Labels, want.Labels)
				break
			}
		}
	}
	if got.URL != want.URL {
		t.Errorf("Parse(%q) URL = %q, want %q", payload, got.URL, want.URL)
	}
	if !got.When.Equal(want.When) {
		t.Errorf("Parse(%q) When = %v, want %v", payload, got.When, want.When)
	}
}

// sameStringPtr compares nullable contract fields, treating two nil pointers as
// equal and dereferencing otherwise.
func sameStringPtr(got, want *string) bool {
	if got == nil || want == nil {
		return got == want
	}
	return *got == *want
}

func TestFilterMatch(t *testing.T) {
	// A pr.merged event naming its repository and no assignee.
	merged := Event{
		Type:       TypePRMerged,
		Repository: str("AG2904/widgets"),
		Branch:     str("fix/tidy"),
		Actor:      "maria.perez",
		Subject:    "Tidy the docs",
		URL:        "https://github.com/AG2904/widgets/pull/7",
		When:       time.Date(2026, 10, 11, 13, 0, 0, 0, time.UTC),
	}
	// An issue.assigned event: null repository, an assignee who is not the actor.
	assigned := Event{
		Type:     TypeIssueAssigned,
		Actor:    "ci-bot",
		Subject:  "Ship the checklist",
		Assignee: str("lucia.gomez"),
		URL:      "https://github.com/AG2904/widgets/issues/7",
		When:     time.Date(2026, 10, 11, 14, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name   string
		filter Filter
		event  Event
		want   bool
	}{
		{
			name:   "empty filter passes every event",
			filter: Filter{},
			event:  Event{Actor: "anyone", Subject: "anything"},
			want:   true,
		},
		{
			name:   "user matches the actor",
			filter: Filter{User: "maria.perez"},
			event:  merged,
			want:   true,
		},
		{
			name:   "user matches the assignee when the actor differs",
			filter: Filter{User: "lucia.gomez"},
			event:  assigned,
			want:   true,
		},
		{
			name:   "user matching neither actor nor assignee fails",
			filter: Filter{User: "carlos.rios"},
			event:  merged,
			want:   false,
		},
		{
			name:   "user filter against a null assignee never matches",
			filter: Filter{User: "lucia.gomez"},
			event:  merged,
			want:   false,
		},
		{
			name:   "user match is case sensitive",
			filter: Filter{User: "MARIA.PEREZ"},
			event:  merged,
			want:   false,
		},
		{
			name:   "repository matches",
			filter: Filter{Repository: "AG2904/widgets"},
			event:  merged,
			want:   true,
		},
		{
			name:   "repository mismatch fails",
			filter: Filter{Repository: "AG2904/other"},
			event:  merged,
			want:   false,
		},
		{
			name:   "repository filter never matches a null repository",
			filter: Filter{Repository: "AG2904/widgets"},
			event:  assigned,
			want:   false,
		},
		{
			name:   "user and repository must both match",
			filter: Filter{User: "maria.perez", Repository: "AG2904/other"},
			event:  merged,
			want:   false,
		},
		{
			name:   "user and repository both matching passes",
			filter: Filter{User: "maria.perez", Repository: "AG2904/widgets"},
			event:  merged,
			want:   true,
		},
		{
			name:   "assignee match still requires the repository to match",
			filter: Filter{User: "lucia.gomez", Repository: "AG2904/widgets"},
			event:  assigned,
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(tt.event); got != tt.want {
				t.Fatalf("Filter{%q, %q}.Match(%+v) = %v, want %v",
					tt.filter.User, tt.filter.Repository, tt.event, got, tt.want)
			}
		})
	}
}

func FuzzParse(f *testing.F) {
	// Seeds: the two valid ActivityEvent shapes; the payload with each
	// required field dropped; the junk shapes the e2e suite pins
	// (client/test/e2e_test.go, TestWatchDiscardsFramesItCannotUse…); and the
	// hostile payloads whose text carries terminal control bytes
	// (client/test/security_test.go). The hostile payloads use the JSON
	// escape forms because a raw ESC is not legal inside a JSON string.
	seeds := []string{
		rawPayload,   // valid pr.merged
		issuePayload, // valid issue.assigned, null branch
		withoutField(`"type":"pr.merged",`),
		withoutField(`"repository":"AG2904/github-activity-feed",`),
		withoutField(`"branch":"fix/memory-leak",`),
		withoutField(`"actor":"maria.perez",`),
		withoutField(`"subject":"Fix parser & renderer",`),
		withoutField(`"assignee":null,`),
		withoutField(`"url":"https://github.com/AG2904/github-activity-feed/pull/42",`),
		withoutField(`,"when":"2026-10-11T13:00:00.000Z"`),
		`data: this is not json`,                  // the junk data frame from the e2e suite
		`{"type":"pr.merged"`,                     // truncated frame
		`{"type":"pr.merged","when":"yesterday"}`, // non-rfc3339 timestamp
		hostilePayload,           // control bytes in the branch and subject
		hostileRepoPayload,       // control bytes in the repository name
		`[{"type":"pr.merged"}]`, // a history array, not an event
		`null`,                   // legal json, no event inside
		`{"when":123}`,           // wrong type on a member
		``,                       // empty stream frame
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, payload []byte) {
		event, err := Parse(payload)
		if err != nil {
			if !strings.HasPrefix(err.Error(), "activity: ") {
				t.Fatalf("Parse(%q) error = %q, want the activity: prefix", payload, err)
			}
			return
		}
		// Accepted events carry every field the contract always carries, and
		// the timestamp round-trips through the wire format.
		if event.Type == "" || event.Actor == "" || event.Subject == "" || event.URL == "" {
			t.Fatalf("Parse(%q) accepted an event with an empty required field: %+v", payload, event)
		}
		if _, err := time.Parse(time.RFC3339, event.When.Format(time.RFC3339Nano)); err != nil {
			t.Fatalf("Parse(%q) returned a When that does not re-parse: %v", payload, err)
		}
	})
}

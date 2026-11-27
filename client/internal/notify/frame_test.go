package notify

// Unit tests for the two decisions that cannot be observed from a pipe: which
// protocol the terminal is known to speak, and the exact frame that leaves the
// program. A byte written to a console is invisible to the e2e suite, so the
// shape of the frame is pinned here instead (ADR-0017).

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
)

// TestDetectReadsOnlyWhatTheTerminalPublishes pins the environment-only choice.
// No case probes: the lookup is the whole input, so a terminal the environment
// does not identify gets no protocol at all.
func TestDetectReadsOnlyWhatTheTerminalPublishes(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Protocol
	}{
		{
			name: "kitty publishes its window id",
			env:  map[string]string{"KITTY_WINDOW_ID": "3"},
			want: OSC99,
		},
		{
			name: "kitty also publishes its term name",
			env:  map[string]string{"TERM": "xterm-kitty"},
			want: OSC99,
		},
		{
			name: "windows terminal publishes a session",
			env:  map[string]string{"WT_SESSION": "6f7c"},
			want: OSC777,
		},
		{
			name: "iterm2 names itself",
			env:  map[string]string{"TERM_PROGRAM": "iTerm.app"},
			want: OSC9,
		},
		{
			name: "wezterm names itself",
			env:  map[string]string{"TERM_PROGRAM": "WezTerm"},
			want: OSC9,
		},
		{
			name: "ghostty names itself",
			env:  map[string]string{"TERM_PROGRAM": "ghostty"},
			want: OSC9,
		},
		{
			name: "ghostty also publishes its term name",
			env:  map[string]string{"TERM": "xterm-ghostty"},
			want: OSC9,
		},
		{
			name: "apple terminal is not a notifier",
			env:  map[string]string{"TERM_PROGRAM": "Apple_Terminal"},
			want: NoProtocol,
		},
		{
			name: "a plain xterm is not a notifier",
			env:  map[string]string{"TERM": "xterm-256color"},
			want: NoProtocol,
		},
		{
			name: "an empty environment has no opinion",
			env:  map[string]string{},
			want: NoProtocol,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(name string) string { return tt.env[name] }
			if got := Detect(lookup); got != tt.want {
				t.Fatalf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFrameShape pins the contract the security argument rests on: exactly one
// introducer, exactly one terminator, and no control byte inside a field. It
// runs for every protocol, because the property belongs to the frame and not to
// the escape the frame happens to start with.
func TestFrameShape(t *testing.T) {
	event := activity.Event{
		Type:       activity.TypePRMerged,
		Actor:      "maria.perez",
		Branch:     ptrTo("fix/memory-leak"),
		Subject:    "Tidy the parser",
		Repository: ptrTo("AG2904/github-activity-feed"),
		URL:        "https://github.com/AG2904/github-activity-feed/pull/42",
	}

	for _, protocol := range []Protocol{OSC9, OSC777, OSC99} {
		t.Run(protocolName(protocol), func(t *testing.T) {
			frame := Frame(protocol, event)

			if got := strings.Count(string(frame), "\x1b"); got != 1 {
				t.Fatalf("introducers = %d, want exactly 1: %q", got, frame)
			}
			if got := strings.Count(string(frame), "\a"); got != 1 {
				t.Fatalf("terminators = %d, want exactly 1: %q", got, frame)
			}
			if !strings.HasPrefix(string(frame), "\x1b]") {
				t.Fatalf("frame %q does not start with the OSC introducer", frame)
			}
			if !strings.HasSuffix(string(frame), "\a") {
				t.Fatalf("frame %q does not end with the terminator", frame)
			}
			for _, field := range frameFields(t, frame) {
				if hasControlByte(field) {
					t.Errorf("field %q carries a control byte", field)
				}
			}
		})
	}
}

// TestFrameNeverLetsEventTextStartASequence is the injection case: text that is
// itself a complete escape sequence must arrive as inert characters, so the one
// introducer in the frame is always the program's own.
func TestFrameNeverLetsEventTextStartASequence(t *testing.T) {
	hostile := "\x1b]9;owned\x07\x1b[2K\x9bé"
	event := activity.Event{
		Type:       "issue.assigned",
		Actor:      hostile,
		Assignee:   ptrTo(hostile),
		Subject:    hostile,
		Repository: ptrTo(hostile),
		URL:        "https://github.com/AG2904/github-activity-feed/issues/1",
	}

	for _, protocol := range []Protocol{OSC9, OSC777, OSC99} {
		t.Run(protocolName(protocol), func(t *testing.T) {
			frame := Frame(protocol, event)

			if got := strings.Count(string(frame), "\x1b"); got != 1 {
				t.Fatalf("introducers = %d, want exactly 1: %q", got, frame)
			}
			if got := strings.Count(string(frame), "\a"); got != 1 {
				t.Fatalf("terminators = %d, want exactly 1: %q", got, frame)
			}
			for _, field := range frameFields(t, frame) {
				if hasControlByte(field) {
					t.Errorf("field %q carries a control byte", field)
				}
				if strings.ContainsRune(field, ';') {
					t.Errorf("field %q carries the program's separator", field)
				}
			}
		})
	}
}

// TestFrameTurnsTheSemicolonIntoASpace pins the one rewrite OSC 777 needs: its
// separator cannot be produced by a subject, so title and body keep their
// meaning whatever the event says.
func TestFrameTurnsTheSemicolonIntoASpace(t *testing.T) {
	event := activity.Event{
		Type:       activity.TypeIssueClosed,
		Actor:      "lucia.gomez",
		Subject:    "break; the; fields",
		Repository: ptrTo("AG2904/github-activity-feed"),
	}

	frame := string(Frame(OSC777, event))
	fields := frameFields(t, []byte(frame))
	if len(fields) != 2 {
		t.Fatalf("fields = %q, want exactly a title and a body", fields)
	}
	for _, field := range fields {
		if strings.ContainsRune(field, ';') {
			t.Errorf("field %q carries a semicolon", field)
		}
	}
	if fields[1] != "break  the  fields · AG2904/github-activity-feed" {
		t.Errorf("body = %q, want the semicolons rendered as spaces", fields[1])
	}
}

// TestFrameClampsTitleAndBody keeps a notification daemon from receiving a
// scrollback: 64 runes for the title, 200 for the body, and never a broken rune
// at the cut.
func TestFrameClampsTitleAndBody(t *testing.T) {
	long := strings.Repeat("ñ", 500)
	event := activity.Event{
		Type:       "release.published",
		Actor:      long,
		Subject:    long,
		Repository: ptrTo(long),
	}

	frame := Frame(OSC777, event)
	if !utf8.Valid(frame) {
		t.Fatal("frame is not valid UTF-8: a rune was cut in half")
	}
	fields := frameFields(t, frame)
	if len(fields) != 2 {
		t.Fatalf("fields = %q, want exactly a title and a body", fields)
	}
	if got := utf8.RuneCountInString(fields[0]); got != titleLimit {
		t.Errorf("title runes = %d, want %d", got, titleLimit)
	}
	if got := utf8.RuneCountInString(fields[1]); got != bodyLimit {
		t.Errorf("body runes = %d, want %d", got, bodyLimit)
	}
}

// TestFrameDropsBidiOverrides pins the class clean predates: a bidi override
// reverses what a toast shows without changing a visible byte.
func TestFrameDropsBidiOverrides(t *testing.T) {
	event := activity.Event{
		Type:    activity.TypeIssueClosed,
		Actor:   "lucia.gomez",
		Subject: "safe\u202eevil\u2066text",
	}

	frame := string(Frame(OSC777, event))
	for _, r := range "\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069" {
		if strings.ContainsRune(frame, r) {
			t.Errorf("frame carries the bidi override %U", r)
		}
	}
	if !strings.Contains(frame, "safeeviltext") {
		t.Errorf("frame %q lost the visible text around the overrides", frame)
	}
}

// TestFrameNeverCarriesTheURL pins the boundary: a ping says what happened, and
// the feed remains the place where a link is clickable.
func TestFrameNeverCarriesTheURL(t *testing.T) {
	const url = "https://github.com/AG2904/github-activity-feed/pull/42"
	event := activity.Event{
		Type:       activity.TypePRMerged,
		Actor:      "maria.perez",
		Branch:     ptrTo("fix/memory-leak"),
		Subject:    "Tidy the parser",
		Repository: ptrTo("AG2904/github-activity-feed"),
		URL:        url,
	}

	for _, protocol := range []Protocol{OSC9, OSC777, OSC99} {
		t.Run(protocolName(protocol), func(t *testing.T) {
			frame := string(Frame(protocol, event))
			if strings.Contains(frame, url) {
				t.Errorf("frame %q carries the URL", frame)
			}
			if strings.Contains(frame, "\x1b]8;;") {
				t.Errorf("frame %q carries a hyperlink", frame)
			}
		})
	}
}

// TestFrameTextIsTheSentenceAndTheSubject pins the wording: the plain sentence
// from the renderer, plus the subject and the repository, so the ping and the
// card cannot describe one event differently.
func TestFrameTextIsTheSentenceAndTheSubject(t *testing.T) {
	event := activity.Event{
		Type:       activity.TypePRMerged,
		Actor:      "maria.perez",
		Branch:     ptrTo("fix/memory-leak"),
		Subject:    "Tidy the parser",
		Repository: ptrTo("AG2904/github-activity-feed"),
	}

	fields := frameFields(t, Frame(OSC777, event))
	if len(fields) != 2 {
		t.Fatalf("fields = %q, want exactly a title and a body", fields)
	}
	if want := "maria.perez merged fix/memory-leak into main"; fields[0] != want {
		t.Errorf("title = %q, want %q", fields[0], want)
	}
	if want := "Tidy the parser · AG2904/github-activity-feed"; fields[1] != want {
		t.Errorf("body = %q, want %q", fields[1], want)
	}
}

// TestUnidentifiedTerminalGetsTheWidestFrame pins the explicit flag's promise:
// `osc` asked for a frame, so an environment the detector does not recognize
// still gets the sequence the widest set of terminals understands. The ones that
// do not understand it drop it, which is the failure mode the ADR accepts.
func TestUnidentifiedTerminalGetsTheWidestFrame(t *testing.T) {
	event := activity.Event{Type: activity.TypeIssueClosed, Actor: "lucia.gomez", Subject: "done"}

	frame := string(Frame(NoProtocol, event))
	if !strings.HasPrefix(frame, "\x1b]9;") {
		t.Fatalf("Frame() = %q, want the OSC 9 fallback", frame)
	}
	if got := strings.Count(frame, "\x1b"); got != 1 {
		t.Fatalf("introducers = %d, want exactly 1: %q", got, frame)
	}
}

// TestFrameOmitsTheSeparatorWhenTheBodyIsEmpty keeps the single-field protocols
// from ending in a dangling separator when the event carries no subject.
func TestFrameOmitsTheSeparatorWhenTheBodyIsEmpty(t *testing.T) {
	event := activity.Event{Type: "release.published", Actor: "maria.perez"}

	frame := string(Frame(OSC9, event))
	if want := "\x1b]9;maria.perez release.published\a"; frame != want {
		t.Fatalf("Frame() = %q, want %q", frame, want)
	}
}

// frameFields extracts the variable fields of a frame so a test can assert what
// a terminal would read: the program's literals are stripped, and only what came
// from the event remains.
func frameFields(t *testing.T, frame []byte) []string {
	t.Helper()

	body := strings.TrimSuffix(strings.TrimPrefix(string(frame), "\x1b]"), "\a")
	parts := strings.Split(body, ";")
	switch {
	case strings.HasPrefix(string(frame), "\x1b]9;"):
		return parts[1:]
	case strings.HasPrefix(string(frame), "\x1b]777;notify;"):
		return parts[2:]
	case strings.HasPrefix(string(frame), "\x1b]99;;"):
		return parts[2:]
	default:
		t.Fatalf("unrecognized frame %q", frame)
		return nil
	}
}

// hasControlByte reports whether a field carries a byte a terminal would act on
// or a layout would lose: C0, DEL and C1.
func hasControlByte(field string) bool {
	for _, r := range field {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return true
		}
	}
	return false
}

// protocolName labels a subtest with the protocol it exercises.
func protocolName(protocol Protocol) string {
	switch protocol {
	case OSC9:
		return "osc9"
	case OSC777:
		return "osc777"
	case OSC99:
		return "osc99"
	default:
		return "none"
	}
}

// ptrTo gives an event the nullable field a frame reads.
func ptrTo(value string) *string {
	return &value
}

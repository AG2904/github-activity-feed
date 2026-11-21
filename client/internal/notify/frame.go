package notify

// The frame a supported terminal turns into a desktop notification. It is a
// fixed template over fields that came out of the renderer's cleaner: the
// introducer, the separators and the terminator are the program's own literals,
// so event text can never start a sequence of its own (ADR-0017).

import (
	"strings"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
	"github.com/AG2904/github-activity-feed/client/internal/render"
)

// Protocol is the escape sequence a terminal acts on. The zero value is no
// protocol: a terminal the environment does not identify gets no frame at all,
// because a frame it does not understand is not worth the bytes.
type Protocol int

const (
	// NoProtocol means the terminal is not known to understand any frame.
	NoProtocol Protocol = iota
	// OSC9 is iTerm2's notification sequence, also understood by Ghostty,
	// WezTerm and kitty: one message, no separate title or body.
	OSC9
	// OSC777 is the sequence Windows Terminal understands, with a title and a
	// body kept apart.
	OSC777
	// OSC99 is kitty's extensible notification protocol.
	OSC99
)

// The frame's literals. The introducer is the only ESC the program emits and the
// terminator the only BEL, which is the property the security argument rests on:
// one of each, and no control byte inside a field.
const (
	introducer = "\x1b]"
	terminator = "\a"
)

// Field budgets: a notification daemon is not a scrollback, and an unbounded
// subject is a denial of service against it.
const (
	titleLimit = 64
	bodyLimit  = 200
)

// Message separator where the protocol has no title/body split.
const messageSeparator = " — "

// Lookup reads an environment variable. Injecting it keeps detection a pure
// function of what the terminal publishes, so no test depends on the machine it
// runs on.
type Lookup func(string) string

// Detect chooses the protocol from what the terminal already publishes. There is
// no query sequence and no probe: a probe writes the terminal's answer into a
// scrollback the session cannot clean up, and a terminal that does not implement
// the query prints the bytes literally (ADR-0017).
func Detect(lookup Lookup) Protocol {
	switch {
	case lookup("KITTY_WINDOW_ID") != "" || lookup("TERM") == "xterm-kitty":
		return OSC99
	case lookup("WT_SESSION") != "":
		return OSC777
	case isOneOf(lookup("TERM_PROGRAM"), "iTerm.app", "ghostty", "WezTerm"):
		return OSC9
	case lookup("TERM") == "xterm-ghostty":
		return OSC9
	default:
		return NoProtocol
	}
}

// Frame builds the bytes a terminal turns into one notification for one event.
// It is the only constructor, and it takes the event rather than strings, so
// every field necessarily came through the sanitizer and no caller can hand it
// text that skipped it. A protocol the terminal is not known to speak yields no
// bytes at all.
func Frame(protocol Protocol, event activity.Event) []byte {
	title := field(render.Sentence(event), titleLimit)
	body := field(bodyText(event), bodyLimit)

	switch protocol {
	case OSC9:
		return frame("9;" + title + messageSeparator + body)
	case OSC777:
		return frame("777;notify;" + title + ";" + body)
	case OSC99:
		// The two semicolons are always present, even with empty metadata; an
		// empty payload type defaults to a title.
		return frame("99;;" + title + messageSeparator + body)
	default:
		return nil
	}
}

// bodyText is the part of the frame that is not the sentence: the subject and
// the repository, in that order. The URL is deliberately absent.
func bodyText(event activity.Event) string {
	parts := []string{event.Subject}
	if event.Repository != nil && *event.Repository != "" {
		parts = append(parts, *event.Repository)
	}
	return strings.Join(parts, " · ")
}

// frame wraps the program's own payload in the introducer and the terminator.
func frame(payload string) []byte {
	return []byte(introducer + payload + terminator)
}

// field turns one event-derived value into one frame field. The renderer's
// cleaner drops the bytes a terminal acts on, a semicolon becomes a space so it
// cannot rewrite the title/body split, bidi overrides are dropped because they
// reverse what a toast shows without changing a visible byte, and the result is
// clamped to the field's budget without cutting a rune in half.
func field(text string, limit int) string {
	text = render.Clean(text)
	text = strings.Map(func(r rune) rune {
		switch {
		case r == ';':
			return ' '
		case isBidiOverride(r):
			return -1
		}
		return r
	}, text)
	return clamp(text, limit)
}

// isBidiOverride reports whether r is one of the bidi embedding, override and
// isolate characters the renderer's cleaner predates (U+202A–202E, U+2066–2069).
func isBidiOverride(r rune) bool {
	return (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069)
}

// clamp trims text to limit runes, never mid-rune.
func clamp(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

// isOneOf reports whether value is one of the accepted spellings.
func isOneOf(value string, accepted ...string) bool {
	for _, candidate := range accepted {
		if value == candidate {
			return true
		}
	}
	return false
}

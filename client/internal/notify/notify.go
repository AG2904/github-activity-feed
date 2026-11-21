// Package notify is the signal a shown event can raise outside the feed. It
// owns the whole decision: the modes, the environment-only detection, the
// protocol each terminal is known to speak, and the frame the terminal turns
// into a desktop notification. It never writes to stdout or stderr and it never
// fails a session: a signal that cannot be delivered is not an error
// (ADR-0017).
package notify

import (
	"io"

	"github.com/AG2904/github-activity-feed/client/internal/activity"
)

// Mode is how a shown event announces itself outside the feed.
type Mode int

const (
	// Off raises nothing. It is the default, because a signal is a decision
	// the reader makes and not something a binary starts doing on their behalf.
	Off Mode = iota
	// Bell rings the terminal bell, which carries no event text at all.
	Bell
	// OSC asks the terminal to raise a desktop notification.
	OSC
	// Auto picks between the frame and the bell from what the terminal already
	// says about itself.
	Auto
)

// modes is the accepted spelling of each mode, in the order the usage line
// names them. The set is closed, so a typo fails at the flag instead of
// silently raising nothing.
var modes = []struct {
	text string
	mode Mode
}{
	{"off", Off},
	{"bell", Bell},
	{"osc", OSC},
	{"auto", Auto},
}

// Parse reads a --notify value and reports whether it is one the client knows.
func Parse(text string) (Mode, bool) {
	for _, m := range modes {
		if m.text == text {
			return m.mode, true
		}
	}
	return Off, false
}

// Values names the accepted values for the usage line and the refusal: one
// sentence, one spelling, so the flag cannot accept a value it never names.
func Values() string {
	return "off, bell, osc or auto"
}

// Sink writes the signal for one shown event. Its writer is the console the
// process is attached to and never a stream, because a notification is not
// output: stdout stays the events contract and stderr stays plain text. A nil
// writer — no console, or a mode that raises nothing — makes every call a
// no-op, and every failure is swallowed so a signal can never take down a
// watch.
type Sink struct {
	mode     Mode
	protocol Protocol
	out      io.Writer
}

// resolve turns the requested mode into the signal the terminal can actually
// produce. It is a pure function of the mode and the environment the terminal
// already published, so the choice is testable without a terminal: auto becomes
// the frame when the environment identifies a terminal that speaks one, and the
// bell otherwise. It never probes (ADR-0017).
func resolve(mode Mode, lookup Lookup) (Mode, Protocol) {
	switch mode {
	case OSC:
		return OSC, Detect(lookup)
	case Auto:
		protocol := Detect(lookup)
		if protocol == NoProtocol {
			return Bell, NoProtocol
		}
		return OSC, protocol
	default:
		return mode, NoProtocol
	}
}

// New builds the sink the watch holds for the life of the process. The mode
// decides what is written; the writer decides whether it can be; escapes says
// whether that console acts on escape sequences at all; and the lookup is the
// environment the terminal already published, read once here because the
// protocol is a fixed-per-process decision. Auto is resolved here, so the sink
// itself only ever holds a concrete signal.
func New(mode Mode, out io.Writer, escapes bool, lookup Lookup) *Sink {
	if mode == Off {
		out = nil
	}
	resolved, protocol := resolve(mode, lookup)
	// A console that does not act on escapes would print the frame literally.
	// Auto falls back to the bell it can deliver; an explicit osc asked for a
	// frame, so its answer to an unusable console is no signal instead.
	if resolved == OSC && !escapes {
		if mode == Auto {
			resolved, protocol = Bell, NoProtocol
		} else {
			out = nil
		}
	}
	return &Sink{mode: resolved, protocol: protocol, out: out}
}

// Notify raises the signal for one event the feed showed. The bell carries no
// event text at all, by construction; osc writes the one frame the terminal is
// known to speak, and no bytes at all when the environment identified no
// terminal. A write that fails is swallowed: a signal is never the session's
// error.
func (s *Sink) Notify(event activity.Event) {
	if s == nil || s.out == nil {
		return
	}
	switch s.mode {
	case Bell:
		_, _ = io.WriteString(s.out, "\a")
	case OSC:
		_, _ = s.out.Write(Frame(s.protocol, event))
	}
}

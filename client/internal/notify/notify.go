// Package notify is the signal a shown event can raise outside the feed. It
// owns the modes, and from their own tickets the terminal protocols and the
// frame they carry. It never writes to stdout or stderr and it never fails a
// session: a signal that cannot be delivered is not an error (ADR-0017).
package notify

import "io"

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
	mode Mode
	out  io.Writer
}

// New builds the sink the watch holds for the life of the process. The mode
// decides what is written; the writer decides whether it can be.
func New(mode Mode, out io.Writer) *Sink {
	if mode == Off {
		out = nil
	}
	return &Sink{mode: mode, out: out}
}

// Notify raises the signal for one event the feed showed. Until the frame's own
// ticket lands, osc and auto are silent: they are accepted so a reader can write
// the flag they want before the mechanism exists, and they will never ring
// something the mode did not ask for.
func (s *Sink) Notify() {
	if s == nil || s.out == nil {
		return
	}
	if s.mode == Bell {
		_, _ = io.WriteString(s.out, "\a")
	}
}

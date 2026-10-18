// Package sse reads the relay's text/event-stream frames by hand (ADR-0007).
//
// Only the `data:` field matters to this client. The relay names every frame
// with `id: <n>`, but that counter restarts at zero on each connection, so it
// would be read as a resume position that does not exist; `event:`, `retry:`,
// and comment frames are skipped for the same reason - the client owns its own
// reconnection policy, and a heartbeat is a comment, not an event.
package sse

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// Reader yields the data payload of each frame, joining multi-line data with a
// newline as the SSE format specifies.
type Reader struct {
	br *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{br: bufio.NewReader(r)}
}

// Next returns the next frame that carries data. Frames without data - an
// `id:`-only frame, an `event:`-only frame, a heartbeat comment - are consumed
// and skipped. io.EOF means the stream ended cleanly.
func (r *Reader) Next() (string, error) {
	var data []string

	for {
		line, readErr := r.br.ReadString('\n')

		if line != "" || readErr == nil {
			line = strings.TrimRight(line, "\r\n")

			switch {
			case line == "":
				if len(data) > 0 {
					return strings.Join(data, "\n"), nil
				}
			case strings.HasPrefix(line, ":"):
				// A comment frame, including the relay's heartbeat.
			default:
				field, value, _ := strings.Cut(line, ":")
				if field == "data" {
					data = append(data, strings.TrimPrefix(value, " "))
				}
			}
		}

		if readErr == nil {
			continue
		}
		// A frame that ended at EOF without its blank line still counts.
		if len(data) > 0 {
			return strings.Join(data, "\n"), nil
		}
		if errors.Is(readErr, io.EOF) {
			return "", io.EOF
		}
		return "", readErr
	}
}

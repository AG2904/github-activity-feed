//go:build windows

package cli

import (
	"io"
	"os"
)

// console is the console the process is attached to, or nil when there is none.
// CONOUT$ is the console device itself, so it is there even when stdout and
// stderr are redirected to pipes or files - which is exactly why the signal can
// be written there while the two streams stay contracts (docs/cli-flow.md §6).
// A process started without a console - a service, a scheduled task, a windowless
// runner - gets nil and the watch simply has nowhere to ring.
//
// The device is also asked to interpret ANSI escapes when it can. The frame is
// an escape sequence, and the colour gate that normally enables the mode looks
// at stdout - which --no-color or a redirected stdout leaves out - while the
// signal belongs to the console and not to colour. A bell needs no mode, so the
// device is returned either way.
func console() io.Writer {
	device, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return nil
	}
	_ = isTerminal(device)
	return device
}

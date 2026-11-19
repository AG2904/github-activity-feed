//go:build windows

package cli

import (
	"io"
	"os"
)

// console is the console the process is attached to, or nil when there is none.
// CONOUT$ is the console device itself, so it is there even when stdout and
// stderr are redirected to pipes or files — which is exactly why the signal can
// be written there while the two streams stay contracts (docs/cli-flow.md §6).
// A process started without a console — a service, a scheduled task, a windowless
// runner — gets nil and the watch simply has nowhere to ring.
func console() io.Writer {
	device, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return nil
	}
	return device
}

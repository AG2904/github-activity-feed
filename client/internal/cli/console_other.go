//go:build !windows

package cli

import (
	"io"
	"os"
)

// console is the terminal the process is attached to and whether it acts on
// escape sequences; nil when there is none. /dev/tty is the controlling
// terminal, so it exists even when stdout and stderr are redirected to pipes or
// files - which is exactly why the signal can be written there while the two
// streams stay contracts (docs/cli-flow.md §6). A process started without a
// controlling terminal - CI, cron, a container with no tty - gets nil and the
// watch simply has nowhere to ring.
//
// A controlling terminal parses the escape sequences it is sent, so the bell
// and the frame are both usable there.
func console() (io.Writer, bool) {
	device, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return nil, false
	}
	return device, true
}

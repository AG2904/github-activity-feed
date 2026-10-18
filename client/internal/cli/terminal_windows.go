//go:build windows

package cli

import (
	"os"
	"syscall"
)

// enableVirtualTerminalProcessing is the console mode flag that makes Windows
// interpret ANSI escape sequences instead of printing them literally.
const enableVirtualTerminalProcessing = 0x0004

// The standard library exposes GetConsoleMode but not SetConsoleMode, and a
// third-party package would break the stdlib-only rule (ADR-0008).
var setConsoleMode = syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")

// isTerminal reports whether f is a console, and asks that console to
// interpret ANSI escapes. A redirected handle is not a console, and a console
// that refuses the mode gets no escapes at all rather than literal garbage.
func isTerminal(f *os.File) bool {
	handle := syscall.Handle(f.Fd())

	var mode uint32
	if err := syscall.GetConsoleMode(handle, &mode); err != nil {
		return false
	}

	mode |= enableVirtualTerminalProcessing
	ok, _, _ := setConsoleMode.Call(uintptr(handle), uintptr(mode))
	return ok != 0
}

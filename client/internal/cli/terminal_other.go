//go:build !windows

package cli

import "os"

// isTerminal reports whether f is a character device, which is what a terminal
// is on the platforms this build covers. A pipe or a regular file is not.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

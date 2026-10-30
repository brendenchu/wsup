//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package cli

import "os"

// isTerminal treats a character device as a terminal where terminal
// attributes cannot be read portably.
func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

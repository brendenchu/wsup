//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

// isTerminal reports whether file is a terminal by reading its terminal
// attributes; other character devices, such as /dev/null, are not terminals.
func isTerminal(file *os.File) bool {
	var termios syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), ioctlReadTermios, uintptr(unsafe.Pointer(&termios)))
	return errno == 0
}

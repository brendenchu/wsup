//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package cli

import "syscall"

const ioctlReadTermios = syscall.TIOCGETA

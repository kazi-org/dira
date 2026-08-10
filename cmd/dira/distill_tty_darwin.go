package main

import "golang.org/x/sys/unix"

// The ioctl requests that read and write a terminal's line discipline on the
// BSDs, macOS included. Linux spells the same two TCGETS and TCSETS; that
// difference in spelling is the whole reason this file and its linux twin exist,
// and distill_tty.go says why the difference is worth two four-line files rather
// than a new module.
const (
	ioctlGetTermios = unix.TIOCGETA
	ioctlSetTermios = unix.TIOCSETA
)

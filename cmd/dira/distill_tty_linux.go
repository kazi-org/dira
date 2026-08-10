package main

import "golang.org/x/sys/unix"

// The ioctl requests that read and write a terminal's line discipline on Linux.
// See distill_tty_darwin.go for the same two constants under their BSD names,
// and distill_tty.go for why the difference is spelled out here rather than
// imported.
const (
	ioctlGetTermios = unix.TCGETS
	ioctlSetTermios = unix.TCSETS
)

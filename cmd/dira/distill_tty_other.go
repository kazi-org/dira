//go:build !darwin && !linux

package main

import "io"

// Everywhere dira does not ship, `dira distill` still builds and still runs — it
// simply never finds a human.
//
// dira releases darwin and linux (build_test.go's buildPlatforms, and CI's
// ubuntu/macos matrix), and raw mode is the one part of this binary that needs a
// platform-specific syscall. The choice was between a new module that covers
// every GOOS (golang.org/x/term) and the termios ioctls already linked here;
// distill_tty.go records why the ioctls won and what it costs. This file is that
// cost, paid in the smallest form it can be: on any other platform the terminal
// reports no human, so `dira distill` prints the line it prints in CI and exits
// 0, and every other verb is unaffected.
func newTerminal(io.Reader) terminal { return offlineTerminal{} }

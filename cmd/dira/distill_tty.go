//go:build darwin || linux

package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"

	"github.com/kazi-org/dira/internal/distill"
)

// The concrete terminal: whether a human is present, how wide their window is,
// and how to read one keystroke from it.
//
// # Why golang.org/x/sys/unix and not golang.org/x/term
//
// Reading a single keystroke means turning off the line discipline, and the only
// two ways to do that from Go are x/term's MakeRaw or the termios ioctls
// underneath it. x/term is the shorter road and it was not taken, because the
// dependency it would add is not free and the one taken is:
//
//   - golang.org/x/sys is ALREADY linked into this binary on both platforms dira
//     ships. modernc.org/libc pulls golang.org/x/sys/unix in for the derived
//     read cache — `go list -deps ./cmd/dira` reports it on darwin and on linux
//     — and it is already on allowedModules in build_test.go. So this import
//     adds no module, needs no allowlist entry, and adds no bytes to the binary.
//   - golang.org/x/term would be a new module in the command path, and therefore
//     a new line on that allowlist, immediately after E1-L6-T5 REMOVED two module
//     groups from it to cut 1.20 MiB and ~21,700 init-time allocations off this
//     binary. Adding one back for a convenience wrapper over two ioctls would
//     spend part of what that lane just bought.
//
// What the shorter road would have bought is portability: x/term covers every
// platform, and the ioctl request names do not — TIOCGETA on the BSDs, TCGETS on
// Linux. That is what the two constants in distill_tty_darwin.go and
// distill_tty_linux.go are, and it is the entire difference between the two
// platforms. Everything else in this file is shared, and every other GOOS gets
// the offline terminal in distill_tty_other.go rather than a compile error.

// newTerminal is the process's end of a human, as a distill.Terminal.
//
// It takes the reader the app was built with rather than reaching for os.Stdin,
// so a test that hands the app a pipe gets a terminal that answers honestly
// instead of a terminal that answers about whatever the test runner's stdin
// happened to be.
func newTerminal(in io.Reader) terminal {
	file, ok := in.(*os.File)
	if !ok {
		// A reader that is not a file has no descriptor, so it has no line
		// discipline and no human behind it.
		return offlineTerminal{}
	}
	return &ttyTerminal{file: file}
}

// ttyTerminal is a real terminal on a file descriptor.
type ttyTerminal struct {
	file *os.File

	// cooked is the line discipline as dira found it, and is nil until raw
	// mode is entered. It is what Restore and Suspend put back, so it holds
	// the caller's settings and never a set this file composed.
	cooked *unix.Termios
}

// Interactive reports whether keystrokes can be read from this terminal.
//
// The question is asked by fetching the line discipline, which is the same
// syscall raw mode needs and therefore the same answer: if the termios can be
// read, the descriptor is a terminal. A mode check (os.ModeCharDevice) would
// have called /dev/null a terminal, and `dira distill < /dev/null` would enter
// raw mode on a descriptor that has none.
func (t *ttyTerminal) Interactive() bool {
	_, err := unix.IoctlGetTermios(int(t.file.Fd()), ioctlGetTermios)
	return err == nil
}

// Width is the terminal's column count, or 0 when the window cannot be
// measured. distill.Card reads 0 as "lay it out for the default".
func (t *ttyTerminal) Width() int {
	size, err := unix.IoctlGetWinsize(int(t.file.Fd()), unix.TIOCGWINSZ)
	if err != nil || size == nil {
		return 0
	}
	return int(size.Col)
}

// Raw turns off the line discipline and returns the keystroke source together
// with the call that turns it back on.
//
// The Restore is returned rather than being a second method for the reason
// distill.Terminal gives: there is no way to enter raw mode without also being
// handed the way out of it.
func (t *ttyTerminal) Raw() (distill.KeySource, distill.Restore, error) {
	fd := int(t.file.Fd())

	cooked, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	if err != nil {
		return nil, nil, fmt.Errorf("reading the terminal's line discipline: %w", err)
	}
	t.cooked = cooked

	if err := unix.IoctlSetTermios(fd, ioctlSetTermios, rawFrom(cooked)); err != nil {
		t.cooked = nil
		return nil, nil, fmt.Errorf("switching the terminal into raw mode: %w", err)
	}

	restored := false
	restore := func() error {
		if restored || t.cooked == nil {
			// Restore is called from a deferred function on every exit
			// path, and Suspend/Resume move the same setting around a
			// running editor. Doing it twice must be harmless rather
			// than an error the session then reports.
			return nil
		}
		restored = true
		saved := t.cooked
		t.cooked = nil
		return unix.IoctlSetTermios(fd, ioctlSetTermios, saved)
	}
	return &keystrokes{file: t.file}, restore, nil
}

// Suspend hands the terminal back for as long as something else needs it, and
// Resume takes it again. Both are no-ops outside raw mode.
func (t *ttyTerminal) Suspend() error {
	if t.cooked == nil {
		return nil
	}
	return unix.IoctlSetTermios(int(t.file.Fd()), ioctlSetTermios, t.cooked)
}

func (t *ttyTerminal) Resume() error {
	if t.cooked == nil {
		return nil
	}
	return unix.IoctlSetTermios(int(t.file.Fd()), ioctlSetTermios, rawFrom(t.cooked))
}

// rawFrom is the caller's line discipline with everything that stands between a
// key press and a byte turned off: no echo, no line buffering, no signal
// generation, no input or output translation, and a read that returns as soon as
// one byte has arrived.
//
// It composes from the terminal's own settings rather than from zero, so a
// window that was already configured some way keeps everything this does not
// name.
func rawFrom(cooked *unix.Termios) *unix.Termios {
	raw := *cooked
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	return &raw
}

// interrupt and endOfTransmission are the two bytes that, with ISIG off, arrive
// as ordinary input instead of as a signal.
const (
	interrupt         = 0x03 // ^C
	endOfTransmission = 0x04 // ^D
)

// keystrokes reads one byte at a time from a terminal in raw mode.
type keystrokes struct{ file *os.File }

// ReadKey returns the next keystroke.
//
// ^C and ^D come back as io.EOF, which the loop treats exactly as `q`: the
// remaining cards stay staged and the session ends through the ordinary exit,
// restoring the terminal on the way out. That mapping is not a convenience. Raw
// mode turns off signal generation, so with ISIG cleared a ^C is a byte and
// nothing else — and the loop ignores bytes it does not recognise, which would
// leave a human who reached for the one key everybody reaches for looking at a
// queue that would not let go. Handling it here rather than by leaving ISIG on
// is what keeps the terminal restored: a default SIGINT kills the process before
// any deferred restore runs, and the shell is left in raw mode.
func (k *keystrokes) ReadKey() (byte, error) {
	var buf [1]byte
	for {
		n, err := k.file.Read(buf[:])
		if n == 1 {
			if buf[0] == interrupt || buf[0] == endOfTransmission {
				return 0, io.EOF
			}
			return buf[0], nil
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, io.EOF
			}
			return 0, err
		}
		// A zero-byte read with no error is not the end of anything; ask
		// again rather than reporting a keystroke nobody pressed.
	}
}

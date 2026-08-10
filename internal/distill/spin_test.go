package distill

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestAFailingDispositionCannotSpinForever pins the bound on the retry loop.
//
// A failed disposition deliberately does not advance the card, so a human sees
// the reason and can retry or quit. That is right for a human and unbounded for
// anything else: a KeySource that never ends, against a store failing
// persistently, spun here forever. E2-L4-T7 found it by mutation, in code that
// was already merged.
//
// The test is shaped so it would HANG without the bound rather than fail
// cleanly, which is why it runs under a timeout. A hang is the defect; a test
// that could not hang would not be testing for it.
func TestAFailingDispositionCannotSpinForever(t *testing.T) {
	t.Parallel()

	store, _, _ := tempLedger(t)
	put(t, store, sniffShaped("dec-0001"), sniffShaped("dec-0002"))

	done := make(chan error, 1)
	go func() {
		_, err := Loop(context.Background(), Options{
			Store:   refusingStore{Store: store, putErr: errors.New("the store refuses this write")},
			Keys:    endless(KeyConfirm),
			Display: silent{},
			Now:     func() time.Time { return loopNow },
		})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a persistently failing disposition returned no error; the loop gave up silently")
		}
		t.Logf("OBSERVED  bounded rather than spinning: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("the loop did not return within 3s against an endless key source and a refusing store — " +
			"this is the unbounded spin the bound exists to prevent")
	}
}

// endless never runs out of keystrokes, which is what a scripted driver does
// not model and what makes the spin reachable.
type endless byte

func (e endless) ReadKey() (byte, error) { return byte(e), nil }

type silent struct{}

func (silent) Show(string) {}

package nest

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// fastClamp lets the tests use millisecond stream lifetimes.
func fastClamp(t *testing.T) {
	t.Helper()
	old := minExtendDelay
	minExtendDelay = time.Millisecond
	t.Cleanup(func() { minExtendDelay = old })
}

// A session must keep extending itself for as long as Google keeps granting
// extensions, not just once.
func TestSessionExtendsRepeatedly(t *testing.T) {
	fastClamp(t)
	var calls atomic.Int32
	const life = 40 * time.Millisecond
	const lead = 10 * time.Millisecond

	s := newSession(time.Now().Add(life), lead, func() (time.Time, error) {
		calls.Add(1)
		return time.Now().Add(life), nil
	})
	defer s.Stop()

	time.Sleep(3*life + lead)

	if n := calls.Load(); n < 3 {
		t.Fatalf("expected at least 3 extensions, got %d", n)
	}
}

// Once an extension fails the stream is going to die at its expiry; nothing
// further should be attempted. The producer reconnect handles the rest.
func TestSessionStopsAfterFailedExtend(t *testing.T) {
	fastClamp(t)
	var calls atomic.Int32
	const life = 40 * time.Millisecond

	s := newSession(time.Now().Add(life), 10*time.Millisecond, func() (time.Time, error) {
		calls.Add(1)
		return time.Time{}, errors.New("nest: wrong status: 404 Not Found")
	})
	defer s.Stop()

	time.Sleep(4 * life)

	if n := calls.Load(); n != 1 {
		t.Fatalf("expected exactly 1 extension attempt, got %d", n)
	}
}

// Stop must cancel a pending extension.
func TestSessionStopCancelsTimer(t *testing.T) {
	fastClamp(t)
	var calls atomic.Int32
	const life = 40 * time.Millisecond

	s := newSession(time.Now().Add(life), 10*time.Millisecond, func() (time.Time, error) {
		calls.Add(1)
		return time.Now().Add(life), nil
	})
	s.Stop()

	time.Sleep(2 * life)

	if n := calls.Load(); n != 0 {
		t.Fatalf("expected no extensions after Stop, got %d", n)
	}
}

// An expiry that is already inside the lead window must not spin: the first
// extension is delayed by at least minExtendDelay.
func TestSessionNeverSchedulesInstantly(t *testing.T) {
	var calls atomic.Int32

	s := newSession(time.Now(), time.Minute, func() (time.Time, error) {
		calls.Add(1)
		return time.Now(), nil
	})
	defer s.Stop()

	time.Sleep(minExtendDelay / 2)

	if n := calls.Load(); n != 0 {
		t.Fatalf("extension fired before minExtendDelay, calls=%d", n)
	}
}

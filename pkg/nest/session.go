package nest

import (
	"sync"
	"time"
)

// Google grants a Nest live stream for five minutes at a time. Unless it is
// extended before then the camera simply stops sending and the producer has
// to start over with a fresh GenerateWebRtcStream/GenerateRtspStream.

// minExtendDelay guards against an expiry that is already inside the lead
// window (or in the past), which would otherwise fire back-to-back.
// A var so tests can run with millisecond lifetimes.
var minExtendDelay = time.Second

// session keeps one live stream alive by extending it `lead` before every
// expiry, for as long as the extensions keep succeeding.
type session struct {
	lead   time.Duration
	extend func() (expiresAt time.Time, err error)

	mu    sync.Mutex
	timer *time.Timer
	done  bool
}

func newSession(expiresAt time.Time, lead time.Duration, extend func() (time.Time, error)) *session {
	s := &session{lead: lead, extend: extend}
	s.schedule(expiresAt)
	return s
}

func (s *session) schedule(expiresAt time.Time) {
	delay := time.Until(expiresAt) - s.lead
	if delay < minExtendDelay {
		delay = minExtendDelay
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return
	}
	s.timer = time.AfterFunc(delay, s.run)
}

func (s *session) run() {
	expiresAt, err := s.extend()
	if err != nil {
		// the stream will die at its expiry and the producer reconnects
		return
	}
	s.schedule(expiresAt)
}

// Stop cancels any pending extension. Safe to call on a nil session.
func (s *session) Stop() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.done = true
	if s.timer != nil {
		s.timer.Stop()
	}
}

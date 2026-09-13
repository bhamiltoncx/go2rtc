package core

import "errors"

// Temporary is implemented by errors that describe a passing condition - a
// rate limit, a quota answer, a server-side failure - rather than a verdict
// about the source. A stream with several sources falls through to the next
// one on a definitive error, but not on a temporary one: serving a fallback
// because the primary was merely throttled would pin consumers to it.
type Temporary interface {
	Temporary() bool
}

// IsTemporary reports whether err, or any error it wraps, is temporary.
func IsTemporary(err error) bool {
	var t Temporary
	return errors.As(err, &t) && t.Temporary()
}

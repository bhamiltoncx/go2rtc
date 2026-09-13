package core

import (
	"errors"
	"fmt"
	"testing"
)

type tempErr struct{ temp bool }

func (e tempErr) Error() string   { return "x" }
func (e tempErr) Temporary() bool { return e.temp }

func TestIsTemporary(t *testing.T) {
	if IsTemporary(errors.New("plain")) {
		t.Fatal("plain errors are not temporary")
	}
	if IsTemporary(tempErr{false}) {
		t.Fatal("Temporary() false must be honoured")
	}
	if !IsTemporary(tempErr{true}) {
		t.Fatal("Temporary() true")
	}
	if !IsTemporary(fmt.Errorf("wrapped: %w", tempErr{true})) {
		t.Fatal("wrapped temporary errors must be found")
	}
}

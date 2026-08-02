// Package jsguard turns a panic into an error at the syscall/js boundary.
//
// The house rule of this codebase: a panic in a wasm frontend takes the
// whole application down — the user's unsaved document with it — so
// nothing that crosses into JavaScript may panic. A JS exception reaching
// Go through syscall/js IS a panic, and a DOM call made with foreign data
// throws for reasons we cannot rule out in advance: `matches()` on a
// selector a book wrote, `Intl.NumberFormat` on a locale from a catalog,
// a method a browser version does not have.
//
// The pattern was written by hand thirteen times before this package,
// each site re-deciding its own fallback and its own message — and seven
// of them (the Intl cache) reported NOTHING, so a formatter that failed
// left numbers rendering wrong with no clue anywhere. One implementation
// makes the decision once and makes the failure say what it was.
//
// The package deliberately imports nothing from syscall/js: guarding is
// plain Go, so this stays portable and unit-testable under the native
// toolchain, which is where its own correctness can actually be checked.
//
// It guards a BLOCK rather than a single call, because that is the shape
// the boundary actually has — building an Intl formatter is a dozen JS
// operations that fail as a unit, and wrapping each one separately would
// be both louder and slower.
package jsguard

import (
	"fmt"
	"runtime/debug"
)

// maxStack bounds the trace kept on a PanicError. A wasm stack is deep
// and mostly runtime frames; the top of it is what names the call site,
// and an unbounded copy on a path that may repeat is a leak waiting to
// be found.
const maxStack = 8 << 10

// PanicError reports a panic recovered at the JS boundary.
//
// The trace is a FIELD, not a chain of wrapped errors. %w exists for
// semantic cause — so errors.Is and errors.As can find a sentinel
// underneath — and a stack is data, not a cause: wrapping one frame per
// line would make errors.Is walk a wall of noise and turn the message
// into "frame: frame: frame: …: the actual problem".
//
// Value is the recovered value itself, and it is usually the datum that
// solves the problem: a DOMException carries text like "'p.haikai[' is
// not a valid selector", which says more than any stack.
type PanicError struct {
	Op    string // what was being attempted, for the message
	Value any    // what recover() returned — often a js.Error
	Stack []byte // Go frames, truncated to maxStack
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("jsguard: %s panicked: %v", e.Op, e.Value)
}

// Do runs fn and converts a panic into a *PanicError. It returns nil when
// fn completes.
func Do(op string, fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = newPanicError(op, r)
		}
	}()
	fn()
	return nil
}

// Value runs fn and returns its result, or the zero value of T and a
// *PanicError when fn panicked.
//
// The zero value is what every call site wanted anyway: an undefined
// js.Value, a false, an empty string — "this did not work, carry on with
// nothing" is the only safe reading of a failure here, and inventing a
// different fallback per site is how thirteen of them drifted apart.
func Value[T any](op string, fn func() T) (out T, err error) {
	defer func() {
		if r := recover(); r != nil {
			var zero T
			out, err = zero, newPanicError(op, r)
		}
	}()
	return fn(), nil
}

func newPanicError(op string, r any) *PanicError {
	stack := debug.Stack()
	if len(stack) > maxStack {
		stack = stack[:maxStack]
	}
	return &PanicError{Op: op, Value: r, Stack: stack}
}

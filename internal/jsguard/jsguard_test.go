package jsguard

import (
	"errors"
	"strings"
	"testing"
)

func TestDoPassesThrough(t *testing.T) {
	ran := false
	if err := Do("noop", func() { ran = true }); err != nil {
		t.Errorf("Do returned %v for a function that did not panic", err)
	}
	if !ran {
		t.Error("Do did not run fn")
	}
}

func TestDoRecovers(t *testing.T) {
	err := Do("matches", func() { panic("boom") })
	if err == nil {
		t.Fatal("Do returned nil for a panicking function")
	}
	var pe *PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("error is %T, want *PanicError", err)
	}
	if pe.Op != "matches" {
		t.Errorf("Op = %q, want %q", pe.Op, "matches")
	}
	if pe.Value != "boom" {
		t.Errorf("Value = %v, want the recovered value", pe.Value)
	}
	if len(pe.Stack) == 0 {
		t.Error("Stack is empty; the trace is the point of keeping it")
	}
	if !strings.Contains(err.Error(), "matches") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("Error() = %q, want it to name the op and the value", err.Error())
	}
}

func TestValuePassesThrough(t *testing.T) {
	got, err := Value("len", func() int { return 42 })
	if err != nil || got != 42 {
		t.Errorf("Value = (%v, %v), want (42, nil)", got, err)
	}
}

// TestValueZeroOnPanic: the zero value is the contract. Every hand-rolled
// site wanted exactly this — an undefined js.Value, a false, an empty
// string — and letting each invent its own fallback is how thirteen of
// them drifted apart.
func TestValueZeroOnPanic(t *testing.T) {
	gotBool, err := Value("b", func() bool { panic("x") })
	if gotBool || err == nil {
		t.Errorf("Value = (%v, %v), want (false, error)", gotBool, err)
	}
	gotStr, err := Value("s", func() string { panic("x") })
	if gotStr != "" || err == nil {
		t.Errorf("Value = (%q, %v), want (\"\", error)", gotStr, err)
	}
}

// TestStackIsBounded: the guarded paths can repeat (a mousemove over a
// document whose selectors are invalid), so an unguarded copy of a deep
// wasm stack would be a slow leak.
func TestStackIsBounded(t *testing.T) {
	var deep func(int) error
	deep = func(n int) error {
		if n == 0 {
			return Do("deep", func() { panic("x") })
		}
		return deep(n - 1)
	}
	var pe *PanicError
	if !errors.As(deep(400), &pe) {
		t.Fatal("expected a PanicError")
	}
	if len(pe.Stack) > maxStack {
		t.Errorf("Stack is %d bytes, want at most %d", len(pe.Stack), maxStack)
	}
}

// TestErrorsIsFindsSentinelUnderneath: PanicError must not swallow a
// wrapped sentinel — the reason the trace is a field and not a chain.
func TestPanicErrorAsWorksThroughFmt(t *testing.T) {
	err := Do("op", func() { panic("x") })
	wrapped := errors.Join(errors.New("context"), err)
	var pe *PanicError
	if !errors.As(wrapped, &pe) {
		t.Error("errors.As could not find the PanicError through a join")
	}
}

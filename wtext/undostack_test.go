package wtext

import "testing"

// fakeOp sizes itself for byte-budget tests.
type fakeOp struct{ size int }

func newStack(maxSteps, maxBytes int) *undoStack[fakeOp] {
	return newUndoStack[fakeOp](maxSteps, maxBytes,
		func(op fakeOp) int { return op.size })
}

func step(sizes ...int) []fakeOp {
	ops := make([]fakeOp, 0, len(sizes))
	for _, s := range sizes {
		ops = append(ops, fakeOp{size: s})
	}
	return ops
}

func TestUndoRedoFlow(t *testing.T) {
	st := newStack(10, 1000)
	st.Push(step(1))
	st.Push(step(2))
	if st.Len() != 2 || st.RedoLen() != 0 {
		t.Fatalf("after pushes: undo %d redo %d", st.Len(), st.RedoLen())
	}
	s, ok := st.PopUndo()
	if !ok || len(s) != 1 || s[0].size != 2 {
		t.Fatalf("PopUndo returned %v %v", s, ok)
	}
	if st.Len() != 1 || st.RedoLen() != 1 {
		t.Fatalf("after undo: undo %d redo %d", st.Len(), st.RedoLen())
	}
	if s, ok = st.PopRedo(); !ok || s[0].size != 2 {
		t.Fatalf("PopRedo returned %v %v", s, ok)
	}
	if st.Len() != 2 || st.RedoLen() != 0 {
		t.Fatalf("after redo: undo %d redo %d", st.Len(), st.RedoLen())
	}
}

func TestPushForksHistory(t *testing.T) {
	st := newStack(10, 1000)
	st.Push(step(1))
	st.Push(step(2))
	st.PopUndo()
	st.Push(step(3)) // a new edit after undo drops the redo branch
	if st.RedoLen() != 0 {
		t.Errorf("redo branch survived a fork: %d", st.RedoLen())
	}
	if st.Len() != 2 {
		t.Errorf("undo depth = %d, want 2", st.Len())
	}
}

func TestStepCapEvicts(t *testing.T) {
	st := newStack(3, 1_000_000)
	for i := 0; i < 5; i++ {
		st.Push(step(i + 1))
	}
	if st.Len() != 3 {
		t.Fatalf("len = %d, want 3", st.Len())
	}
	// Oldest surviving step must be the 3rd push (size 3).
	st.PopUndo()
	st.PopUndo()
	s, _ := st.PopUndo()
	if s[0].size != 3 {
		t.Errorf("oldest survivor has size %d, want 3", s[0].size)
	}
}

func TestByteBudgetEvicts(t *testing.T) {
	st := newStack(100, 100)
	st.Push(step(60))
	st.Push(step(60)) // 120 > 100: first step must go
	if st.Len() != 1 {
		t.Fatalf("len = %d, want 1", st.Len())
	}
}

func TestNewestStepAlwaysKept(t *testing.T) {
	st := newStack(100, 100)
	st.Push(step(50))
	st.Push(step(5000)) // alone over budget: still kept, older evicted
	if st.Len() != 1 {
		t.Fatalf("len = %d, want 1", st.Len())
	}
	if s, _ := st.PopUndo(); s[0].size != 5000 {
		t.Errorf("huge step was evicted instead of kept")
	}
}

func TestEmptyStepIgnored(t *testing.T) {
	st := newStack(10, 1000)
	st.Push(nil)
	st.Push(step())
	if st.Len() != 0 {
		t.Errorf("empty steps were recorded: %d", st.Len())
	}
}

func TestClear(t *testing.T) {
	st := newStack(10, 1000)
	st.Push(step(1))
	st.PopUndo()
	st.Push(step(2))
	st.Clear()
	if st.Len() != 0 || st.RedoLen() != 0 {
		t.Errorf("clear left undo %d redo %d", st.Len(), st.RedoLen())
	}
}

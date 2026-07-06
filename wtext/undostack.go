package wtext

// undoStack is the bounded two-list undo/redo bookkeeping, generic over
// the op type so the js side can store DOM-holding ops while native tests
// exercise the eviction logic with plain fakes. Bounded everything: a step
// cap and a byte budget, whichever trips first, evict the oldest steps —
// except the newest step, which is always kept even if it alone exceeds
// the budget (a huge paste must still be undoable once).
type undoStack[O any] struct {
	undo     [][]O
	redo     [][]O
	size     func(O) int
	maxSteps int
	maxBytes int
	bytes    int
}

// newUndoStack returns a stack bounded by maxSteps and maxBytes, using
// size to account each op's retained bytes.
func newUndoStack[O any](maxSteps, maxBytes int, size func(O) int) *undoStack[O] {
	return &undoStack[O]{size: size, maxSteps: maxSteps, maxBytes: maxBytes}
}

// stepBytes sums the retained size of one step.
func (u *undoStack[O]) stepBytes(step []O) int {
	total := 0
	for _, op := range step {
		total += u.size(op)
	}
	return total
}

// Push records a new user step and clears the redo branch (a new edit
// forks history). Empty steps are ignored.
func (u *undoStack[O]) Push(step []O) {
	if len(step) == 0 {
		return
	}
	u.redo = nil
	u.undo = append(u.undo, step)
	u.bytes += u.stepBytes(step)
	for len(u.undo) > 1 && (len(u.undo) > u.maxSteps || u.bytes > u.maxBytes) {
		u.bytes -= u.stepBytes(u.undo[0])
		u.undo[0] = nil
		u.undo = u.undo[1:]
	}
}

// PopUndo moves the newest step to the redo branch and returns it.
func (u *undoStack[O]) PopUndo() (step []O, ok bool) {
	if len(u.undo) == 0 {
		return nil, false
	}
	last := len(u.undo) - 1
	step = u.undo[last]
	u.undo = u.undo[:last]
	u.bytes -= u.stepBytes(step)
	u.redo = append(u.redo, step)
	return step, true
}

// PopRedo moves the newest undone step back to the undo branch and
// returns it.
func (u *undoStack[O]) PopRedo() (step []O, ok bool) {
	if len(u.redo) == 0 {
		return nil, false
	}
	last := len(u.redo) - 1
	step = u.redo[last]
	u.redo = u.redo[:last]
	u.undo = append(u.undo, step)
	u.bytes += u.stepBytes(step)
	return step, true
}

// Clear drops both branches (form reset, new content loaded).
func (u *undoStack[O]) Clear() {
	u.undo, u.redo, u.bytes = nil, nil, 0
}

// Len returns the number of undoable steps.
func (u *undoStack[O]) Len() int { return len(u.undo) }

// RedoLen returns the number of redoable steps.
func (u *undoStack[O]) RedoLen() int { return len(u.redo) }

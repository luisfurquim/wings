package wtext

// Exported toolbar building blocks. They are what BasicToolbar itself is
// made of, and they are public on purpose: a MyToolbar in another package
// composes the same behaviours instead of reimplementing them. Names avoid
// colliding with EditorCore's method set.

// ToggleMark applies m to the current selection unless the whole
// selection is already inside the mark, in which case it removes it — the
// Word behaviour: a partially bold selection turns fully bold first;
// toggling again removes it.
func ToggleMark(core EditorCore, m Mark) error {
	sel, ok := core.Sel()
	if !ok {
		return nil // no selection in the editor: nothing to do
	}
	in, err := core.InMark(sel, m.Tag())
	if err != nil {
		return err
	}
	if in {
		return core.Unwrap(sel, m.Tag())
	}
	return core.Wrap(sel, m)
}

// MarkToggle adapts ToggleMark to a ToggleItem.Do closure.
func MarkToggle(m Mark) func(EditorCore) error {
	return func(core EditorCore) error { return ToggleMark(core, m) }
}

// MarkActive returns a ToggleItem.Active closure reporting whether the
// current selection sits inside the mark tag.
func MarkActive(tag string) func(EditorCore) bool {
	return func(core EditorCore) bool {
		sel, ok := core.Sel()
		if !ok {
			return false
		}
		in, err := core.InMark(sel, tag)
		return err == nil && in
	}
}

// BlockCurrent returns a SelectItem.Current closure reporting the block
// tag at the selection start ("" when there is no selection).
func BlockCurrent() func(EditorCore) string {
	return func(core EditorCore) string {
		sel, ok := core.Sel()
		if !ok {
			return ""
		}
		tag, err := core.BlockType(sel)
		if err != nil {
			return ""
		}
		return tag
	}
}

// BlockPick returns a SelectItem.Pick closure converting the selection's
// blocks to the picked tag.
func BlockPick() func(EditorCore, string) error {
	return func(core EditorCore, tag string) error {
		sel, ok := core.Sel()
		if !ok {
			return nil
		}
		return core.SetBlock(sel, tag)
	}
}

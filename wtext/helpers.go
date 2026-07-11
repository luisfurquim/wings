package wtext

import "strings"

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

// ToggleClass applies name to the current selection unless it already
// spans the WHOLE selection, in which case it removes it — the same Word
// nuance ToggleMark gives semantic marks (a partially-covered selection
// spans fully first; toggling again removes it), built on ClassSpanned
// instead of InMark. Meant for a plain on/off character class with no
// block half (Bold, Italic) — a single name, not a picker family; the
// class must already be registered (DefineClass, typically from an
// InitPlugin) before any toggle fires.
func ToggleClass(core EditorCore, name string) error {
	sel, ok := core.Sel()
	if !ok {
		return nil
	}
	on, err := core.ClassSpanned(sel, name)
	if err != nil {
		return err
	}
	if on {
		return core.RemoveClass(sel, name)
	}
	return core.ApplyClass(sel, name)
}

// ClassMarkToggle adapts ToggleClass to a ToggleItem.Do closure.
func ClassMarkToggle(name string) func(EditorCore) error {
	return func(core EditorCore) error { return ToggleClass(core, name) }
}

// ClassMarkActive returns a ToggleItem.Active closure reporting whether
// name spans the whole current selection.
func ClassMarkActive(name string) func(EditorCore) bool {
	return func(core EditorCore) bool {
		sel, ok := core.Sel()
		if !ok {
			return false
		}
		on, err := core.ClassSpanned(sel, name)
		return err == nil && on
	}
}

// DualToggle mirrors ToggleClass/ToggleMark's Word nuance but recognizes
// EITHER a character class or a semantic mark as "this formatting is
// active" — content a user pasted or loaded may carry the genuine
// semantic tag (a document authored elsewhere, or a deliberately marked
// phrase the profile's filter preserves) while the toolbar's own writes
// use the class; a Bold/Italic click must be able to affect either, or
// pasted-bold text becomes permanently stuck as far as the toolbar is
// concerned. Checked at both ends of the selection like its two
// building blocks. Turning ON with neither present applies the class —
// the toolbar's own mechanism; turning OFF removes whichever is
// actually present (both, if somehow both wrap the same range).
func DualToggle(core EditorCore, className, markTag string) error {
	sel, ok := core.Sel()
	if !ok {
		return nil
	}
	classOn, err := core.ClassSpanned(sel, className)
	if err != nil {
		return err
	}
	markOn, err := core.InMark(sel, markTag)
	if err != nil {
		return err
	}
	if !classOn && !markOn {
		return core.ApplyClass(sel, className)
	}
	if classOn {
		if err := core.RemoveClass(sel, className); err != nil {
			return err
		}
	}
	if markOn {
		if err := core.Unwrap(sel, markTag); err != nil {
			return err
		}
	}
	return nil
}

// DualMarkToggle adapts DualToggle to a ToggleItem.Do closure.
func DualMarkToggle(className, markTag string) func(EditorCore) error {
	return func(core EditorCore) error { return DualToggle(core, className, markTag) }
}

// DualMarkActive returns a ToggleItem.Active closure reporting whether
// EITHER the class or the mark is active over the whole selection.
func DualMarkActive(className, markTag string) func(EditorCore) bool {
	return func(core EditorCore) bool {
		sel, ok := core.Sel()
		if !ok {
			return false
		}
		if classOn, err := core.ClassSpanned(sel, className); err == nil && classOn {
			return true
		}
		markOn, err := core.InMark(sel, markTag)
		return err == nil && markOn
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

// SwapClass makes name the only class of its prefix family on the current
// selection: every other registered class sharing prefix is removed from
// the range, then name (when non-empty) is applied — one undo step. It is
// the exclusivity rule behind single-axis pickers (font face, font size,
// alignment): a range has one font, picking another replaces it.
func SwapClass(core EditorCore, prefix, name string) error {
	sel, ok := core.Sel()
	if !ok {
		return nil
	}
	return core.Txn(func(c EditorCore) error {
		for _, cls := range c.Classes() {
			if !strings.HasPrefix(cls, prefix) || cls == name {
				continue
			}
			if err := c.RemoveClass(sel, cls); err != nil {
				return err
			}
		}
		if name == "" {
			return nil
		}
		return c.ApplyClass(sel, name)
	})
}

// ClassPick adapts SwapClass to a SelectItem.Pick closure: the picked
// option value is the class id within the family (prefix + value), the
// empty value clears the family from the selection.
func ClassPick(prefix string) func(EditorCore, string) error {
	return func(core EditorCore, value string) error {
		name := ""
		if value != "" {
			name = prefix + value
		}
		return SwapClass(core, prefix, name)
	}
}

// ClassToggle adapts SwapClass to a ToggleItem.Do: it applies name when
// inactive and removes it (back to the family's bare default) when the
// selection already carries it.
func ClassToggle(prefix, name string) func(EditorCore) error {
	return func(core EditorCore) error {
		sel, ok := core.Sel()
		if !ok {
			return nil
		}
		classes, err := core.ClassesAt(sel)
		if err != nil {
			return err
		}
		target := name
		if containsString(classes, name) {
			target = ""
		}
		return SwapClass(core, prefix, target)
	}
}

// ClassCurrent returns a SelectItem.Current closure reporting the first
// class of the prefix family in effect at the selection, without the
// prefix ("" when none — the family's default option).
func ClassCurrent(prefix string) func(EditorCore) string {
	return func(core EditorCore) string {
		sel, ok := core.Sel()
		if !ok {
			return ""
		}
		classes, err := core.ClassesAt(sel)
		if err != nil {
			return ""
		}
		for _, cls := range classes {
			if strings.HasPrefix(cls, prefix) {
				return strings.TrimPrefix(cls, prefix)
			}
		}
		return ""
	}
}

// ClassActive returns a ToggleItem.Active closure reporting whether name
// is in effect at the selection.
func ClassActive(name string) func(EditorCore) bool {
	return func(core EditorCore) bool {
		sel, ok := core.Sel()
		if !ok {
			return false
		}
		classes, err := core.ClassesAt(sel)
		return err == nil && containsString(classes, name)
	}
}

// containsString reports whether list holds s.
func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

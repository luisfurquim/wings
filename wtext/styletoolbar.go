package wtext

import (
	"strings"

	"github.com/luisfurquim/wings/epubhtml"
)

// StyleToolbar is the named-styles plugin — Word's style gallery: a
// "create style from selection" prompt that captures the formatting
// classes covering the selection into one named, reusable class, and a
// picker applying registered styles to the selection.
//
// Utility classes (wt-*) are wings' own vocabulary: the picker never
// lists them, user styles cannot take the prefix, and applying a style
// leaves them on the range — direct formatting keeps overriding the
// style, as it does in Word.
type StyleToolbar struct{}

// Items declares the prompt and the picker.
func (StyleToolbar) Items() []ToolbarItem {
	return []ToolbarItem{
		InputItem{
			ID: "style-new", Label: "wtext-style-new", Icon: "style_new", Help: "wtext-style-new-help",
			Placeholder: "wtext-style-name",
			Do:          CreateStyle,
		},
		SelectItem{
			ID: "style", Label: "wtext-style", Help: "wtext-style-help",
			Options: StyleOptions, Current: StyleCurrent, Pick: StylePick,
		},
	}
}

// CreateStyle captures the formatting in effect at the selection into a
// named class: the CSS of every class covering the selection start is
// merged (innermost formatting wins), registered under name, and the
// source selection swaps its old classes for the new style — from here
// on it follows the style. Naming an existing style redefines it.
func CreateStyle(core EditorCore, name string) error {
	name = strings.TrimSpace(name)
	if err := epubhtml.ValidClassName(name); err != nil {
		return err
	}
	if strings.HasPrefix(name, "wt-") {
		return ErrReservedClass
	}
	sel, ok := core.Sel()
	if !ok {
		return ErrNoSelection
	}
	classes, err := core.ClassesAt(sel)
	if err != nil {
		return err
	}
	var layers []string
	for _, cls := range classes {
		if css, defined := core.ClassCSS(cls); defined {
			layers = append(layers, css)
		}
	}
	merged := epubhtml.MergeCSS(layers...)
	if merged == "" {
		return ErrNoFormatting
	}
	if err := core.DefineClass(name, merged); err != nil {
		return err
	}
	// Each step RE-READS the selection instead of reusing the one captured
	// above, and that is not defensive noise: RemoveClass carves text
	// nodes, and a Selection captured before a carve silently stops
	// describing the user's range — the original text node object becomes
	// the FIRST carved piece, so the stale handles still pass every
	// validity check while naming a fragment (the hazard mutate.go spells
	// out at length). Reusing one Selection across the removals left the
	// final ApplyClass working on a sliver, so a style created from a
	// selection was not applied to the very text it was created from.
	//
	// Re-reading is safe because every mutation restores the document
	// selection by CHARACTER OFFSET (restoreSelAt), which survives the
	// restructuring the handles do not.
	return core.Txn(func(c EditorCore) error {
		for _, cls := range classes {
			if cls == name {
				continue
			}
			if _, defined := c.ClassCSS(cls); !defined {
				// A class the DOCUMENT brought and this editor never
				// registered — a book's `dropcaps`, kept on the element so
				// the book's own `span.dropcaps` rule can still match it.
				// Not ours to remove: RemoveClass refuses an unregistered
				// class (ErrUnknownClass) and would take the whole
				// transaction down with it, so creating a style anywhere
				// inside an imported document failed outright and silently
				// left the text unstyled.
				//
				// Skipping is also the right behaviour and not merely the
				// safe one: stripping the hook a book's stylesheet selects
				// on would delete formatting the user never asked to lose.
				continue
			}
			cur, ok := c.Sel()
			if !ok {
				return ErrNoSelection
			}
			if err := c.RemoveClass(cur, cls); err != nil {
				return err
			}
		}
		cur, ok := c.Sel()
		if !ok {
			return ErrNoSelection
		}
		return c.ApplyClass(cur, name)
	})
}

// StyleOptions lists the registered named styles behind a "(none)"
// option that clears the style from the selection. Style names are user
// data, not message ids — they display as themselves.
func StyleOptions(core EditorCore) []Option {
	opts := []Option{{Value: "", Label: "wtext-style-none"}}
	for _, name := range core.Classes() {
		if strings.HasPrefix(name, "wt-") {
			continue
		}
		opts = append(opts, Option{Value: name, Label: name})
	}
	return opts
}

// StyleCurrent reports the first named style in effect at the selection.
func StyleCurrent(core EditorCore) string {
	sel, ok := core.Sel()
	if !ok {
		return ""
	}
	classes, err := core.ClassesAt(sel)
	if err != nil {
		return ""
	}
	for _, cls := range classes {
		if strings.HasPrefix(cls, "wt-") {
			continue
		}
		// A style with a character component only counts as current when
		// that half is actually spanned here — otherwise unstyled text
		// sharing a paragraph with styled text (the style's block half
		// bleeds through ClassesAt for the whole block) would falsely
		// report as "already this style", and picking it again would look
		// like a no-op re-select to the combobox (no @change, no apply).
		css, _ := core.ClassCSS(cls)
		charCSS, _ := epubhtml.SplitCSS(css)
		if charCSS != "" {
			spanned, err := core.ClassSpanned(sel, cls)
			if err != nil || !spanned {
				continue
			}
		}
		return cls
	}
	return ""
}

// StylePick applies the picked style exclusively among named styles:
// other named classes leave the selection, wt-* direct formatting stays.
// The empty pick (the "(none)" option) just clears.
func StylePick(core EditorCore, name string) error {
	sel, ok := core.Sel()
	if !ok {
		return nil
	}
	return core.Txn(func(c EditorCore) error {
		for _, cls := range c.Classes() {
			if strings.HasPrefix(cls, "wt-") || cls == name {
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

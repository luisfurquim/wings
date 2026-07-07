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
			ID: "style-new", Label: "wtext-style-new", Icon: "style_new",
			Placeholder: "wtext-style-name",
			Do:          CreateStyle,
		},
		SelectItem{
			ID: "style", Label: "wtext-style",
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
	return core.Txn(func(c EditorCore) error {
		for _, cls := range classes {
			if cls == name {
				continue
			}
			if err := c.RemoveClass(sel, cls); err != nil {
				return err
			}
		}
		return c.ApplyClass(sel, name)
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

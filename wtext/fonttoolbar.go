package wtext

import (
	"fmt"
	"strings"
)

// FontToolbar is the stock character/paragraph formatting plugin: font
// face and font size pickers plus the four paragraph alignment toggles.
// Faces and Sizes are configurable; the zero value uses the stock generic
// families and the em-based size ladder.
//
// No choice ever becomes an inline style: each one is a deterministic
// utility class (wt-ff-<id>, wt-fs-<id>, wt-al-<dir>) defined at attach
// time (InitPlugin) and applied with the Word split — face and size ride
// spans over the exact range, alignment marks the touched blocks. One
// class per axis: picking another face replaces the previous one.
type FontToolbar struct {
	Faces []FontFace
	Sizes []FontSize
}

// FontFace is one font-family choice. ID names the utility class
// (wt-ff-<ID>, so it must satisfy the class-name rules); Label is the
// option text (an i18n message id, or shown as-is); Family is the CSS
// font-family stack, validated through the CSS sanitizer at Init.
type FontFace struct {
	ID, Label, Family string
}

// FontSize is one font-size choice (utility class wt-fs-<ID>).
type FontSize struct {
	ID, Label, Size string
}

// Utility class namespaces. The wt- prefix as a whole is reserved:
// CreateStyle refuses user styles inside it.
const (
	facePrefix  = "wt-ff-"
	sizePrefix  = "wt-fs-"
	alignPrefix = "wt-al-"
)

// DefaultFontFaces is the stock face list: the CSS generic families plus
// a curated set of web-safe named stacks. Every stack ends in a generic,
// so it is device-safe and EPUB-safe — a reading system without the
// named face substitutes its own — and carries no licensing strings
// attached. Font names are proper nouns: their Labels are shown as-is,
// not translated.
func DefaultFontFaces() []FontFace {
	return []FontFace{
		{ID: "serif", Label: "wtext-font-serif", Family: "serif"},
		{ID: "sans", Label: "wtext-font-sans", Family: "sans-serif"},
		{ID: "mono", Label: "wtext-font-mono", Family: "monospace"},
		{ID: "cursive", Label: "wtext-font-cursive", Family: "cursive"},
		{ID: "georgia", Label: "Georgia", Family: "Georgia, 'Times New Roman', serif"},
		{ID: "palatino", Label: "Palatino", Family: "'Palatino Linotype', Palatino, serif"},
		{ID: "times", Label: "Times New Roman", Family: "'Times New Roman', Times, serif"},
		{ID: "arial", Label: "Arial", Family: "Arial, Helvetica, sans-serif"},
		{ID: "verdana", Label: "Verdana", Family: "Verdana, Geneva, sans-serif"},
		{ID: "trebuchet", Label: "Trebuchet MS", Family: "'Trebuchet MS', Tahoma, sans-serif"},
		{ID: "courier", Label: "Courier New", Family: "'Courier New', Courier, monospace"},
	}
}

// DefaultFontSizes is the stock size ladder, in em so nested content and
// EPUB readers rescale it; the labels are language-neutral percentages.
func DefaultFontSizes() []FontSize {
	return []FontSize{
		{ID: "75", Label: "75%", Size: "0.75em"},
		{ID: "90", Label: "90%", Size: "0.9em"},
		{ID: "100", Label: "100%", Size: "1em"},
		{ID: "125", Label: "125%", Size: "1.25em"},
		{ID: "150", Label: "150%", Size: "1.5em"},
		{ID: "200", Label: "200%", Size: "2em"},
	}
}

// alignments are the four paragraph alignments, in toolbar order. The
// class value doubles as the CSS text-align value.
var alignments = []struct{ dir, label, icon, help string }{
	{"left", "wtext-align-left", "format_align_left", "wtext-align-left-help"},
	{"center", "wtext-align-center", "format_align_center", "wtext-align-center-help"},
	{"right", "wtext-align-right", "format_align_right", "wtext-align-right-help"},
	{"justify", "wtext-align-justify", "format_align_justify", "wtext-align-justify-help"},
}

// webFontAwarePick is ClassPick(facePrefix) plus lazy class definition:
// a runtime-installed webfont has no attach-time Init to define its
// wt-ff-* class, so the first pick defines it here.
func webFontAwarePick() func(EditorCore, string) error {
	pick := ClassPick(facePrefix)
	return func(core EditorCore, value string) error {
		if value != "" {
			if err := defineWebFontClass(core, value); err != nil {
				return err
			}
		}
		return pick(core, value)
	}
}

// defineWebFontClass ensures an installed webfont's utility class exists
// on this editor. A no-op for ids that are not installed webfonts or
// whose class is already defined.
func defineWebFontClass(core EditorCore, id string) error {
	name := facePrefix + id
	if _, ok := core.ClassCSS(name); ok {
		return nil
	}
	wf, found := webFontByID(id)
	if !found {
		return nil
	}
	return core.DefineClass(name, "font-family: "+wf.Family)
}

func (t FontToolbar) faces() []FontFace {
	if len(t.Faces) > 0 {
		return t.Faces
	}
	return DefaultFontFaces()
}

func (t FontToolbar) sizes() []FontSize {
	if len(t.Sizes) > 0 {
		return t.Sizes
	}
	return DefaultFontSizes()
}

// Init defines the utility classes before any content loads, so a
// persisted document that uses them survives the class filter. IDs and
// CSS values are webdev input and go through the DefineClass validators —
// a bad face fails loudly here, not silently at pick time.
func (t FontToolbar) Init(core EditorCore) error {
	for _, f := range t.faces() {
		if err := core.DefineClass(facePrefix+f.ID, "font-family: "+f.Family); err != nil {
			return err
		}
	}
	for _, s := range t.sizes() {
		if err := core.DefineClass(sizePrefix+s.ID, "font-size: "+s.Size); err != nil {
			return err
		}
	}
	for _, a := range alignments {
		if err := core.DefineClass(alignPrefix+a.dir, "text-align: "+a.dir); err != nil {
			return err
		}
	}
	return nil
}

// Items declares the two pickers and the four alignment toggles.
func (t FontToolbar) Items() []ToolbarItem {
	faceOpts := []Option{{Value: "", Label: "wtext-font-default"}}
	for _, f := range t.faces() {
		faceOpts = append(faceOpts, Option{Value: f.ID, Label: f.Label, Font: f.Family})
	}
	sizeOpts := []Option{{Value: "", Label: "wtext-size-default"}}
	for _, s := range t.sizes() {
		sizeOpts = append(sizeOpts, Option{Value: s.ID, Label: s.Label})
	}
	items := []ToolbarItem{
		SelectItem{
			ID: "fontface", Label: "wtext-font", Help: "wtext-font-help",
			// Webfonts installed at runtime (AddFont / a store URL dropped
			// into this very picker) join the configured faces on every
			// refresh — Options is dynamic like the style picker's.
			Options: func(EditorCore) []Option {
				opts := append([]Option(nil), faceOpts...)
				configured := map[string]bool{}
				for _, f := range t.faces() {
					configured[f.ID] = true
				}
				for _, wf := range InstalledWebFonts() {
					if configured[wf.ID] {
						continue
					}
					opts = append(opts, Option{Value: wf.ID, Label: wf.Label, Font: wf.Family})
				}
				return opts
			},
			Current: ClassCurrent(facePrefix),
			Pick:    webFontAwarePick(),
			// The URL door: Enter on text no option matches. Only store
			// URLs pass (allowlist, deny state); the loaded font applies
			// to the remembered selection and appears in the picker.
			NotInList: func(core EditorCore, text string) error {
				if loadWebFont == nil {
					return fmt.Errorf("wtext: webfont loading unavailable")
				}
				text = strings.TrimSpace(text)
				// A pasted URL sometimes arrives GLUED to the field's
				// previous label ("Lobsterhttps://..."): an async repaint
				// can kill the input's select-all right before the paste,
				// turning replace into append. Salvage the URL — the
				// store allowlist still has the final word on it.
				if i := strings.Index(text, "https://"); i > 0 {
					text = text[i:]
				}
				if !strings.HasPrefix(text, "https://") {
					return nil // ordinary typing, not a URL drop
				}
				loadWebFont(text, func(family string, err error) {
					if err != nil {
						return // the loader logged it
					}
					id := fontSlug(family)
					if err := defineWebFontClass(core, id); err != nil {
						return
					}
					_ = SwapClass(core, facePrefix, facePrefix+id)
				})
				return nil
			},
		},
		SelectItem{
			ID: "fontsize", Label: "wtext-size", Help: "wtext-size-help",
			Options: func(EditorCore) []Option { return sizeOpts },
			Current: ClassCurrent(sizePrefix),
			Pick:    ClassPick(sizePrefix),
		},
		Separator{},
	}
	for _, a := range alignments {
		items = append(items, ToggleItem{
			ID: "align-" + a.dir, Label: a.label, Icon: a.icon, Help: a.help,
			Do:     ClassToggle(alignPrefix, alignPrefix+a.dir),
			Active: alignActive(a.dir),
		})
	}
	return items
}

// alignActive lights the toggle whose alignment is in effect. A block
// with no wt-al-* class sits on the default alignment, shown on the left
// toggle — but only while a selection exists (no selection, no state).
func alignActive(dir string) func(EditorCore) bool {
	return func(core EditorCore) bool {
		sel, ok := core.Sel()
		if !ok {
			return false
		}
		classes, err := core.ClassesAt(sel)
		if err != nil {
			return false
		}
		current := ""
		for _, cls := range classes {
			if strings.HasPrefix(cls, alignPrefix) {
				current = strings.TrimPrefix(cls, alignPrefix)
				break
			}
		}
		if current == "" {
			return dir == "left"
		}
		return current == dir
	}
}

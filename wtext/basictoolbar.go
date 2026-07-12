package wtext

// boldClass and italicClass are Bold/Italic's utility classes
// (font-weight/font-style) — visual weight, not semantic markup. Word's
// own Bold/Italic buttons are a formatting toggle, not an assertion of
// "strong importance" or "emphasis"; conflating the two meant a named
// style captured from bold text silently lost the bold on reapplication,
// since CreateStyle only ever captured CSS classes, never mark elements.
// <strong>/<em> stay in the content profile (epubhtml) and keep working
// for pasted or loaded content that already carries them — genuinely
// semantic bold/italic (from Word documents, external HTML) round-trips
// fine; BasicToolbar's own Bold/Italic buttons simply stop producing new
// ones. But the buttons still need to SEE that mark: text pasted from an
// external source may arrive as <strong> rather than wt-b, and a Bold
// button blind to that would read it as inactive and be unable to turn
// it off — DualMarkToggle checks and clears either representation, only
// ever writing the class for new formatting. code stays a real semantic
// mark: a code span is structural, not a font-weight, and has no
// meaningful CSS-only substitute.
const (
	boldClass      = "wt-b"
	italicClass    = "wt-i"
	underlineClass = "wt-u"
)

// BasicToolbar is the stock ToolbarPlugin: bold/italic/code toggles plus a
// block-style picker. It is deliberately thin — pure declarations over the
// exported helpers — both as the default experience and as the reference
// for writing custom toolbars. Labels are message ids resolved through the
// widget's i18n catalog.
type BasicToolbar struct{}

// Init defines Bold/Italic's utility classes before any content loads, so
// a persisted document using them survives the class filter.
func (BasicToolbar) Init(core EditorCore) error {
	if err := core.DefineClass(boldClass, "font-weight: bold"); err != nil {
		return err
	}
	if err := core.DefineClass(italicClass, "font-style: italic"); err != nil {
		return err
	}
	return core.DefineClass(underlineClass, "text-decoration: underline")
}

// Items declares the stock items.
func (BasicToolbar) Items() []ToolbarItem {
	return []ToolbarItem{
		ToggleItem{
			ID: "bold", Label: "wtext-bold", Icon: "format_bold", Help: "wtext-bold-help",
			Do: DualMarkToggle(boldClass, "strong"), Active: DualMarkActive(boldClass, "strong"),
		},
		ToggleItem{
			ID: "italic", Label: "wtext-italic", Icon: "format_italic", Help: "wtext-italic-help",
			Do: DualMarkToggle(italicClass, "em"), Active: DualMarkActive(italicClass, "em"),
		},
		ToggleItem{
			ID: "underline", Label: "wtext-underline", Icon: "format_underlined", Help: "wtext-underline-help",
			Do: DualMarkToggle(underlineClass, "u"), Active: DualMarkActive(underlineClass, "u"),
		},
		ToggleItem{
			ID: "code", Label: "wtext-code", Icon: "code", Help: "wtext-code-help",
			Do: MarkToggle(Code()), Active: MarkActive("code"),
		},
		Separator{},
		SelectItem{
			ID: "block", Label: "wtext-block", Help: "wtext-block-help",
			Options: func(EditorCore) []Option {
				return []Option{
					{Value: "p", Label: "wtext-block-p"},
					{Value: "h1", Label: "wtext-block-h1"},
					{Value: "h2", Label: "wtext-block-h2"},
					{Value: "h3", Label: "wtext-block-h3"},
					{Value: "h4", Label: "wtext-block-h4"},
					{Value: "h5", Label: "wtext-block-h5"},
					{Value: "h6", Label: "wtext-block-h6"},
					{Value: "blockquote", Label: "wtext-block-quote"},
					{Value: "pre", Label: "wtext-block-pre"},
				}
			},
			Current: BlockCurrent(),
			Pick:    BlockPick(),
		},
	}
}

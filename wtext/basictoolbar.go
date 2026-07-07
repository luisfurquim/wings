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
// ones. code stays a real semantic mark: a code span is structural, not
// a font-weight, and has no meaningful CSS-only substitute.
const (
	boldClass   = "wt-b"
	italicClass = "wt-i"
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
	return core.DefineClass(italicClass, "font-style: italic")
}

// Items declares the stock items.
func (BasicToolbar) Items() []ToolbarItem {
	return []ToolbarItem{
		ToggleItem{
			ID: "bold", Label: "wtext-bold", Icon: "format_bold",
			Do: ClassMarkToggle(boldClass), Active: ClassMarkActive(boldClass),
		},
		ToggleItem{
			ID: "italic", Label: "wtext-italic", Icon: "format_italic",
			Do: ClassMarkToggle(italicClass), Active: ClassMarkActive(italicClass),
		},
		ToggleItem{
			ID: "code", Label: "wtext-code", Icon: "code",
			Do: MarkToggle(Code()), Active: MarkActive("code"),
		},
		Separator{},
		SelectItem{
			ID: "block", Label: "wtext-block",
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

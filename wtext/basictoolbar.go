package wtext

// BasicToolbar is the stock ToolbarPlugin: bold/italic/code toggles plus a
// block-style picker. It is deliberately thin — pure declarations over the
// exported helpers — both as the default experience and as the reference
// for writing custom toolbars. Labels are message ids resolved through the
// widget's i18n catalog.
type BasicToolbar struct{}

// Items declares the stock items.
func (BasicToolbar) Items() []ToolbarItem {
	return []ToolbarItem{
		ToggleItem{
			ID: "bold", Label: "wtext-bold", Icon: "format_bold",
			Do: MarkToggle(Strong()), Active: MarkActive("strong"),
		},
		ToggleItem{
			ID: "italic", Label: "wtext-italic", Icon: "format_italic",
			Do: MarkToggle(Em()), Active: MarkActive("em"),
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

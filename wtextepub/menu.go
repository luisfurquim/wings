//go:build js && wasm

package wtextepub

import (
	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/wtext"
)

// Menu is the export entry: a MenuPlugin whose single action, under the
// standard wtext-export group, builds the EPUB from the editor's
// persisted document and hands it to the browser as a download. It is
// the js half of the package — the portable Build carries all the logic
// and the tests.
//
// Menu plugins render through the w-tabs accordion, registered by the
// app, not by w-text: a profile using this plugin also needs
// import _ ".../widget/tabs", ".../widget/tabbutton", ".../widget/tab".
// The item label id (wtext-epub) has no built-in default in the widget
// (this is not a stock plugin); provide it the same way as any other
// label: <span slot="labels" id="wtext-epub">… in the host's light DOM,
// swept by gen_i18n. The group id (wtext-export) does have a built-in
// default.
type Menu struct {
	Cfg Config
}

// ConfigSections declares the book-metadata settings page (title, author,
// publisher), seeded by Cfg — the iOS model: the plugin registers its
// schema, the widget renders the central settings UI, the USER edits the
// values and they persist inside the document. Add the same Menu instance
// to Profile.Config for the section to appear.
func (m Menu) ConfigSections() []wtext.ConfigSection {
	return []wtext.ConfigSection{{
		ID:    "epub",
		Label: "wtext-epub-config",
		Help:  "wtext-epub-config-help",
		Fields: []wtext.ConfigField{
			wtext.ConfigText{ID: "title", Label: "wtext-epub-title", Default: m.Cfg.Title},
			wtext.ConfigText{ID: "author", Label: "wtext-epub-author", Default: m.Cfg.Author},
			wtext.ConfigText{ID: "publisher", Label: "wtext-epub-publisher", Default: m.Cfg.Publisher},
		},
	}}
}

// bookConfig assembles the export-time Config from the document store
// (user edits win; the store already falls back to the declared
// defaults), guarded by the constructor Cfg for profiles that registered
// the Menu without its config section.
func (m Menu) bookConfig(core wtext.EditorCore) Config {
	or := func(v, fallback string) string {
		if v != "" {
			return v
		}
		return fallback
	}
	return Config{
		Title:     or(core.Config("epub.title"), m.Cfg.Title),
		Author:    or(core.Config("epub.author"), m.Cfg.Author),
		Publisher: or(core.Config("epub.publisher"), m.Cfg.Publisher),
	}
}

// MenuItems declares the export action: a MenuInput whose prompt asks
// the document name, Save-As style — seeded with the book title (Enter
// keeps it, typing replaces it). The typed string names the document AS
// IS (the TOC entry, the content page's <title>); its Filename-sanitized
// form is only the download name.
func (m Menu) MenuItems() []wtext.MenuItem {
	return []wtext.MenuItem{
		wtext.MenuInput{
			Group:       "wtext-export",
			ID:          "epub",
			Label:       "wtext-epub",
			Placeholder: "wtext-epub-name",
			Help:        "wtext-epub-help",
			Value: func(core wtext.EditorCore) string {
				return m.bookConfig(core).Title
			},
			Do: func(core wtext.EditorCore, name string) error {
				b, err := Build(core.Content(), wings.Locale, name, m.bookConfig(core))
				if err != nil {
					return err
				}
				download(Filename(name), b)
				return nil
			},
		},
	}
}

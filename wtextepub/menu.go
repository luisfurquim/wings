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

// usedWebFonts collects the store fonts the document actually uses, as
// embeddable files. The store curation (libre catalogs) is what makes the
// embed legitimate.
//
// A document uses a font in one of two ways, and both count. The picker
// applies one through its `wt-ff-<id>` class, which shows up in the
// markup. A document that arrived from somewhere else — an imported book
// — names the family in its OWN rules instead (`font-family: "Lobster"`),
// and its font was installed from the store by that name: a book that
// came in with embedded fonts must go back out with them, or a round trip
// through this editor would quietly strip a book of its typography.
func usedWebFonts(content string) []EmbeddedFont {
	used := fontUsage(content)
	var out []EmbeddedFont
	for _, wf := range wtext.InstalledWebFonts() {
		if !used(wf.ID, wf.Label) {
			continue
		}
		for _, s := range wf.Sources {
			out = append(out, EmbeddedFont{
				Family: wf.Label, Style: s.Style, Weight: s.Weight,
				Range: s.Range, Format: s.Format, Data: s.Data,
			})
		}
	}
	return out
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

// MenuItems declares the two halves of the book round trip.
//
// Export is a MenuInput whose prompt asks the document name, Save-As
// style — seeded with the book title (Enter keeps it, typing replaces
// it). The typed string names the document AS IS (the TOC entry, the
// content page's <title>); its Filename-sanitized form is only the
// download name.
//
// Import is a MenuUpload: the widget owns the file picker and hands over
// the bytes, which importAction treats as the hostile input they are. Its
// label ids have no built-in default in the widget either (this is not a
// stock plugin) — supply them through the host's labels slot.
func (m Menu) MenuItems() []wtext.MenuItem {
	return []wtext.MenuItem{
		wtext.MenuUpload{
			Group:  "wtext-import",
			ID:     "epub",
			Label:  "wtext-epub-import",
			Help:   "wtext-epub-import-help",
			Accept: ".epub,application/epub+zip",
			MaxLen: MaxImportBytes,
			Do:     importAction,
		},
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
				content := core.Content()
				cfg := m.bookConfig(core)
				cfg.Fonts = usedWebFonts(content)
				b, err := Build(content, wings.Locale, name, cfg)
				if err != nil {
					return err
				}
				download(Filename(name), b)
				return nil
			},
		},
	}
}

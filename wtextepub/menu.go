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

// MenuItems declares the export action.
func (m Menu) MenuItems() []wtext.MenuItem {
	return []wtext.MenuItem{
		wtext.MenuAction{
			Group: "wtext-export",
			ID:    "epub",
			Label: "wtext-epub",
			Help:  "wtext-epub-help",
			Do: func(core wtext.EditorCore) error {
				b, err := Build(core.Content(), wings.Locale, m.Cfg)
				if err != nil {
					return err
				}
				download(Filename(m.Cfg.Title), b)
				return nil
			},
		},
	}
}

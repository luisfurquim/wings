package wtext

import "testing"

// installFake seeds the registry with a font as if a store had just
// loaded it (the fetch itself is the js half's, unreachable natively).
func installFake(t *testing.T, family, storeURL string) WebFont {
	t.Helper()
	resetFontState(t)
	wf := WebFont{
		ID:       fontSlug(family),
		Label:    family,
		Family:   `"` + family + `"`,
		StoreURL: storeURL,
		Sources:  []WebFontSource{{Style: "normal", Weight: "400", Format: "woff2"}},
	}
	if err := registerWebFont(wf); err != nil {
		t.Fatal(err)
	}
	return wf
}

// TestRememberWebFontPersistsTheReference: the document remembers WHERE a
// user-installed font came from, so reopening it can bring the font back —
// the class rule alone only says which font the text wants.
func TestRememberWebFontPersistsTheReference(t *testing.T) {
	wf := installFake(t, "Lobster", "https://fonts.googleapis.com/css2?family=Lobster&display=swap")
	core := &fakeCore{}
	rememberWebFont(core, wf.ID)

	if got := core.Config(webFontCfgPrefix + wf.ID); got != wf.StoreURL {
		t.Errorf("stored %q, want the store URL %q", got, wf.StoreURL)
	}
	// A font nobody installed leaves no trace.
	rememberWebFont(core, "not-installed")
	if got := core.Config(webFontCfgPrefix + "not-installed"); got != "" {
		t.Errorf("unknown font persisted as %q", got)
	}
}

// TestStyleLibCarriesTheFontOfItsStyles: exporting a style that uses a
// webfont exports the REFERENCE to that font (never its bytes), and only
// for the fonts the exported styles actually name.
func TestStyleLibCarriesTheFontOfItsStyles(t *testing.T) {
	wf := installFake(t, "Lobster", "https://fonts.googleapis.com/css2?family=Lobster&display=swap")
	unused := WebFont{ID: "inter", Label: "Inter", Family: `"Inter"`,
		StoreURL: "https://fonts.googleapis.com/css2?family=Inter"}
	if err := registerWebFont(unused); err != nil {
		t.Fatal(err)
	}

	core := &fakeCore{}
	if err := core.DefineClass("titulo", "font-family: "+wf.Family+"; font-size: 2em"); err != nil {
		t.Fatal(err)
	}
	if err := core.DefineClass("nota", "color: gray"); err != nil {
		t.Fatal(err)
	}

	lib := CollectStyleLib(core)
	if len(lib.Fonts) != 1 {
		t.Fatalf("fonts = %v, want only the one a style names", lib.Fonts)
	}
	if lib.Fonts[0].Family != "Lobster" || lib.Fonts[0].URL != wf.StoreURL {
		t.Errorf("font = %#v, want the Lobster store reference", lib.Fonts[0])
	}
	// The file carries a reference, not a font: no bytes cross it.
	data, err := lib.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 4096 {
		t.Errorf("library file is %d bytes — a font's bytes leaked into it?", len(data))
	}
}

// TestReferencedWebFontQuotesAreLoadBearing: "Roboto" must not match the
// style of a document using "Roboto Slab" — the quotes are what makes the
// containment check exact.
func TestReferencedWebFontQuotesAreLoadBearing(t *testing.T) {
	installFake(t, "Roboto", "https://fonts.googleapis.com/css2?family=Roboto")
	fonts := referencedWebFonts([]LibStyle{{Name: "t", CSS: `font-family: "Roboto Slab", serif`}})
	if len(fonts) != 0 {
		t.Errorf("fonts = %v, want none: \"Roboto\" is not \"Roboto Slab\"", fonts)
	}
	fonts = referencedWebFonts([]LibStyle{{Name: "t", CSS: `font-family: "Roboto", sans-serif`}})
	if len(fonts) != 1 {
		t.Errorf("fonts = %v, want the exact family matched", fonts)
	}
}

package wtext

import (
	"errors"
	"strings"
	"testing"
)

// resetFontState restores the package's webfont globals after a test
// that mutates them (deny/disable have no user-facing undo — fail
// closed — so tests reset the internals directly).
func resetFontState(t *testing.T) {
	t.Cleanup(func() {
		fontMu.Lock()
		deniedStores = map[string]bool{}
		webFontsDisabled = false
		webFonts = nil
		clear(embeddedFamilies)
		fontMu.Unlock()
	})
}

func TestParseFontURL(t *testing.T) {
	resetFontState(t)
	cases := map[string]struct{ store, family string }{
		"https://fonts.google.com/specimen/Roboto":                   {"google", "Roboto"},
		"https://fonts.google.com/specimen/Roboto+Mono":              {"google", "Roboto Mono"},
		"https://fonts.google.com/specimen/Open+Sans/tester":         {"google", "Open Sans"},
		"https://fonts.googleapis.com/css2?family=Lato:wght@400;700": {"google", "Lato"},
		"https://fonts.googleapis.com/css?family=Lobster|Roboto":     {"google", "Lobster"},
		"https://fonts.bunny.net/css?family=abel:400":                {"bunny", "abel"},
		"https://fonts.bunny.net/family/fira-sans":                   {"bunny", "fira-sans"},
		" https://fonts.google.com/specimen/Inter ":                  {"google", "Inter"},
	}
	for raw, want := range cases {
		p, err := parseFontURL(raw)
		if err != nil {
			t.Errorf("parseFontURL(%q): %v", raw, err)
			continue
		}
		if p.store.name != want.store || p.family != want.family {
			t.Errorf("parseFontURL(%q) = %s/%q, want %s/%q", raw, p.store.name, p.family, want.store, want.family)
		}
		if !strings.HasPrefix(p.cssURL, "https://") {
			t.Errorf("cssURL not https: %q", p.cssURL)
		}
	}
	for _, bad := range []string{
		"http://fonts.google.com/specimen/Roboto", // https only
		"https://fonts.evil.com/specimen/Roboto",
		"https://fonts.google.com/",
		"https://fonts.googleapis.com/css2",
		"not a url",
		"https://fonts.google.com/specimen/" + strings.Repeat("x", maxFontURLLen),
	} {
		if _, err := parseFontURL(bad); err == nil {
			t.Errorf("parseFontURL(%q) accepted", bad)
		}
	}
}

func TestFontStoreDeny(t *testing.T) {
	resetFontState(t)
	if err := DenyFontStore("typosquat"); !errors.Is(err, ErrFontStoreUnknown) {
		t.Errorf("unknown store deny = %v", err)
	}
	if err := DenyFontStore("google"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseFontURL("https://fonts.google.com/specimen/Roboto"); !errors.Is(err, ErrFontStoreDenied) {
		t.Errorf("denied store = %v, want ErrFontStoreDenied", err)
	}
	if _, err := parseFontURL("https://fonts.bunny.net/css?family=abel"); err != nil {
		t.Errorf("bunny must stay allowed: %v", err)
	}
	if got := FontStoreNames(); len(got) != 1 || got[0] != "bunny" {
		t.Errorf("FontStoreNames = %v", got)
	}
	DisableWebFonts()
	if _, err := parseFontURL("https://fonts.bunny.net/css?family=abel"); !errors.Is(err, ErrWebFontsDisabled) {
		t.Errorf("disabled = %v, want ErrWebFontsDisabled", err)
	}
	if got := FontStoreNames(); len(got) != 0 {
		t.Errorf("FontStoreNames after disable = %v", got)
	}
}

// sampleCSS mirrors a css2 response's shape: multiple subsets per
// (style, weight) — the dedup keeps the first of each — plus a hostile
// off-host url that must not survive.
const sampleCSS = `
/* latin-ext */
@font-face {
  font-family: 'Lato';
  font-style: normal;
  font-weight: 400;
  src: url(https://fonts.gstatic.com/s/lato/v24/latin-ext.woff2) format('woff2');
  unicode-range: U+0100-02BA, U+1E00-1EFF;
}
/* latin */
@font-face {
  font-family: 'Lato';
  font-style: normal;
  font-weight: 400;
  src: url(https://fonts.gstatic.com/s/lato/v24/latin.woff2) format('woff2');
  unicode-range: U+0000-00FF;
}
@font-face {
  font-family: 'Lato';
  font-style: italic;
  font-weight: 700;
  src: url(https://evil.example/x.woff2) format('woff2'), url('https://fonts.gstatic.com/s/lato/v24/it700.woff2') format('woff2');
}
`

func TestParseFontFaceCSS(t *testing.T) {
	resetFontState(t)
	st := fontStores[0] // google
	srcs, err := parseFontFaceCSS(sampleCSS, st)
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 3 {
		t.Fatalf("sources = %d, want 3 (EVERY subset kept, per unicode-range): %+v", len(srcs), srcs)
	}
	if srcs[0].Range != "U+0100-02BA, U+1E00-1EFF" || srcs[1].Range != "U+0000-00FF" {
		t.Errorf("unicode-ranges lost: %+v", srcs[:2])
	}
	if srcs[2].Style != "italic" || srcs[2].Weight != "700" ||
		srcs[2].URL != "https://fonts.gstatic.com/s/lato/v24/it700.woff2" {
		t.Errorf("hostile url must be skipped, on-host kept: %+v", srcs[2])
	}
	if _, err := parseFontFaceCSS("@font-face { src: url(https://evil.example/a.woff2); }", st); !errors.Is(err, ErrFontCSS) {
		t.Errorf("all-hostile css = %v, want ErrFontCSS", err)
	}
	if _, err := parseFontFaceCSS(strings.Repeat("x", maxFontCSSLen+1), st); !errors.Is(err, ErrFontCSS) {
		t.Errorf("oversized css = %v, want ErrFontCSS", err)
	}
}

func TestWebFontRegistry(t *testing.T) {
	resetFontState(t)
	notified := 0
	unsub := OnWebFontsChanged(func() { notified++ })
	f := WebFont{ID: "lato", Label: "Lato", Family: `"Lato"`,
		Sources: []WebFontSource{{Style: "normal", Weight: "400"}}}
	if err := registerWebFont(f); err != nil {
		t.Fatal(err)
	}
	if notified != 1 {
		t.Errorf("watcher notified %d times, want 1", notified)
	}
	if got, ok := webFontByID("lato"); !ok || got.Label != "Lato" {
		t.Errorf("webFontByID = %+v, %v", got, ok)
	}
	// Replace by ID keeps the registry size.
	f.Label = "Lato v2"
	if err := registerWebFont(f); err != nil {
		t.Fatal(err)
	}
	if list := InstalledWebFonts(); len(list) != 1 || list[0].Label != "Lato v2" {
		t.Errorf("replace by ID failed: %+v", list)
	}
	// Unsubscribed watchers stay quiet.
	unsub()
	_ = registerWebFont(WebFont{ID: "abel", Label: "Abel"})
	if notified != 2 {
		t.Errorf("watcher after replace+unsub = %d, want 2", notified)
	}
	// The registry is bounded.
	for i := 0; len(InstalledWebFonts()) < maxWebFonts; i++ {
		if err := registerWebFont(WebFont{ID: fontSlug(strings.Repeat("x", i+1)), Label: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := registerWebFont(WebFont{ID: "overflow", Label: "o"}); err == nil {
		t.Error("registry over maxWebFonts must refuse")
	}
}

// TestFontNotInListSalvage pins the paste-glue fix: a URL that arrives
// concatenated to the picker's previous label (an async repaint killing
// the select-all turned replace into append) must still reach the loader
// as a clean URL; plain typing must stay ignored.
func TestFontNotInListSalvage(t *testing.T) {
	resetFontState(t)
	var got []string
	prev := loadWebFont // non-nil under js/wasm (AddFont); restore it
	loadWebFont = func(u string, _ func(string, error)) { got = append(got, u) }
	t.Cleanup(func() { loadWebFont = prev })

	sel, ok := FontToolbar{}.Items()[0].(SelectItem)
	if !ok || sel.ID != "fontface" || sel.NotInList == nil {
		t.Fatalf("Items()[0] is not the fontface SelectItem with NotInList")
	}
	const url = "https://fonts.google.com/specimen/Lobster"
	for _, in := range []string{url, "Lobster" + url, "  Fonte padrão" + url + "  "} {
		if err := sel.NotInList(nil, in); err != nil {
			t.Fatalf("NotInList(%q): %v", in, err)
		}
	}
	if len(got) != 3 {
		t.Fatalf("loader called %d times, want 3: %v", len(got), got)
	}
	for _, u := range got {
		if u != url {
			t.Errorf("loader got %q, want %q", u, url)
		}
	}
	if err := sel.NotInList(nil, "Verdana Pro"); err != nil || len(got) != 3 {
		t.Errorf("plain typing must be ignored: err=%v calls=%d", err, len(got))
	}
}

func TestFontSlug(t *testing.T) {
	cases := map[string]string{
		"Roboto":       "roboto",
		"Open Sans":    "open-sans",
		"Fira Sans 2":  "fira-sans-2",
		"Ação & Cores": "a-o-cores",
		"123":          "f-123",
		"":             "f-",
	}
	for in, want := range cases {
		if got := fontSlug(in); got != want {
			t.Errorf("fontSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestAddDocumentFontSkipsRemembered: a document that already remembers a
// font is having it installed from its own store URL (restoreWebFonts,
// asynchronous). Asking the store again by name would fetch the same
// files twice — which is exactly what importing a book exported from here
// would do, since such a book carries both the property and the files.
func TestAddDocumentFontSkipsRemembered(t *testing.T) {
	prev := loadWebFont
	defer func() { loadWebFont = prev }()
	asked := 0
	loadWebFont = func(string, func(string, error)) { asked++ }

	core := &fakeCore{cfg: map[string]string{
		webFontCfgPrefix + fontSlug("Lobster"): "https://fonts.googleapis.com/css2?family=Lobster",
	}}
	var err error
	AddDocumentFont(core, "Lobster", func(e error) { err = e })
	if err != nil {
		t.Errorf("err = %v, want nil (the restore path owns this font)", err)
	}
	if asked != 0 {
		t.Errorf("asked the store %d time(s) for a font the document remembers", asked)
	}

	// A font the document does NOT remember still goes to the store.
	AddDocumentFont(&fakeCore{}, "EB Garamond", func(error) {})
	if asked != 1 {
		t.Errorf("store asks = %d, want 1 for an unknown family", asked)
	}
}

// TestStoreURLForFamily: the name comes out of a file, so it is checked
// before it becomes a URL.
func TestStoreURLForFamily(t *testing.T) {
	got, err := StoreURLForFamily(`"EB Garamond"`)
	if err != nil || !strings.Contains(got, "EB+Garamond") {
		t.Errorf("StoreURLForFamily = %q, %v", got, err)
	}
	for _, bad := range []string{"", "   ", `Foo"); }`, "Foo\nBar", strings.Repeat("x", 200)} {
		if _, err := StoreURLForFamily(bad); err == nil {
			t.Errorf("StoreURLForFamily(%q) was accepted", bad)
		}
	}
}

// TestMarkDocumentFontWaitsForTheFont: the two ways a document's font
// arrives land at different times, so the provenance mark has to be able
// to wait for one that is still installing — the case of a book exported
// from here and imported back, whose font comes from the remembered store
// URL, asynchronously.
func TestMarkDocumentFontWaitsForTheFont(t *testing.T) {
	resetFontState(t)
	clearDocumentFontMarks()
	MarkDocumentFont("Lobster")
	if err := registerWebFont(WebFont{ID: fontSlug("Lobster"), Label: "Lobster"}); err != nil {
		t.Fatal(err)
	}
	fonts := InstalledWebFonts()
	if len(fonts) != 1 || !fonts[0].Embedded {
		t.Fatalf("font = %+v, want Embedded", fonts)
	}
	// Re-registering (the same font installed again) must not forget it.
	if err := registerWebFont(WebFont{ID: fontSlug("Lobster"), Label: "Lobster"}); err != nil {
		t.Fatal(err)
	}
	if fonts = InstalledWebFonts(); !fonts[0].Embedded {
		t.Error("provenance was lost when the font re-registered")
	}
	// And a font nobody marked stays unmarked.
	if err := registerWebFont(WebFont{ID: fontSlug("EB Garamond"), Label: "EB Garamond"}); err != nil {
		t.Fatal(err)
	}
	for _, f := range InstalledWebFonts() {
		if f.Label == "EB Garamond" && f.Embedded {
			t.Error("a font nobody marked came out marked")
		}
	}
}

package wtext

// The webfont "store" system: a HARD-CODED allowlist of notoriously
// trustworthy font providers with libre catalogs — the trust decision
// lives here, in code, not in configuration. Fonts arrive through two
// doors, both funneled through the same parser and the same origin
// checks: the webdev calls AddFont (fontload.go, js) and the end user
// drops a store URL into the face picker. A webdev can only SHRINK the
// surface — DenyFontStore removes one store, DisableWebFonts closes the
// whole feature — never add an origin the allowlist doesn't carry.
// Licensing is an entry CRITERION of the list: both stores serve
// libre-licensed catalogs (OFL/Apache), which is what makes embedding
// the files in an exported EPUB legitimate.
//
// This file is portable: URL recognition and @font-face extraction are
// pure parsing over hostile input (fuzzed in fontstore_fuzz_test.go);
// the js half (fontload.go) only fetches and hands bytes to the
// browser's FontFace API.

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// Bounds — bounded everything: a hostile store response (or a hostile
// URL) must not grow memory without limit.
const (
	maxFontURLLen    = 2048
	maxFontCSSLen    = 512 << 10 // one store CSS response
	maxFontFaceRules = 32        // @font-face blocks per response
	MaxFontFileLen   = 8 << 20   // one font file (checked by the js fetch)
	maxWebFonts      = 64        // registry size
)

// Errors of the webfont pipeline.
var (
	// ErrWebFontsDisabled reports DisableWebFonts was called.
	ErrWebFontsDisabled = fmt.Errorf("wtext: webfonts are disabled")
	// ErrFontStoreDenied reports the URL's store was denied by the webdev.
	ErrFontStoreDenied = fmt.Errorf("wtext: font store denied")
	// ErrFontStoreUnknown reports a DenyFontStore name that matches no store.
	ErrFontStoreUnknown = fmt.Errorf("wtext: unknown font store")
	// ErrFontURL reports a URL no allowlisted store recognizes.
	ErrFontURL = fmt.Errorf("wtext: URL is not from an allowed font store")
	// ErrFontCSS reports a store CSS response the parser refuses.
	ErrFontCSS = fmt.Errorf("wtext: invalid font CSS")
)

// WebFontSource is one @font-face of a loaded webfont — a (style ×
// weight × unicode-range) combination. Stores split each variant into
// SUBSET files (latin, latin-ext, cyrillic…), each covering only its
// range's glyphs: all of them must load, each with its range, for the
// browser to compose the full coverage — the first cut of this parser
// deduplicated by (style, weight) and shipped a single subset, which
// "loaded" fine and then rendered fallback glyphs, letter by letter.
type WebFontSource struct {
	Style, Weight, Format string
	Range                 string // unicode-range, "" = all
	URL                   string // where the file came from (store CDN)
	Data                  []byte // the fetched bytes (display + EPUB embed)
}

// WebFont is one font installed through a store, as the registry and the
// face picker see it.
type WebFont struct {
	ID     string // face-picker id: class wt-ff-<ID>
	Label  string // display name (the family, shown as-is)
	Family string // CSS stack: "Family", generic fallback
	// StoreURL is the canonical store URL this font came from — the
	// REFERENCE that can re-install it: it is what a document persists
	// (so reopening it brings the font back) and what a style library
	// file carries (so importing a style brings its font along). Fonts
	// travel as this URL and never as bytes: the allowlist gets the final
	// word again every time one is followed.
	StoreURL string
	Sources  []WebFontSource
}

// fontStore is one hard-coded provider: how to recognize its URLs, where
// its files may live, and how to ask it for a family's CSS.
type fontStore struct {
	name     string
	parse    func(u *url.URL) (family string, ok bool)
	fileHost func(host string) bool
	cssURL   func(family string) string
}

// fontStores is THE allowlist. Google Fonts: specimen pages users copy
// (fonts.google.com/specimen/Roboto) and css2 API URLs; files on
// fonts.gstatic.com. Bunny Fonts: the GDPR-friendly mirror of the same
// libre catalog (fonts.bunny.net serves CSS and files on one host).
var fontStores = []fontStore{
	{
		name: "google",
		parse: func(u *url.URL) (string, bool) {
			switch u.Host {
			case "fonts.google.com":
				rest, ok := strings.CutPrefix(u.Path, "/specimen/")
				if !ok || rest == "" {
					return "", false
				}
				return strings.ReplaceAll(strings.SplitN(rest, "/", 2)[0], "+", " "), true
			case "fonts.googleapis.com":
				if u.Path != "/css" && u.Path != "/css2" {
					return "", false
				}
				fam := queryParam(u.RawQuery, "family")
				fam = strings.SplitN(fam, "|", 2)[0] // css v1 multi-family
				fam = strings.SplitN(fam, ":", 2)[0] // axis spec
				fam = strings.ReplaceAll(fam, "+", " ")
				return fam, fam != ""
			}
			return "", false
		},
		fileHost: func(h string) bool { return h == "fonts.gstatic.com" },
		cssURL: func(family string) string {
			return "https://fonts.googleapis.com/css2?family=" +
				strings.ReplaceAll(url.QueryEscape(family), "%20", "+") + "&display=swap"
		},
	},
	{
		name: "bunny",
		parse: func(u *url.URL) (string, bool) {
			if u.Host != "fonts.bunny.net" {
				return "", false
			}
			if fam, ok := strings.CutPrefix(u.Path, "/family/"); ok && fam != "" {
				return strings.SplitN(fam, "/", 2)[0], true
			}
			if u.Path == "/css" || u.Path == "/css2" {
				fam := strings.SplitN(queryParam(u.RawQuery, "family"), ":", 2)[0]
				return fam, fam != ""
			}
			return "", false
		},
		fileHost: func(h string) bool { return h == "fonts.bunny.net" },
		cssURL: func(family string) string {
			return "https://fonts.bunny.net/css?family=" + url.QueryEscape(family)
		},
	},
}

// loadWebFont is the loader hook the portable FontToolbar calls: nil
// under the native toolchain (declarations stay testable without a
// browser), assigned to AddFont by fontload.go's init on js.
var loadWebFont func(rawURL string, done func(family string, err error))

// ── deny state and registry ─────────────────────────────────────────────

var (
	fontMu           sync.Mutex
	deniedStores     = map[string]bool{}
	webFontsDisabled bool
	webFonts         []WebFont

	webFontWatchers   = map[int]func(){}
	nextFontWatcherID int
)

// OnWebFontsChanged subscribes to registry changes — how a face picker
// learns an asynchronous AddFont finished and refreshes its options.
// The returned func unsubscribes; call it when the subscriber goes away.
func OnWebFontsChanged(fn func()) (unsubscribe func()) {
	fontMu.Lock()
	id := nextFontWatcherID
	nextFontWatcherID++
	webFontWatchers[id] = fn
	fontMu.Unlock()
	return func() {
		fontMu.Lock()
		delete(webFontWatchers, id)
		fontMu.Unlock()
	}
}

// notifyWebFonts runs the watchers, outside the lock.
func notifyWebFonts() {
	fontMu.Lock()
	fns := make([]func(), 0, len(webFontWatchers))
	for _, fn := range webFontWatchers {
		fns = append(fns, fn)
	}
	fontMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// DenyFontStore removes one store from the allowlist for this app —
// both AddFont and the picker's URL drop stop accepting its URLs. It
// only shrinks: there is no way to add an origin the hard-coded list
// does not carry. Unknown names error, so a typo cannot silently deny
// nothing.
func DenyFontStore(name string) error {
	for _, st := range fontStores {
		if st.name == name {
			fontMu.Lock()
			deniedStores[name] = true
			fontMu.Unlock()
			return nil
		}
	}
	return fmt.Errorf("%w: %q", ErrFontStoreUnknown, name)
}

// DisableWebFonts closes the webfont feature entirely: every store is
// refused, AddFont fails, the picker's URL drop rejects everything.
// There is no re-enable — fail closed.
func DisableWebFonts() {
	fontMu.Lock()
	webFontsDisabled = true
	fontMu.Unlock()
}

// WebFontsEnabled reports whether the feature is still on.
func WebFontsEnabled() bool {
	fontMu.Lock()
	defer fontMu.Unlock()
	return !webFontsDisabled
}

// FontStoreNames lists the allowlisted stores still enabled.
func FontStoreNames() []string {
	fontMu.Lock()
	defer fontMu.Unlock()
	var names []string
	if webFontsDisabled {
		return names
	}
	for _, st := range fontStores {
		if !deniedStores[st.name] {
			names = append(names, st.name)
		}
	}
	return names
}

// InstalledWebFonts returns the registry (shared backing entries: treat
// as read-only).
func InstalledWebFonts() []WebFont {
	fontMu.Lock()
	defer fontMu.Unlock()
	out := make([]WebFont, len(webFonts))
	copy(out, webFonts)
	return out
}

// webFontCfgPrefix namespaces the document properties that remember an
// installed webfont: one wtfont.<id> per font, holding the store URL it
// can be re-installed from. They ride Content()'s head like any other
// property, so a document that uses a webfont reopens WITH it — the class
// rule alone (font-family: "Lobster") only says which font the text
// wants, not where the browser can get it.
const webFontCfgPrefix = "wtfont."

// rememberWebFont persists an installed webfont in the DOCUMENT, as the
// store reference it came from. Only fonts the USER installed are worth
// remembering (a URL dropped into the face picker, a style library
// imported): a font the webdev adds at boot is put back by the app on
// every load, and persisting it would only stale the document with a URL
// the app itself owns.
func rememberWebFont(core EditorCore, id string) {
	wf, ok := webFontByID(id)
	if !ok || wf.StoreURL == "" {
		return
	}
	if err := core.SetConfig(webFontCfgPrefix+id, wf.StoreURL); err != nil {
		G.Logf(1, "wtext: webfont %q not persisted: %v\n", id, err)
	}
}

// webFontByID looks an installed font up by its face-picker id.
func webFontByID(id string) (WebFont, bool) {
	fontMu.Lock()
	defer fontMu.Unlock()
	for _, f := range webFonts {
		if f.ID == id {
			return f, true
		}
	}
	return WebFont{}, false
}

// registerWebFont installs (or replaces, by ID) a loaded font and
// notifies the watchers.
func registerWebFont(f WebFont) error {
	fontMu.Lock()
	replaced := false
	for i := range webFonts {
		if webFonts[i].ID == f.ID {
			webFonts[i] = f
			replaced = true
			break
		}
	}
	if !replaced {
		if len(webFonts) >= maxWebFonts {
			fontMu.Unlock()
			return fmt.Errorf("wtext: webfont registry full (%d)", maxWebFonts)
		}
		webFonts = append(webFonts, f)
	}
	fontMu.Unlock()
	notifyWebFonts()
	return nil
}

// ── parsing ─────────────────────────────────────────────────────────────

// pendingFont is a recognized, not-yet-fetched store font.
type pendingFont struct {
	store  fontStore
	family string
	cssURL string
}

// parseFontURL recognizes a user-dropped (or webdev-supplied) URL as one
// of the allowlisted stores' and returns what to fetch. Hostile input:
// bounded, https-only, and the deny state is consulted here — the single
// choke point of both doors.
func parseFontURL(raw string) (pendingFont, error) {
	var p pendingFont
	if len(raw) > maxFontURLLen {
		return p, fmt.Errorf("%w: too long", ErrFontURL)
	}
	fontMu.Lock()
	disabled := webFontsDisabled
	fontMu.Unlock()
	if disabled {
		return p, ErrWebFontsDisabled
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" {
		return p, fmt.Errorf("%w: %q", ErrFontURL, raw)
	}
	for _, st := range fontStores {
		fam, ok := st.parse(u)
		if !ok {
			continue
		}
		fontMu.Lock()
		denied := deniedStores[st.name]
		fontMu.Unlock()
		if denied {
			return p, fmt.Errorf("%w: %s", ErrFontStoreDenied, st.name)
		}
		return pendingFont{store: st, family: fam, cssURL: st.cssURL(fam)}, nil
	}
	return p, fmt.Errorf("%w: %q", ErrFontURL, raw)
}

// parseFontFaceCSS extracts the @font-face variants of a store CSS
// response, keeping only file URLs on the store's own hosts (https) —
// the response is hostile input like any other: even a trusted store
// could be misconfigured or intercepted, so nothing outside its
// allowlisted file hosts survives. Per (style, weight) the first woff2
// source wins, then any first source.
func parseFontFaceCSS(css string, st fontStore) ([]WebFontSource, error) {
	if len(css) > maxFontCSSLen {
		return nil, fmt.Errorf("%w: response too large", ErrFontCSS)
	}
	var out []WebFontSource
	seen := map[string]bool{}
	rest := css
	for len(out) <= maxFontFaceRules {
		_, after, found := strings.Cut(rest, "@font-face")
		if !found {
			break
		}
		_, after, found = strings.Cut(after, "{")
		if !found {
			break
		}
		block, after, found := strings.Cut(after, "}")
		if !found {
			break
		}
		rest = after
		src := cssDecl(block, "src")
		u := pickFontURL(src, st)
		if u == "" {
			continue
		}
		style := cssDecl(block, "font-style")
		if style == "" {
			style = "normal"
		}
		weight := cssDecl(block, "font-weight")
		if weight == "" {
			weight = "400"
		}
		urange := cssDecl(block, "unicode-range")
		// One face per (style, weight, range): every SUBSET stays — see
		// the WebFontSource comment for the fallback-glyphs failure a
		// per-(style,weight) dedup caused.
		key := style + "/" + weight + "/" + urange
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, WebFontSource{Style: style, Weight: weight, Range: urange, Format: "woff2", URL: u})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no usable @font-face", ErrFontCSS)
	}
	return out, nil
}

// cssDecl extracts one declaration's value from a block.
func cssDecl(block, prop string) string {
	rest := block
	for {
		_, after, found := strings.Cut(rest, prop)
		if !found {
			return ""
		}
		after = strings.TrimLeft(after, " \t\r\n")
		if !strings.HasPrefix(after, ":") {
			rest = after
			continue
		}
		val, _, _ := strings.Cut(after[1:], ";")
		return strings.TrimSpace(val)
	}
}

// pickFontURL picks the woff2 (or first) url(...) of a src declaration
// whose host the store allows.
func pickFontURL(src string, st fontStore) string {
	first := ""
	rest := src
	for {
		_, after, found := strings.Cut(rest, "url(")
		if !found {
			return first
		}
		rawu, after, found := strings.Cut(after, ")")
		if !found {
			return first
		}
		rest = after
		rawu = strings.Trim(strings.TrimSpace(rawu), `'"`)
		u, err := url.Parse(rawu)
		if err != nil || u.Scheme != "https" || !st.fileHost(u.Host) {
			continue
		}
		if strings.Contains(after[:min(len(after), 40)], "woff2") {
			return rawu
		}
		if first == "" {
			first = rawu
		}
	}
}

// queryParam extracts one raw query parameter by hand: real css2 URLs
// carry ';' inside the family value (wght@400;700), which the standard
// url.Values parser rejects outright since Go 1.17.
func queryParam(rawQuery, name string) string {
	for _, kv := range strings.Split(rawQuery, "&") {
		if val, ok := strings.CutPrefix(kv, name+"="); ok {
			if dec, err := url.QueryUnescape(strings.ReplaceAll(val, "+", "%20")); err == nil {
				return dec
			}
			return val
		}
	}
	return ""
}

// fontSlug derives the face-picker id (class wt-ff-<slug>) from a family
// name: ASCII letters/digits kept lowercase, runs of anything else
// become one dash. The class-name rules require a letter first.
func fontSlug(family string) string {
	var sb strings.Builder
	dash := false
	for _, r := range strings.ToLower(family) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			sb.WriteRune(r)
			dash = false
		default:
			if !dash && sb.Len() > 0 {
				sb.WriteRune('-')
				dash = true
			}
		}
	}
	s := strings.TrimSuffix(sb.String(), "-")
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		s = "f-" + s
	}
	return s
}

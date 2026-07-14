package wtext

// The personal style library: the named styles a user builds up in one
// document, saved to a file and loaded into another — Word's "styles
// travel with the template", minus the template.
//
// Nothing new about trust is invented here. A library file is the same
// class registry a stored document already round-trips through its head
// <style>, in a portable envelope, so its import funnels through the very
// same gate (ValidClassName + SanitizeCSS + DefineClass) that adopting a
// document's sheet does, with the same fail-toward-text rule: one hostile
// entry poisons only itself. Two invariants are load-bearing and stated
// here once:
//
//   - Fonts travel as a store REFERENCE (a URL), never as bytes and never
//     as an @font-face rule. Import re-follows the URL through the store
//     allowlist (fontstore.go), so a library file cannot smuggle in an
//     origin the hard-coded list does not carry.
//   - Import DEFINES styles; it never APPLIES them. A library populates
//     the picker — what the text wears stays the user's decision.
//
// This file is portable: the format, its parser and the collection walk
// are pure functions over hostile input (fuzzed in stylelib_fuzz_test.go);
// the js half (download.go) only hands the bytes to the browser.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/luisfurquim/wings/epubhtml"
)

// Bounds — bounded everything: a hostile file must not grow the registry
// (or memory) without limit.
const (
	// StyleLibVersion is the format version this build writes and reads.
	StyleLibVersion = 1
	// MaxStyleLibLen bounds a library file.
	MaxStyleLibLen = 256 << 10
	// MaxLibStyles bounds how many styles one file may carry — the same
	// bound a stored document's sheet answers to.
	MaxLibStyles = maxDocClasses
	// MaxLibFonts bounds its font references: the registry they install
	// into is itself bounded, so a file cannot ask for more.
	MaxLibFonts = maxWebFonts
)

// Errors of the style library.
var (
	// ErrStyleLib reports a file that is not a readable library.
	ErrStyleLib = errors.New("wtext: not a style library file")
	// ErrStyleLibVersion reports a format version this build does not read.
	ErrStyleLibVersion = errors.New("wtext: unsupported style library version")
	// ErrStyleLibEmpty reports a file that carried nothing usable — every
	// entry was rejected, or there were none.
	ErrStyleLibEmpty = errors.New("wtext: style library carries no usable style")
	// ErrNoStyles reports an export with no named style to save.
	ErrNoStyles = errors.New("wtext: no named style to export")
	// ErrSaveUnavailable reports a save with no browser to save into (the
	// native toolchain: plugin declarations stay testable, the mechanism
	// is the js half's).
	ErrSaveUnavailable = errors.New("wtext: file saving unavailable")
)

// StyleLib is a library file: a version, the named styles, and the store
// references of the webfonts those styles name. Unknown fields are
// ignored on read, so a file written by a later build still loads what
// this one understands.
type StyleLib struct {
	Version int        `json:"wtstyles"`
	Styles  []LibStyle `json:"styles,omitempty"`
	Fonts   []LibFont  `json:"fonts,omitempty"`
}

// LibStyle is one named style: the name it registers under and its
// sanitized declaration list — the same pair DefineClass takes.
type LibStyle struct {
	Name string `json:"name"`
	CSS  string `json:"css"`
}

// LibFont is a reference to a webfont a style names: the family and the
// store URL to re-install it from. Never the font's bytes — see the
// package comment above.
type LibFont struct {
	Family string `json:"family"`
	URL    string `json:"url"`
}

// JSON renders the library as the file's bytes. Indented on purpose: the
// file is small, it belongs to the user, and it should be readable — and
// editable — in any text editor.
func (l StyleLib) JSON() ([]byte, error) {
	l.Version = StyleLibVersion
	return json.MarshalIndent(l, "", "  ")
}

// ParseStyleLib decodes a library file. The file is hostile input — it
// arrives from the user's disk and nothing vouches for it — so it is read
// in two registers: a STRUCTURAL failure (too large, not JSON, a version
// this build does not read) rejects the whole file and returns an error,
// while a single bad ENTRY is dropped, named in rejected for the caller
// to log, and the good ones still load. Everything that survives is
// canonical: names passed ValidClassName and are outside the reserved wt-
// namespace, CSS passed SanitizeCSS, font URLs passed the store
// allowlist.
func ParseStyleLib(data []byte) (lib StyleLib, rejected []string, err error) {
	if len(data) > MaxStyleLibLen {
		return StyleLib{}, nil, fmt.Errorf("%w: %d bytes over the %d limit",
			ErrStyleLib, len(data), MaxStyleLibLen)
	}
	var raw StyleLib
	if err := json.Unmarshal(data, &raw); err != nil {
		return StyleLib{}, nil, fmt.Errorf("%w: %v", ErrStyleLib, err)
	}
	if raw.Version != StyleLibVersion {
		return StyleLib{}, nil, fmt.Errorf("%w: %d (this build reads %d)",
			ErrStyleLibVersion, raw.Version, StyleLibVersion)
	}
	if len(raw.Styles) > MaxLibStyles {
		rejected = append(rejected,
			fmt.Sprintf("%d styles past the %d bound", len(raw.Styles)-MaxLibStyles, MaxLibStyles))
		raw.Styles = raw.Styles[:MaxLibStyles]
	}

	out := StyleLib{Version: StyleLibVersion}
	seen := map[string]bool{}
	for _, s := range raw.Styles {
		name := strings.TrimSpace(s.Name)
		css, err := validLibStyle(name, s.CSS)
		if err != nil {
			rejected = append(rejected, s.Name)
			continue
		}
		if seen[name] {
			continue // a name repeated inside one file: the first wins
		}
		seen[name] = true
		out.Styles = append(out.Styles, LibStyle{Name: name, CSS: css})
	}
	for _, f := range raw.Fonts {
		if len(out.Fonts) >= MaxLibFonts {
			break
		}
		family, u := strings.TrimSpace(f.Family), strings.TrimSpace(f.URL)
		if family == "" || u == "" {
			continue
		}
		// The allowlist has the final word, exactly as it does for a URL
		// dropped into the face picker: a file cannot name an origin the
		// hard-coded store list does not carry (nor one the webdev denied,
		// nor any at all once webfonts are disabled).
		if _, err := parseFontURL(u); err != nil {
			rejected = append(rejected, family)
			continue
		}
		out.Fonts = append(out.Fonts, LibFont{Family: family, URL: u})
	}
	if len(out.Styles) == 0 && len(out.Fonts) == 0 {
		return StyleLib{}, rejected, ErrStyleLibEmpty
	}
	return out, rejected, nil
}

// validLibStyle is the entry gate of an imported style: the same checks
// DefineClass runs, plus the reserved-namespace rule CreateStyle enforces
// on names the user types. A file cannot redefine wt-* — that vocabulary
// is the toolbar's, and a style that could take it would change what the
// buttons do.
func validLibStyle(name, css string) (string, error) {
	if err := epubhtml.ValidClassName(name); err != nil {
		return "", err
	}
	if strings.HasPrefix(name, "wt-") {
		return "", ErrReservedClass
	}
	return epubhtml.SanitizeCSS(css)
}

// CollectStyleLib gathers an editor's exportable library: every named
// style (the wt-* utilities are wings' own vocabulary, not the user's)
// plus a reference to each store webfont those styles name.
func CollectStyleLib(core EditorCore) StyleLib {
	lib := StyleLib{Version: StyleLibVersion}
	for _, name := range core.Classes() {
		if strings.HasPrefix(name, "wt-") {
			continue
		}
		css, ok := core.ClassCSS(name)
		if !ok || css == "" {
			continue
		}
		lib.Styles = append(lib.Styles, LibStyle{Name: name, CSS: css})
		if len(lib.Styles) >= MaxLibStyles {
			break
		}
	}
	lib.Fonts = referencedWebFonts(lib.Styles)
	return lib
}

// referencedWebFonts finds the installed store fonts the styles name.
// The match is on the font's CSS family — the quoted "Family" a webfont's
// utility class declares, which is what CreateStyle merged into the
// style's own font-family declaration. The quotes are load-bearing:
// "Roboto" does not match "Roboto Slab".
func referencedWebFonts(styles []LibStyle) []LibFont {
	var out []LibFont
	for _, wf := range InstalledWebFonts() {
		if wf.StoreURL == "" || wf.Family == "" {
			continue
		}
		for _, s := range styles {
			if strings.Contains(s.CSS, wf.Family) {
				out = append(out, LibFont{Family: wf.Label, URL: wf.StoreURL})
				break
			}
		}
	}
	return out
}

// saveFile is the download hook the portable plugin calls: nil under the
// native toolchain, assigned by download.go's init on js — the same
// portable-declaration / js-mechanism split as loadWebFont.
var saveFile func(name, mime string, data []byte)

// StyleLibrary is the personal-style-library plugin: two menu items, one
// per direction — save the document's named styles to a file, load a
// saved file back — under the standard Export and Import groups.
//
// Reusing a style across documents is what the library is FOR, so an
// imported style that the user never applies still exists: it sits in the
// picker for the next selection. (It only lands in the DOCUMENT once
// something wears it — Content() persists the styles the tree uses. The
// library file is where an unused style lives.)
type StyleLibrary struct {
	// DefaultName seeds the save prompt (the file name, without the .json
	// suffix). Empty means "styles".
	DefaultName string
}

// name is the save prompt's seed.
func (l StyleLibrary) name() string {
	if l.DefaultName != "" {
		return l.DefaultName
	}
	return "styles"
}

// MenuItems declares save and load.
func (l StyleLibrary) MenuItems() []MenuItem {
	return []MenuItem{
		MenuInput{
			Group:       "wtext-export",
			ID:          "stylelib",
			Label:       "wtext-stylelib-export",
			Placeholder: "wtext-stylelib-name",
			Help:        "wtext-stylelib-export-help",
			Value:       func(EditorCore) string { return l.name() },
			Do:          l.save,
		},
		MenuUpload{
			Group:  "wtext-import",
			ID:     "stylelib",
			Label:  "wtext-stylelib-import",
			Help:   "wtext-stylelib-import-help",
			Accept: ".json,application/json",
			MaxLen: MaxStyleLibLen,
			Do:     l.load,
		},
	}
}

// save writes the editor's named styles (and the references of the
// webfonts they use) to a downloaded file.
func (l StyleLibrary) save(core EditorCore, name string) error {
	lib := CollectStyleLib(core)
	if len(lib.Styles) == 0 {
		return ErrNoStyles
	}
	data, err := lib.JSON()
	if err != nil {
		return err
	}
	if saveFile == nil {
		return ErrSaveUnavailable
	}
	saveFile(LibFilename(name), "application/json", data)
	return nil
}

// load imports a library file. Styles whose names are free register right
// away and their fonts start loading; names ALREADY TAKEN by a style in
// this editor are the one thing the plugin will not decide alone — it
// hands the choice back as a PendingDecision, which the widget asks the
// user (once per import, for all the colliding names at once) and can
// remember for the next time.
func (l StyleLibrary) load(core EditorCore, data []byte) error {
	lib, rejected, err := ParseStyleLib(data)
	if err != nil {
		return err
	}
	for _, name := range rejected {
		G.Logf(1, "wtext: style library entry %q rejected\n", name)
	}

	var collided []LibStyle
	for _, s := range lib.Styles {
		if _, taken := core.ClassCSS(s.Name); taken {
			collided = append(collided, s)
			continue
		}
		if err := core.DefineClass(s.Name, s.CSS); err != nil {
			G.Logf(1, "wtext: style library: %q not defined: %v\n", s.Name, err)
		}
	}
	for _, f := range lib.Fonts {
		installLibFont(core, f)
	}
	if len(collided) == 0 {
		return nil
	}

	names := make([]string, 0, len(collided))
	for _, s := range collided {
		names = append(names, s.Name)
	}
	return &PendingDecision{
		Title:    "wtext-stylelib-conflict",
		Message:  "wtext-stylelib-conflict-msg",
		Detail:   names,
		Remember: StyleLibCollisionKey,
		Options: []DecisionOption{
			{Value: "overwrite", Label: "wtext-stylelib-overwrite"},
			{Value: "skip", Label: "wtext-stylelib-skip"},
		},
		Resume: func(core EditorCore, choice string) error {
			if choice != "overwrite" {
				return nil // skip: the styles already here stand
			}
			for _, s := range collided {
				if err := core.DefineClass(s.Name, s.CSS); err != nil {
					G.Logf(1, "wtext: style library: %q not redefined: %v\n", s.Name, err)
				}
			}
			return nil
		},
	}
}

// StyleLibCollisionKey is where the widget remembers the answer to the
// name-collision question ("don't ask again").
const StyleLibCollisionKey = "wtext.stylelib.collision"

// installLibFont re-installs a referenced webfont: the URL goes back
// through the store allowlist (parseFontURL, inside the loader) exactly
// as if the user had just dropped it into the face picker, and the
// document then remembers it like any other font the user installed. A
// font that fails to load costs the import nothing — the style still
// imports and renders in the fallback family, which is what the reader of
// a document missing a font sees anyway.
func installLibFont(core EditorCore, f LibFont) {
	if loadWebFont == nil {
		return // native: no browser to install a font into
	}
	loadWebFont(f.URL, func(family string, err error) {
		if err != nil {
			return // the loader logged it
		}
		rememberWebFont(core, fontSlug(family))
	})
}

// LibFilename derives the download name of a library file from the name
// the user typed: ASCII letters and digits kept, runs of anything else
// collapsed into one dash, and the .json suffix appended.
func LibFilename(name string) string {
	var sb strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
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
	slug := strings.Trim(sb.String(), "-")
	if slug == "" {
		slug = "styles"
	}
	return slug + ".json"
}

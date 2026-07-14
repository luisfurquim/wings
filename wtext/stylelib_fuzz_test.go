package wtext

import (
	"strings"
	"testing"

	"github.com/luisfurquim/wings/epubhtml"
)

// FuzzParseStyleLib: a library file is hostile input. Never panic; and
// whatever survives the parse must be safe to hand straight to
// DefineClass — a sanitized declaration list, under a valid name outside
// the reserved wt- namespace — with the store allowlist still the only
// door a font URL can come through, and every list bounded.
func FuzzParseStyleLib(f *testing.F) {
	f.Add([]byte(`{"wtstyles":1,"styles":[{"name":"titulo","css":"font-size: 2em"}]}`))
	f.Add([]byte(`{"wtstyles":1,"styles":[{"name":"wt-b","css":"color: red"}]}`))
	f.Add([]byte(`{"wtstyles":1,"styles":[{"name":"x","css":"color: red } body { display: none"}]}`))
	f.Add([]byte(`{"wtstyles":1,"styles":[{"name":"x","css":"background-color: url(https://e.example/x)"}]}`))
	f.Add([]byte(`{"wtstyles":1,"fonts":[{"family":"Lobster","url":"https://fonts.googleapis.com/css2?family=Lobster"}]}`))
	f.Add([]byte(`{"wtstyles":1,"fonts":[{"family":"E","url":"https://evil.example/css2?family=E"}]}`))
	f.Add([]byte(`{"wtstyles":2}`))
	f.Add([]byte(""))
	f.Fuzz(func(t *testing.T, data []byte) {
		lib, _, err := ParseStyleLib(data)
		if err != nil {
			return
		}
		if len(lib.Styles) > MaxLibStyles || len(lib.Fonts) > MaxLibFonts {
			t.Fatalf("unbounded output: %d styles, %d fonts", len(lib.Styles), len(lib.Fonts))
		}
		for _, s := range lib.Styles {
			if err := epubhtml.ValidClassName(s.Name); err != nil {
				t.Fatalf("invalid name survived: %q (%v)", s.Name, err)
			}
			if strings.HasPrefix(s.Name, "wt-") {
				t.Fatalf("reserved name survived: %q", s.Name)
			}
			// The core property: anything the parser kept re-passes the CSS
			// gate unchanged, so importing it can never widen what
			// DefineClass would have allowed.
			clean, err := epubhtml.SanitizeCSS(s.CSS)
			if err != nil {
				t.Fatalf("unsanitized CSS survived: %q (%v)", s.CSS, err)
			}
			if clean != s.CSS {
				t.Fatalf("non-canonical CSS survived: %q, re-sanitizes to %q", s.CSS, clean)
			}
		}
		for _, fnt := range lib.Fonts {
			if _, err := parseFontURL(fnt.URL); err != nil {
				t.Fatalf("off-store font URL survived: %q (%v)", fnt.URL, err)
			}
		}
	})
}

// FuzzStyleLibRoundTrip: whatever the parser accepts, the encoder can
// write back and the parser accepts again, identically — the file format
// is stable under a save/load cycle (the library's whole promise).
func FuzzStyleLibRoundTrip(f *testing.F) {
	f.Add([]byte(`{"wtstyles":1,"styles":[{"name":"a","css":"color: red"},{"name":"b","css":"font-size: 2em"}]}`))
	f.Add([]byte(`{"wtstyles":1,"styles":[{"name":"a","css":"COLOR:  RED ;;"}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		lib, _, err := ParseStyleLib(data)
		if err != nil {
			return
		}
		out, err := lib.JSON()
		if err != nil {
			t.Fatalf("own library will not encode: %v", err)
		}
		again, _, err := ParseStyleLib(out)
		if err != nil {
			t.Fatalf("own output will not re-parse: %v", err)
		}
		if len(again.Styles) != len(lib.Styles) || len(again.Fonts) != len(lib.Fonts) {
			t.Fatalf("round trip lost entries: %d/%d styles, %d/%d fonts",
				len(again.Styles), len(lib.Styles), len(again.Fonts), len(lib.Fonts))
		}
		for i, s := range lib.Styles {
			if again.Styles[i] != s {
				t.Fatalf("round trip changed %v into %v", s, again.Styles[i])
			}
		}
	})
}

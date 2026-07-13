package wtext

import (
	"net/url"
	"strings"
	"testing"
)

// FuzzParseFontURL: never panic; anything accepted must resolve to a
// known store with an https css URL.
func FuzzParseFontURL(f *testing.F) {
	f.Add("https://fonts.google.com/specimen/Roboto")
	f.Add("https://fonts.googleapis.com/css2?family=Lato:wght@400;700")
	f.Add("https://fonts.bunny.net/css?family=abel:400")
	f.Add("http://fonts.google.com/specimen/Roboto")
	f.Add("javascript:alert(1)")
	f.Add("")
	f.Fuzz(func(t *testing.T, raw string) {
		p, err := parseFontURL(raw)
		if err != nil {
			return
		}
		if p.store.name != "google" && p.store.name != "bunny" {
			t.Fatalf("unknown store %q accepted", p.store.name)
		}
		if !strings.HasPrefix(p.cssURL, "https://") {
			t.Fatalf("non-https cssURL %q", p.cssURL)
		}
	})
}

// FuzzParseFontFaceCSS: never panic; every extracted URL must be https
// on the store's own file hosts; output is bounded.
func FuzzParseFontFaceCSS(f *testing.F) {
	f.Add(sampleCSS)
	f.Add("@font-face{}")
	f.Add("@font-face { src: url( ) }")
	f.Add(strings.Repeat("@font-face{src:url(https://fonts.gstatic.com/a.woff2)}", 50))
	f.Fuzz(func(t *testing.T, css string) {
		for _, st := range fontStores {
			srcs, err := parseFontFaceCSS(css, st)
			if err != nil {
				continue
			}
			if len(srcs) > maxFontFaceRules+1 {
				t.Fatalf("unbounded output: %d", len(srcs))
			}
			for _, s := range srcs {
				u, err := url.Parse(s.URL)
				if err != nil || u.Scheme != "https" || !st.fileHost(u.Host) {
					t.Fatalf("hostile URL survived: %q (store %s)", s.URL, st.name)
				}
			}
		}
	})
}

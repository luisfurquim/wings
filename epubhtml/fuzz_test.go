package epubhtml

import (
	"strings"
	"testing"
	"unicode"
)

// FuzzCanonicalizeHref feeds arbitrary strings to the URL canonicalizer.
// Properties: it never panics; whatever it accepts re-canonicalizes to
// itself (idempotence — the stored form IS the canonical form); and no
// accepted URL carries a forbidden scheme or an invisible character.
func FuzzCanonicalizeHref(f *testing.F) {
	for _, s := range []string{
		"https://example.com/a?b=c#d",
		"https://bücher.example/",
		"https://аpple.com/",
		"https://google.com@evil.example/",
		"mailto:a@e.com?subject=oi%0ABcc:x@y.z&body=l1%0Al2",
		"mailto:user+tag@example.com,b@e.com?cc=c@e.com",
		"javascript:alert(1)",
		"data:text/html,x",
		"#frag",
		"http://[::1]:8080/x",
		"https://example.com/%E2%80%AEtxt.exe",
		"", "%", "://", "mailto:", "mailto:?", "https://:99999",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out, err := CanonicalizeHref(s, nil)
		if err != nil {
			return
		}
		if !strings.HasPrefix(out, "http://") && !strings.HasPrefix(out, "https://") &&
			!strings.HasPrefix(out, "mailto:") && !strings.HasPrefix(out, "#") {
			t.Fatalf("accepted URL with unexpected shape: %q -> %q", s, out)
		}
		for _, r := range out {
			if unicode.IsControl(r) || (unicode.Is(unicode.Cf, r) && r != '‌' && r != '‍') {
				t.Fatalf("control/invisible survived: %q -> %q (U+%04X)", s, out, r)
			}
		}
		again, err := CanonicalizeHref(out, nil)
		if err != nil {
			t.Fatalf("canonical form rejected on re-entry: %q -> %q: %v", s, out, err)
		}
		if again != out {
			t.Fatalf("not idempotent: %q -> %q -> %q", s, out, again)
		}
	})
}

// FuzzCleanText: never panics, idempotent per mode, and no banned rune
// survives in any mode.
func FuzzCleanText(f *testing.F) {
	for _, s := range []string{
		"plain", "a\r\nb\rc\nd", "می‌خواهم", "👩‍💻", "a‮b\x00c",
		"\xff\xfe", "‪‫‬‭‮", "",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, mode := range []TextMode{DocumentText, HeaderValue, BodyValue} {
			out := CleanText(s, mode)
			for _, r := range out {
				if r >= 0x202A && r <= 0x202E {
					t.Fatalf("bidi override survived mode %v: %q -> %q", mode, s, out)
				}
				if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
					t.Fatalf("control survived mode %v: %q -> %q (U+%04X)", mode, s, out, r)
				}
			}
			if mode == HeaderValue && strings.ContainsAny(out, "\r\n") {
				t.Fatalf("line break survived HeaderValue: %q -> %q", s, out)
			}
			if again := CleanText(out, mode); again != out {
				t.Fatalf("not idempotent mode %v: %q -> %q -> %q", mode, s, out, again)
			}
		}
	})
}

// FuzzSanitizeCSS: never panics; whatever it accepts contains no banned
// construct and re-sanitizes to itself.
func FuzzSanitizeCSS(f *testing.F) {
	for _, s := range []string{
		"color: red", "color:red;font-weight:bold",
		"background-color: url(https://evil/px.gif)",
		"color: \\75rl(x)", "@import 'x'", "color: red} body{",
		"font-family: 'A B', serif", "", ";;;", "color: var(--x)",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out, err := SanitizeCSS(s)
		if err != nil {
			return
		}
		low := strings.ToLower(out)
		for _, bad := range cssBanned {
			if strings.Contains(low, bad) {
				t.Fatalf("banned construct survived: %q -> %q (%q)", s, out, bad)
			}
		}
		again, err := SanitizeCSS(out)
		if err != nil {
			t.Fatalf("sanitized form rejected on re-entry: %q -> %q: %v", s, out, err)
		}
		if again != out {
			t.Fatalf("not idempotent: %q -> %q -> %q", s, out, again)
		}
	})
}

// FuzzFilterCSS: unlike SanitizeCSS, FilterCSS never errors — for any
// input, whatever it keeps (if anything) must itself be safe to install
// as a class's CSS, i.e. re-pass SanitizeCSS verbatim. Seeded with the
// real Google Docs clipboard style strings this function exists for.
func FuzzFilterCSS(f *testing.F) {
	for _, s := range []string{
		"color: red", "color:red;font-weight:bold",
		"background-color: url(https://evil/px.gif)",
		"color: \\75rl(x)", "@import 'x'", "color: red} body{",
		"font-family: 'A B', serif", "", ";;;", "color: var(--x)",
		"vertical-align:baseline;color:red;position:fixed",
		"line-height:1.38;margin-left: -56.69pt;text-align: justify;",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := FilterCSS(s)
		if out == "" {
			return
		}
		low := strings.ToLower(out)
		for _, bad := range cssBanned {
			if strings.Contains(low, bad) {
				t.Fatalf("banned construct survived: %q -> %q (%q)", s, out, bad)
			}
		}
		clean, err := SanitizeCSS(out)
		if err != nil {
			t.Fatalf("FilterCSS output rejected by SanitizeCSS: %q -> %q: %v", s, out, err)
		}
		if clean != out {
			t.Fatalf("FilterCSS output not already canonical: %q -> %q -> %q", s, out, clean)
		}
	})
}

// FuzzSplitCSS: for any input SanitizeCSS accepts, splitting loses no
// declaration, sends each to exactly one half, and both halves survive
// re-sanitization (they reach DefineClass-installed rules verbatim).
func FuzzSplitCSS(f *testing.F) {
	for _, s := range []string{
		"color: red", "text-align: center",
		"color:red;text-align:justify;font-family:serif",
		"margin: 0 1em; font-size: 2em", "", "color: var(--x)",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		clean, err := SanitizeCSS(s)
		if err != nil {
			return
		}
		ch, bl := SplitCSS(clean)
		count := func(decls string) int {
			if decls == "" {
				return 0
			}
			return strings.Count(decls, ";") + 1
		}
		if count(ch)+count(bl) != count(clean) {
			t.Fatalf("declarations lost or duplicated: %q -> (%q, %q)", clean, ch, bl)
		}
		for _, half := range []string{ch, bl} {
			if half == "" {
				continue
			}
			again, err := SanitizeCSS(half)
			if err != nil {
				t.Fatalf("split half rejected on re-entry: %q -> %q: %v", clean, half, err)
			}
			if again != half {
				t.Fatalf("split half not canonical: %q -> %q -> %q", clean, half, again)
			}
		}
	})
}

// FuzzValidClassName: never panics; an accepted name is ASCII and bounded.
func FuzzValidClassName(f *testing.F) {
	for _, s := range []string{"titulo", "a-b_c1", "", "1x", "a b", "tí"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if err := ValidClassName(s); err != nil {
			return
		}
		if len(s) == 0 || len(s) > MaxClassNameLen {
			t.Fatalf("accepted out-of-bounds name %q", s)
		}
		for _, r := range s {
			if r > unicode.MaxASCII {
				t.Fatalf("accepted non-ASCII name %q", s)
			}
		}
	})
}

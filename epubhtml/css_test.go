package epubhtml

import (
	"errors"
	"strings"
	"testing"
)

func TestValidClassName(t *testing.T) {
	for _, ok := range []string{"titulo", "destaque-1", "Nota_Rodape", "a"} {
		if err := ValidClassName(ok); err != nil {
			t.Errorf("ValidClassName(%q) rejected: %v", ok, err)
		}
	}
	bad := []string{
		"", "1titulo", "-lead", "tí tulo", "a.b", "a b", "a{b", "a:hover",
		strings.Repeat("x", MaxClassNameLen+1),
	}
	for _, b := range bad {
		if err := ValidClassName(b); err == nil {
			t.Errorf("ValidClassName(%q) accepted", b)
		}
	}
}

func TestSanitizeCSSAccepts(t *testing.T) {
	cases := []struct{ in, want string }{
		{"color: red", "color: red"},
		{"color:red;font-weight:bold", "color: red; font-weight: bold"},
		// Only the property is case-folded; value case is meaningful
		// (font-family names), so "Red" survives as written.
		{"COLOR: Red;", "color: Red"},
		{"margin: 0 1em 0 2em; text-align: justify", "margin: 0 1em 0 2em; text-align: justify"},
		{"font-family: 'Gentium Book', serif", "font-family: 'Gentium Book', serif"},
		{"color: var(--wings-primary)", "color: var(--wings-primary)"},
		{"border: 1px solid rgb(0, 0, 0)", "border: 1px solid rgb(0, 0, 0)"},
		{"page-break-inside: avoid", "page-break-inside: avoid"},
	}
	for _, c := range cases {
		got, err := SanitizeCSS(c.in)
		if err != nil {
			t.Errorf("SanitizeCSS(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("SanitizeCSS(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSplitCSS(t *testing.T) {
	cases := []struct{ in, char, block string }{
		// pure character formatting
		{"color: red; font-size: 1.2em", "color: red; font-size: 1.2em", ""},
		// pure paragraph formatting
		{"text-align: center; margin-top: 1em", "", "text-align: center; margin-top: 1em"},
		// mixed style splits into its two application points
		{"color: red; text-align: justify; font-family: serif; text-indent: 2em",
			"color: red; font-family: serif",
			"text-align: justify; text-indent: 2em"},
		{"", "", ""},
	}
	for _, c := range cases {
		ch, bl := SplitCSS(c.in)
		if ch != c.char || bl != c.block {
			t.Errorf("SplitCSS(%q) = (%q, %q), want (%q, %q)", c.in, ch, bl, c.char, c.block)
		}
	}
	// Every allowlisted property lands in exactly one half.
	for prop := range cssProps {
		ch, bl := SplitCSS(prop + ": x")
		if (ch == "") == (bl == "") {
			t.Errorf("SplitCSS(%q: x) = (%q, %q): property must land in exactly one half", prop, ch, bl)
		}
	}
	// Every block property is allowlisted (no orphan classification).
	for prop := range blockProps {
		if !cssProps[prop] {
			t.Errorf("blockProps[%q] is not in cssProps", prop)
		}
	}
}

func TestMergeCSS(t *testing.T) {
	cases := []struct {
		layers []string
		want   string
	}{
		{[]string{"color: red", "color: blue"}, "color: blue"},
		{[]string{"color: red; font-size: 1em", "font-size: 2em; text-align: center"},
			"color: red; font-size: 2em; text-align: center"},
		{[]string{"", "color: red", ""}, "color: red"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := MergeCSS(c.layers...); got != c.want {
			t.Errorf("MergeCSS(%q) = %q, want %q", c.layers, got, c.want)
		}
	}
}

func TestSanitizeCSSRejects(t *testing.T) {
	cases := []struct {
		name, in string
		want     error
	}{
		{"url exfiltration", "background-color: url(https://evil/px.gif)", ErrCSSValue},
		{"url case tricks", "color: URL(x)", ErrCSSValue},
		{"image-set", "background-color: image-set(url(x) 1x)", ErrCSSValue},
		{"expression", "color: expression(alert(1))", ErrCSSValue},
		{"at-rule", "@import 'x'", ErrCSSValue},
		{"backslash escape", "color: \\75rl(x)", ErrCSSValue},
		{"brace breakout", "color: red} body{background:url(x)", ErrCSSValue},
		{"comment", "color: /*x*/ red", ErrCSSValue},
		{"html smuggle", "color: <style>", ErrCSSValue},
		{"control char", "color: re\x00d", ErrCSSValue},
		{"position", "position: fixed", ErrCSSProperty},
		{"content", "content: 'fake'", ErrCSSProperty},
		{"display", "display: none", ErrCSSProperty},
		{"unknown property", "colour: red", ErrCSSProperty},
		{"no colon", "color red", ErrCSSSyntax},
		{"empty value", "color:", ErrCSSSyntax},
		{"empty input", "  ;; ", ErrCSSSyntax},
		{"oversized", "color: " + strings.Repeat("r", MaxCSSLen), ErrCSSTooLong},
	}
	for _, c := range cases {
		got, err := SanitizeCSS(c.in)
		if err == nil {
			t.Errorf("%s: SanitizeCSS(%q) accepted as %q", c.name, c.in, got)
			continue
		}
		if !errors.Is(err, c.want) {
			t.Errorf("%s: SanitizeCSS(%q) error %v, want %v", c.name, c.in, err, c.want)
		}
	}
}

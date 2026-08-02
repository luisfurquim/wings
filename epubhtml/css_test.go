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

func TestPasteClassName(t *testing.T) {
	a := PasteClassName("text-align: justify; margin-left: 10pt")
	b := PasteClassName("text-align: justify; margin-left: 10pt")
	if a != b {
		t.Errorf("not deterministic: %q != %q for the same input", a, b)
	}
	if !strings.HasPrefix(a, "wt-paste-") {
		t.Errorf("PasteClassName(...) = %q, want wt-paste- prefix", a)
	}
	if err := ValidClassName(a); err != nil {
		t.Errorf("PasteClassName produced an invalid class name %q: %v", a, err)
	}
	if c := PasteClassName("font-family: serif"); c == a {
		t.Errorf("different CSS collided onto the same class name %q", a)
	}
}

// TestPasteClassNameOrderIndependent guards the exact case the user
// raised: two elements' styles listing the same properties in a
// different order (nothing about how a source serializes style=""
// guarantees one property order over another) must collapse onto the
// SAME class, not register a spurious near-duplicate per element.
func TestPasteClassNameOrderIndependent(t *testing.T) {
	a := PasteClassName("color: red; font-size: 1em; text-align: justify")
	b := PasteClassName("text-align: justify; color: red; font-size: 1em")
	c := PasteClassName("font-size: 1em; text-align: justify; color: red")
	if a != b || b != c {
		t.Errorf("reordered-but-equivalent styles hashed differently: %q, %q, %q", a, b, c)
	}
}

// TestPasteClassNameDuplicatePropertyOrderMatters is the case sorting by
// the WHOLE declaration string (rather than just the property name)
// would get wrong: these two inputs both duplicate "color", but in
// opposite order — CSS's cascade means the LAST one wins, so they are
// NOT equivalent (one nets blue, the other red) and must not collapse
// onto the same class.
func TestPasteClassNameDuplicatePropertyOrderMatters(t *testing.T) {
	redWins := PasteClassName("color: blue; color: red")
	blueWins := PasteClassName("color: red; color: blue")
	if redWins == blueWins {
		t.Errorf("genuinely different cascades (duplicate property, opposite order) collided onto %q", redWins)
	}
}

func TestFilterCSS(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"all recognized, passes through canonical",
			"color:red;font-weight:bold", "color: red; font-weight: bold"},
		{"the real-world Google Docs span: every property in it is carried now",
			"font-size:11pt;font-family:Arial,sans-serif;color:#000000;" +
				"background-color:transparent;font-weight:400;font-style:normal;" +
				"font-variant:normal;text-decoration:none;vertical-align:baseline;" +
				"white-space:pre;white-space:pre-wrap;",
			"font-size: 11pt; font-family: Arial,sans-serif; color: #000000; " +
				"background-color: transparent; font-weight: 400; font-style: normal; " +
				"font-variant: normal; text-decoration: none; vertical-align: baseline; " +
				"white-space: pre; white-space: pre-wrap"},
		{"the real-world Google Docs p: all allowed",
			"line-height:1.38;margin-left: -56.69pt;text-indent: 84.75pt;" +
				"text-align: justify;margin-top:0pt;margin-bottom:0pt;",
			"line-height: 1.38; margin-left: -56.69pt; text-indent: 84.75pt; " +
				"text-align: justify; margin-top: 0pt; margin-bottom: 0pt"},
		{"nothing recognized survives", "position:fixed;display:none", ""},
		{"one hostile declaration poisons only itself, not its neighbors",
			"color:red;background:url(evil);font-size:1em", "color: red; font-size: 1em"},
		{"at-rule dropped, rest kept", "@import 'x';color:red", "color: red"},
		{"control char in one declaration drops just that one",
			"color:re\x00d;font-size:1em", "font-size: 1em"},
		{"empty input", "", ""},
		{"only garbage", ";;;", ""},
	}
	for _, c := range cases {
		if got := FilterCSS(c.in); got != c.want {
			t.Errorf("%s: FilterCSS(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestFilterCSSNeverErrors(t *testing.T) {
	// FilterCSS's whole point is leniency: an oversized or garbage input
	// degrades to "" (or a partial result), never a panic, and its output
	// (when non-empty) must always re-pass SanitizeCSS — anything it kept
	// really is safe to install as a class's CSS.
	huge := strings.Repeat("color:red;", MaxCSSLen)
	if got := FilterCSS(huge); got != "" {
		t.Errorf("oversized input should yield \"\", got %q", got)
	}
	if got := FilterCSS("color:red;position:fixed"); got != "" {
		if _, err := SanitizeCSS(got); err != nil {
			t.Errorf("FilterCSS output %q does not re-pass SanitizeCSS: %v", got, err)
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
		// display is carried now, but not this value: it would let a document
		// hide its own text, which then travels invisible into the export.
		{"display: none", "display: none", ErrCSSValue},
		{"positioning", "position: fixed", ErrCSSProperty},
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

// TestCSSAcceptsMultilineRules pins the fix for a whole book losing its
// typography to its own indentation: newline, tab and friends ARE control
// characters, but they are how a stylesheet is written, not how meaning
// is hidden — the check exists for invisibles (zero width, bidi), which
// stay refused.
func TestCSSAcceptsMultilineRules(t *testing.T) {
	const multiline = "font-family: \"EB Garamond\",\n\tserif;\r\n  text-align: justify;\n"
	got, err := SanitizeCSS(multiline)
	if err != nil {
		t.Fatalf("SanitizeCSS of a multi-line rule: %v", err)
	}
	for _, want := range []string{"font-family:", "text-align: justify"} {
		if !strings.Contains(got, want) {
			t.Errorf("SanitizeCSS = %q, missing %q", got, want)
		}
	}
	if strings.ContainsAny(got, "\n\t\r") {
		t.Errorf("SanitizeCSS kept raw whitespace controls: %q", got)
	}
	if got := FilterCSS(multiline); !strings.Contains(got, "text-align: justify") {
		t.Errorf("FilterCSS = %q, want the supported declarations", got)
	}
	// The invisibles this check exists for stay refused: zero-width space,
	// right-to-left override, and a bare NUL.
	for _, bad := range []string{
		"color: red​", "color:‮red", "color: red\x00",
	} {
		if _, err := SanitizeCSS(bad); err == nil {
			t.Errorf("SanitizeCSS(%q) was accepted", bad)
		}
	}
}

// TestFilterCSSKeepsSupportedAlongsideUnsupported is the property the
// document-load path depends on: a book's rule mixing an unsupported
// property with a supported one keeps the supported half instead of
// being dropped whole. (counter-reset and list-style-type used to play
// the unsupported role here; both are carried now, so the example moved
// to position, which stays out on purpose.)
func TestFilterCSSKeepsSupportedAlongsideUnsupported(t *testing.T) {
	got := FilterCSS("position: fixed; text-align: center; top: 0; font-size: 2em")
	if !strings.Contains(got, "text-align: center") || !strings.Contains(got, "font-size: 2em") {
		t.Errorf("FilterCSS = %q, want the supported declarations kept", got)
	}
	if strings.Contains(got, "position") || strings.Contains(got, "top") {
		t.Errorf("FilterCSS = %q, want the unsupported ones dropped", got)
	}
	if _, err := SanitizeCSS(got); err != nil {
		t.Errorf("what the filter left must pass the strict gate: %v", err)
	}
}

// TestCSSValueRestrictions: the first value-level rules in the profile.
// Judging by property name alone stopped being enough once a book's own
// stylesheet was carried verbatim.
func TestCSSValueRestrictions(t *testing.T) {
	for _, tc := range []struct {
		css  string
		keep bool
	}{
		// display: everything but none, whatever the spelling.
		{"display: block", true},
		{"display: grid", true},
		{"display: inline-block", true},
		{"display: none", false},
		{"display: NONE", false},
		{"display:none !important", false},
		{"display:   none   ", false},
		// overflow: the values that contain what overflows.
		{"overflow: hidden", true},
		{"overflow: scroll", true},
		{"overflow: auto", true},
		{"overflow: HIDDEN !important", true},
		{"overflow: visible", false},
		{"overflow: hidden auto", false}, // two-value form is not one of them
		// Unrestricted properties are unaffected.
		{"float: left", true},
		{"color: none", true},
	} {
		got := FilterCSS(tc.css) != ""
		if got != tc.keep {
			t.Errorf("FilterCSS(%q) kept=%v, want %v", tc.css, got, tc.keep)
		}
		_, err := SanitizeCSS(tc.css)
		if (err == nil) != tc.keep {
			t.Errorf("SanitizeCSS(%q) err=%v, want accepted=%v", tc.css, err, tc.keep)
		}
	}
}

// TestCSSFlowProperties: the properties the document-sheet pass-through
// brought in. float is the one that turns a big letter into a drop cap;
// without it the letter sits on the first line's baseline.
func TestCSSFlowProperties(t *testing.T) {
	css := "float: left; width: 2em; height: 2em; vertical-align: super; " +
		"list-style-type: none; counter-reset: x 0; gap: 1em"
	clean, err := SanitizeCSS(css)
	if err != nil {
		t.Fatalf("SanitizeCSS refused the flow group: %v", err)
	}
	for _, prop := range []string{"float", "width", "height", "vertical-align",
		"list-style-type", "counter-reset", "gap"} {
		if !strings.Contains(clean, prop) {
			t.Errorf("%q did not survive: %q", prop, clean)
		}
	}
	// position stays out: fixed positions against the viewport and would
	// let a rule inside the editor cover the application around it.
	for _, bad := range []string{"position: fixed", "position: static", "top: 0", "left: 0"} {
		if FilterCSS(bad) != "" {
			t.Errorf("FilterCSS(%q) kept a positioning declaration", bad)
		}
	}
}

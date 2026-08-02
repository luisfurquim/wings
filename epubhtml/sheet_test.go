package epubhtml

import (
	"strings"
	"testing"
)

// TestSafeSelectorShadowPiercing: the whole reason a selector is checked
// at all. Inside a shadow root these reach the host element and the
// webdev's own content, so a book carrying one could delete the widget.
func TestSafeSelectorShadowPiercing(t *testing.T) {
	for _, sel := range []string{
		":host",
		":host(.x)",
		":host-context(body)",
		"::slotted(p)",
		"::part(label)",
		":HOST",              // case
		"p, :host { x: y }",  // hidden in a list
		"span ::slotted(em)", // hidden after a combinator
	} {
		if err := SafeSelector(sel); err == nil {
			t.Errorf("SafeSelector(%q) was accepted; it pierces the shadow root", sel)
		}
	}
}

// TestSafeSelectorBreakout: anything that could close our rule and open a
// raw one.
func TestSafeSelectorBreakout(t *testing.T) {
	for _, sel := range []string{
		"p { color: red } script",
		"p }",
		"p /* c */",
		"@media print",
		"p; q",
		"a[href^=\"url(\"]",
		"", "   ",
		strings.Repeat("p", MaxSelectorLen+1),
		"p‮", // bidi override
		"p\x00",
	} {
		if err := SafeSelector(sel); err == nil {
			t.Errorf("SafeSelector(%q) was accepted", sel)
		}
	}
}

// TestSafeSelectorKeepsRealBooks: what a book actually writes must pass,
// including the backslash escape cssBanned forbids in declarations —
// `epub:type` is an ordinary EPUB attribute and ':' is not valid in a CSS
// identifier, so the escape is the only correct spelling.
func TestSafeSelectorKeepsRealBooks(t *testing.T) {
	for _, sel := range []string{
		"p.haikai",
		"span.dropcaps.b",
		".chtitle *",
		"#toclist li",
		"h1 + p",
		"blockquote > p ~ p",
		"li:first-child",
		"p:nth-child(2n+1)",
		`[epub\:type="chapter"]`,
		"h1, h2, h3, h4, h5, h6",
		"p::first-line", // matches nothing harmful: `content` is not allowlisted
	} {
		if err := SafeSelector(sel); err != nil {
			t.Errorf("SafeSelector(%q) refused a legitimate selector: %v", sel, err)
		}
	}
}

// TestParseSheetNestedBraces: splitting on '}' would end the @media rule
// at its INNER brace and hand the leftover "}" to the next rule as part
// of a selector. The scanner must consume the at-rule whole.
func TestParseSheetNestedBraces(t *testing.T) {
	rules := ParseSheet(`@media print { p { color: red } }
	                     p.haikai { text-align: right }`)
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2: %+v", len(rules), rules)
	}
	if rules[0].Kept() || !strings.Contains(rules[0].Drop, "at-rule") {
		t.Errorf("rule 0 = %+v, want dropped as an at-rule", rules[0])
	}
	if !rules[1].Kept() || rules[1].Selector != "p.haikai" {
		t.Errorf("rule 1 = %+v, want p.haikai kept", rules[1])
	}
}

// TestParseSheetBraceInString: `[title="{"]` is valid CSS. A scanner that
// does not track quotes desynchronizes here and starts reading document
// text as a selector.
func TestParseSheetBraceInString(t *testing.T) {
	rules := ParseSheet(`a[title="{"] { color: red }
	                     p { text-align: left }`)
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2: %+v", len(rules), rules)
	}
	if rules[0].Selector != `a[title="{"]` {
		t.Errorf("selector = %q, want the brace kept inside the string", rules[0].Selector)
	}
	if !rules[1].Kept() || rules[1].Selector != "p" {
		t.Errorf("rule 1 = %+v; the scanner lost sync after the quoted brace", rules[1])
	}
}

// TestParseSheetEscapedBrace: `a\}` is one escaped character, not the end
// of a block.
func TestParseSheetEscapedBrace(t *testing.T) {
	rules := ParseSheet(`a\} { color: red }
	                     p { text-align: left }`)
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2: %+v", len(rules), rules)
	}
	if !rules[1].Kept() || rules[1].Selector != "p" {
		t.Errorf("rule 1 = %+v; the scanner lost sync after the escape", rules[1])
	}
}

// TestParseSheetAtRuleStatement: `@import "x";` has no block and must not
// swallow the rule after it.
func TestParseSheetAtRuleStatement(t *testing.T) {
	rules := ParseSheet(`@import "other.css";
	                     p.haikai { text-align: right }`)
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2: %+v", len(rules), rules)
	}
	if rules[0].Kept() {
		t.Errorf("rule 0 = %+v, want the @import dropped", rules[0])
	}
	if !rules[1].Kept() || rules[1].Selector != "p.haikai" {
		t.Errorf("rule 1 = %+v, want p.haikai kept", rules[1])
	}
}

// TestParseSheetComments: a commented-out rule must not come back as a
// rule whose selector is `/* body`.
func TestParseSheetComments(t *testing.T) {
	rules := ParseSheet(`/* body { font-family: serif } */
	                     p { text-align: left }`)
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1: %+v", len(rules), rules)
	}
	if rules[0].Selector != "p" {
		t.Errorf("selector = %q, want p", rules[0].Selector)
	}
}

// TestParseSheetKeepsDeclarationsAndOrder: source order is the browser's
// last tie-breaker, so it is the sheet's meaning and must survive; and
// !important has to reach the browser intact, since resolving it is
// precisely the job being delegated.
func TestParseSheetKeepsDeclarationsAndOrder(t *testing.T) {
	rules := ParseSheet(`
		p.haikai   { font-family: "Uncial Antiqua" !important; position: fixed }
		.chtitle * { font-family: "Uncial Antiqua" !important }
		.gone      { position: absolute }
	`)
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3: %+v", len(rules), rules)
	}
	if got := rules[0].Selector; got != "p.haikai" {
		t.Errorf("rules[0].Selector = %q, want p.haikai (order lost)", got)
	}
	if !strings.Contains(rules[0].Decls, "!important") {
		t.Errorf("rules[0].Decls = %q, want !important preserved", rules[0].Decls)
	}
	if strings.Contains(rules[0].Decls, "position") {
		t.Errorf("rules[0].Decls = %q, want the unsupported property filtered out", rules[0].Decls)
	}
	if rules[2].Kept() {
		t.Errorf("rules[2] = %+v, want dropped for having nothing supported", rules[2])
	}
}

// TestParseSheetUnterminated: a rule that never closes is reported, not
// silently merged into whatever came before.
func TestParseSheetUnterminated(t *testing.T) {
	rules := ParseSheet(`p { color: red } q { text-align: left`)
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2: %+v", len(rules), rules)
	}
	if rules[1].Kept() || !strings.Contains(rules[1].Drop, "never closed") {
		t.Errorf("rules[1] = %+v, want it reported as unterminated", rules[1])
	}
}

// TestParseSheetRuleCap: bounded like everything else, and the cap is
// reported rather than passed over in silence.
func TestParseSheetRuleCap(t *testing.T) {
	rules := ParseSheet(strings.Repeat("p { text-align: left }\n", MaxSheetRules+10))
	if len(rules) != MaxSheetRules+1 {
		t.Fatalf("got %d rules, want the cap plus one report", len(rules))
	}
	last := rules[len(rules)-1]
	if last.Kept() || !strings.Contains(last.Drop, "more than") {
		t.Errorf("last = %+v, want the cap reported", last)
	}
}

// TestStripComments: a commented-out rule must not survive as text, or
// the scanner reads it as a rule of its own. The last case is the shape
// that motivated it, taken from a real book.
func TestStripComments(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"a { color: red }", "a { color: red }"},
		{"/* gone */", ""},
		{"a/* mid */b", "ab"},
		{"/* one */x/* two */y", "xy"},
		{"a { color: red } /* unterminated", "a { color: red } "},
		{"/**/x", "x"},
		{"/* nested /* still one */ x", " x"},
		{
			in:   "/*\nbody {\n\tfont-family: serif;\n}\n*/\n\nbody { font-size: 10pt }",
			want: "\n\nbody { font-size: 10pt }",
		},
	} {
		if got := stripComments(tc.in); got != tc.want {
			t.Errorf("stripComments(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestScopeSelector: the editor and the toolbar share one shadow tree, so
// an unscoped `p { … }` from a book would restyle the toolbar's own
// markup. Every part of a selector list must carry the scope — the bug a
// naive prefix makes is scoping only the first.
func TestScopeSelector(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"p", "[contenteditable] p"},
		{"p.haikai", "[contenteditable] p.haikai"},
		{"h1, h2, h3", "[contenteditable] h1, [contenteditable] h2, [contenteditable] h3"},
		{"  h1 ,  h2  ", "[contenteditable] h1, [contenteditable] h2"},
		// A pseudo-element survives because the list is split rather than
		// wrapped in :is(), which does not accept one.
		{"p::first-line", "[contenteditable] p::first-line"},
		// Commas that are not separators: inside (), [] and a string.
		{"p:not(.a, .b)", "[contenteditable] p:not(.a, .b)"},
		{`a[title="x,y"]`, `[contenteditable] a[title="x,y"]`},
		{"li:nth-child(2n+1), p", "[contenteditable] li:nth-child(2n+1), [contenteditable] p"},
		{`a\,b`, `[contenteditable] a\,b`},
	} {
		if got := ScopeSelector(tc.in, "[contenteditable]"); got != tc.want {
			t.Errorf("ScopeSelector(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestSelectorClasses: the walker keeps a class attribute only for
// classes the editor knows, so every class a preserved rule selects on
// has to be findable here — a miss loses the formatting silently, which
// is exactly how a book's haikai and drop caps lost their font while its
// title kept one.
func TestSelectorClasses(t *testing.T) {
	for _, tc := range []struct {
		sel  string
		want []string
	}{
		{"p.haikai", []string{"haikai"}},
		{"span.dropcaps.b", []string{"dropcaps", "b"}},
		{".chtitle *", []string{"chtitle"}},
		{"p", nil},
		{"#toclist li", nil},
		{".a > .b + .c ~ .d", []string{"a", "b", "c", "d"}},
		{".a, .a, .b", []string{"a", "b"}}, // deduplicated
		{"li:first-child.x", []string{"x"}},
		{`[epub\:type="chapter"].ch`, []string{"ch"}},
		// A dot inside a string is not a class.
		{`a[title=".notaclass"]`, nil},
		{`a[title=".x"].real`, []string{"real"}},
		{"p.acentuação", []string{"acentuação"}},
	} {
		got := SelectorClasses(tc.sel)
		if len(got) != len(tc.want) {
			t.Errorf("SelectorClasses(%q) = %v, want %v", tc.sel, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("SelectorClasses(%q) = %v, want %v", tc.sel, got, tc.want)
				break
			}
		}
	}
}

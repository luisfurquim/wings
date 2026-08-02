package wtext

import (
	"sort"
	"strings"

	"github.com/luisfurquim/wings/epubhtml"
)

// The document-sheet side of the editor: turning a book's own
// stylesheet into named styles, and accounting for what could not be
// turned. Portable — none of it touches the DOM, so it is unit-testable
// under the native toolchain, which is where the counting and the
// message assembly actually live.

// docSheetStats accounts for what became of a document's stylesheet, so
// the adoption can say at the end what it did — the summary is the part
// that survives when GCSS is turned down, and the reason a reader who
// only sees "font installed" is not left guessing why the text ignores it.
type docSheetStats struct {
	total    int      // rules seen (a selector with a body)
	skipped  int      // rules that did not become a named style
	reserved int      // rules that tried to redefine a wt-* the toolbar owns
	fontSels []string // skipped rules that declared a font-family
}

// maxReportedFontSelectors bounds the selectors named in the summary: the
// point is to hand over a lead, not to replay a whole stylesheet.
const maxReportedFontSelectors = 8

// drop records a refused rule and says which one and why. A rule that
// asked for a font is remembered by name: it is the one whose absence is
// visible as text in the wrong face, and the one a reader can act on.
func (st *docSheetStats) drop(sel, decls, why string) {
	st.skipped++
	// An at-rule is excluded from the lead even though @font-face names a
	// family: it DECLARES a font rather than asking for one, and it is
	// read by a different path (the importer takes the family name from it
	// to ask the stores). Naming it here would send the reader to the one
	// rule that is not the problem.
	if !strings.HasPrefix(sel, "@") &&
		strings.Contains(strings.ToLower(decls), "font-family") &&
		len(st.fontSels) < maxReportedFontSelectors {
		st.fontSels = append(st.fontSels, sel)
	}
	GCSS.Logf(3, "wtext: document rule %q skipped: %s\n", sel, why)
}

// report closes the adoption with one line at level 1 — and a second one
// when a refused rule named a font, because "the font is installed" and
// "the text does not use it" are otherwise two facts with nothing
// connecting them.
//
// The two counts are two different fates, not one number split: a NAMED
// style can be applied and removed by the picker, while a preserved
// document rule only renders — the browser matches it and the user cannot
// pick it. Reporting them apart is what tells a reader why a style they
// can see is not in the style list.
func (st *docSheetStats) report(adopted, preserved int) {
	if st.total == 0 {
		return
	}
	GCSS.Logf(1, "wtext: document sheet: %d of %d rules kept (%d as named styles, %d as document rules), %d skipped\n",
		adopted+preserved, st.total, adopted, preserved, st.skipped)
	if st.reserved > 0 {
		GCSS.Logf(2, "wtext: %d document rule(s) tried to redefine a wt-* style the toolbar owns\n", st.reserved)
	}
	if len(st.fontSels) > 0 {
		GCSS.Logf(1, "wtext: %d skipped rule(s) asked for a font — %s; that text keeps its fallback face even though the font itself may be installed\n",
			len(st.fontSels), strings.Join(st.fontSels, ", "))
	}
}

// StyleProbe is one selector to test an element against, and the text to
// show when it matches. Show may carry a single "%s", which the caller
// fills with the tag of the element that matched.
type StyleProbe struct {
	Match string
	Show  string
}

// styleProbes lists what to test an element against to learn which rules
// reach it: the document's own rules first, in source order, then the
// registered classes, sorted — the order renderClasses emits them, which
// is also the order that decided which declaration won a specificity tie.
//
// A named style contributes the TWO selectors it is actually rendered as,
// never a collapsed ".name". renderClasses splits a style into its Word
// halves — character declarations on `span.name`, paragraph declarations
// on `:is(p,h1,…).name` — and ApplyClass puts the class on the span AND
// on every block the selection touched. Collapsing them into ".name"
// reports a style across a whole paragraph when only its character half
// sits on the words the user actually styled, which is indistinguishable
// from a bug to whoever is reading the tooltip. (It bites easily:
// CreateStyle merges everything in effect at the selection, so a style
// made from bold+underline inside an imported paragraph quietly picks up
// that paragraph's alignment and margins, and with them a block half.)
// Showing "span.name" and "p.name" answers instead of confusing.
//
// Utility classes (wt-*) are NOT filtered out. They are the honest answer
// to "why does this look like this" — a run in bold through the toolbar
// carries wt-b and nothing else explains it.
//
// Portable: the pure half of Editor.StyleProbes, testable without a DOM.
func styleProbes(docRules []string, classes map[string]string, blockSel string) []StyleProbe {
	out := make([]StyleProbe, 0, len(docRules)+2*len(classes))
	seen := map[string]bool{}
	add := func(match, show string) {
		if match == "" || seen[match] {
			return
		}
		seen[match] = true
		out = append(out, StyleProbe{Match: match, Show: show})
	}
	for _, sel := range docRules {
		add(sel, sel)
	}
	names := make([]string, 0, len(classes))
	for name := range classes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		char, block := epubhtml.SplitCSS(classes[name])
		if char != "" {
			add("span."+name, "span."+name)
		}
		if block != "" {
			// Shown with the tag that actually matched: the rendered
			// selector is an :is() over every block tag, which is true but
			// unreadable in a tooltip.
			add(blockSel+"."+name, "%s."+name)
		}
	}
	return out
}

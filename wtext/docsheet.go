package wtext

import "strings"

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

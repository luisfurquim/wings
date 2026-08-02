package wtext

import "github.com/luisfurquim/goose"

// G is the logger for this module. Errors are visible by default: the
// guard recovers report through Logf(1), and a swallowed panic with no
// trace is undebuggable.
//
// It lives in a file without a build tag because the portable half logs
// too: the style-library parser drops a hostile entry and says so, the
// way the document sheet's adoption does.
var G goose.Alert

// GCSS is the logger for adopting a DOCUMENT's own stylesheet — a
// separate goose so this one section can be verbose without dragging the
// rest of the module up with it.
//
// It is loud by default (level 3) because of what the quiet version cost:
// a book whose every font-bearing rule was written `p.haikai` rather than
// `.haikai` had those rules dropped, and the only thing the console said
// was that the font had been installed. Formatting silently missing is
// the hardest kind of bug to even notice, let alone report — the person
// who can fix it is looking at the book, not at this code, and needs to
// be told which rule was refused and why. A webdev who disagrees can
// quiet it: wtext.GCSS.Set(1) keeps only the summary.
var GCSS goose.Alert

func init() {
	G.Set(1)
	GCSS.Set(3)
}

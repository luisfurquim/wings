package wtext

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

// TestDocSheetStatsReport: the accounting is what the console says, so
// it is checked here rather than by reading a browser log — a skipped
// rule that asked for a font is the lead the reader needs, and it must
// be picked up regardless of how the declaration is spelled.
func TestDocSheetStatsReport(t *testing.T) {
	var st docSheetStats
	st.total = 4
	st.drop("p.haikai", "font-family: \"Uncial Antiqua\" !important", "x")
	st.drop("SPAN.dropcaps", "FONT-FAMILY: Uncial", "x") // case-insensitive
	st.drop(".nameref", "width: 0.75em", "x")            // no font: not a lead

	if st.skipped != 3 {
		t.Errorf("skipped = %d, want 3", st.skipped)
	}
	want := []string{"p.haikai", "SPAN.dropcaps"}
	if len(st.fontSels) != len(want) {
		t.Fatalf("fontSels = %v, want %v", st.fontSels, want)
	}
	for i, w := range want {
		if st.fontSels[i] != w {
			t.Errorf("fontSels[%d] = %q, want %q", i, st.fontSels[i], w)
		}
	}
}

// TestDocSheetStatsFontCap: the summary hands over a lead, it does not
// replay a stylesheet — a sheet where everything names a font must not
// turn the console into the sheet.
func TestDocSheetStatsFontCap(t *testing.T) {
	var st docSheetStats
	for i := 0; i < maxReportedFontSelectors*3; i++ {
		st.drop(".s", "font-family: X", "x")
	}
	if len(st.fontSels) != maxReportedFontSelectors {
		t.Errorf("fontSels grew to %d, want the %d cap", len(st.fontSels), maxReportedFontSelectors)
	}
	if st.skipped != maxReportedFontSelectors*3 {
		t.Errorf("skipped = %d, want every drop counted", st.skipped)
	}
}

// TestDocSheetStatsSkipsAtRules: @font-face declares a font instead of
// asking for one, and the importer reads it elsewhere — pointing the
// reader at it would point away from the rule that actually lost the face.
func TestDocSheetStatsSkipsAtRules(t *testing.T) {
	var st docSheetStats
	st.drop("@font-face", "font-family: \"Uncial Antiqua\"; src: url(x.ttf)", "x")
	st.drop("p.haikai", "font-family: \"Uncial Antiqua\"", "x")

	if len(st.fontSels) != 1 || st.fontSels[0] != "p.haikai" {
		t.Errorf("fontSels = %v, want only [p.haikai]", st.fontSels)
	}
	if st.skipped != 2 {
		t.Errorf("skipped = %d, want both rules counted", st.skipped)
	}
}

// captureLog collects what goose emits through the standard logger while
// fn runs — GCSS's output IS the feature here, so it is asserted rather
// than eyeballed in a browser console.
func captureLog(fn func()) string {
	var buf bytes.Buffer
	out, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(out)
		log.SetFlags(flags)
	}()
	fn()
	return buf.String()
}

// TestDocSheetReportSilentWhenEmpty: a document with no style rules at
// all has nothing to report, and must not announce that it adopted zero
// of zero — noise on every plain paste is how a useful log stops
// being read.
func TestDocSheetReportSilentWhenEmpty(t *testing.T) {
	var st docSheetStats
	if got := captureLog(func() { st.report(0, 0) }); got != "" {
		t.Errorf("report on an empty sheet said %q, want silence", got)
	}
}

// TestDocSheetReportContents: the summary must carry the counts, and the
// font line must name the selectors, because those two lines are the
// whole point — they are what connects "the font is installed" to "the
// text does not use it".
func TestDocSheetReportContents(t *testing.T) {
	var st docSheetStats
	st.total = 22
	st.reserved = 2
	st.drop("p.haikai", "font-family: X", "why")

	got := captureLog(func() { st.report(4, 6) })
	for _, want := range []string{
		"10 of 22 rules kept",
		"4 as named styles, 6 as document rules",
		"1 skipped",
		"p.haikai",
		"2 document rule(s) tried to redefine",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q; got:\n%s", want, got)
		}
	}
}

// TestDocSheetReportQuietGoose: turning GCSS down to 1 must still leave
// the summary and the font lead — the webdev who silences the per-rule
// detail is not asking to be told nothing.
func TestDocSheetReportQuietGoose(t *testing.T) {
	saved := GCSS
	GCSS.Set(1)
	defer func() { GCSS = saved }()

	var st docSheetStats
	st.total = 3
	st.reserved = 1
	perRule := captureLog(func() { st.drop("p.haikai", "font-family: X", "why") })
	if perRule != "" {
		t.Errorf("per-rule detail leaked at level 1: %q", perRule)
	}
	got := captureLog(func() { st.report(2, 0) })
	if !strings.Contains(got, "2 of 3 rules kept") || !strings.Contains(got, "p.haikai") {
		t.Errorf("summary or font lead lost at level 1; got:\n%s", got)
	}
	if strings.Contains(got, "redefine") {
		t.Errorf("level-2 detail leaked at level 1; got:\n%s", got)
	}
}

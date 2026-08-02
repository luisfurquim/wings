package wtext

import (
	"strings"
	"testing"
)

// blockIs is the block half's selector shape, as renderClasses builds it.
const blockIs = ":is(p,h1,h2,h3,h4,h5,h6,blockquote,pre)"

func shown(probes []StyleProbe) string {
	var s []string
	for _, p := range probes {
		s = append(s, p.Show)
	}
	return strings.Join(s, "|")
}

func matched(probes []StyleProbe) string {
	var s []string
	for _, p := range probes {
		s = append(s, p.Match)
	}
	return strings.Join(s, "|")
}

// TestStyleProbesOrder: the order is the order the rules reach the
// browser (document rules first, then named styles), because that is what
// decided which declaration won a specificity tie.
func TestStyleProbesOrder(t *testing.T) {
	got := styleProbes(
		[]string{"p.haikai", ".chtitle *"},
		map[string]string{"titulo": "color: red", "abc": "color: blue"},
		blockIs,
	)
	if want := "p.haikai|.chtitle *|span.abc|span.titulo"; shown(got) != want {
		t.Errorf("shown = %q, want %q", shown(got), want)
	}
}

// TestStyleProbesSplitsNamedStyle is the bug the browser found and the
// tests had not: a style is rendered as TWO rules, and reporting it as
// one ".name" claims the whole style is in effect across a paragraph when
// only its character half sits on the words the user styled.
//
// A style with both halves must probe both, and the paragraph half must
// be shown against the tag that matched, not as the :is() it is rendered
// as.
func TestStyleProbesSplitsNamedStyle(t *testing.T) {
	got := styleProbes(nil,
		map[string]string{"teste": "font-weight: bold; text-align: center"},
		blockIs)

	if want := "span.teste|%s.teste"; shown(got) != want {
		t.Errorf("shown = %q, want %q", shown(got), want)
	}
	if want := "span.teste|" + blockIs + ".teste"; matched(got) != want {
		t.Errorf("matched = %q, want %q", matched(got), want)
	}
}

// TestStyleProbesCharacterOnly: a style made of character formatting has
// no paragraph half and must NOT probe one — that half is not rendered,
// so claiming it would report a rule the browser never applied. This is
// the shape of a plain bold+underline style.
func TestStyleProbesCharacterOnly(t *testing.T) {
	got := styleProbes(nil,
		map[string]string{"forte": "font-weight: bold; text-decoration: underline"},
		blockIs)
	if len(got) != 1 || got[0].Match != "span.forte" {
		t.Errorf("probes = %+v, want only the character half", got)
	}
}

// TestStyleProbesBlockOnly: the mirror case — alignment and margins only.
func TestStyleProbesBlockOnly(t *testing.T) {
	got := styleProbes(nil,
		map[string]string{"centro": "text-align: center; margin-top: 1em"},
		blockIs)
	if len(got) != 1 || got[0].Match != blockIs+".centro" || got[0].Show != "%s.centro" {
		t.Errorf("probes = %+v, want only the paragraph half", got)
	}
}

// TestStyleProbesKeepsUtilityClasses: wt-* is the honest answer to "why
// is this bold" — a run formatted through the toolbar carries wt-b and
// nothing else explains it. The style PICKER hides them; the inspector
// must not.
func TestStyleProbesKeepsUtilityClasses(t *testing.T) {
	got := styleProbes(nil, map[string]string{"wt-b": "font-weight: bold"}, blockIs)
	if len(got) != 1 || got[0].Show != "span.wt-b" {
		t.Errorf("probes = %+v, want the utility class kept", got)
	}
}

// TestStyleProbesDeduplicates: a book whose sheet names ".chtitle" both
// as a document rule and as a registered class must not probe it twice.
func TestStyleProbesDeduplicates(t *testing.T) {
	got := styleProbes([]string{"span.chtitle"},
		map[string]string{"chtitle": "color: red"}, blockIs)
	if len(got) != 1 {
		t.Errorf("probes = %+v, want the repeat collapsed", got)
	}
}

// TestStyleProbesEmpty: a pristine editor probes nothing, and the caller
// gets an empty list rather than a nil it has to special-case.
func TestStyleProbesEmpty(t *testing.T) {
	if got := styleProbes(nil, nil, blockIs); len(got) != 0 {
		t.Errorf("probes = %+v, want empty", got)
	}
}

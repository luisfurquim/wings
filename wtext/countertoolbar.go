package wtext

import (
	"strings"
	"unicode"
)

// CounterToolbar is a stock ToolbarPlugin with a single StatusItem: a
// live character/letter/word count over the whole document. It is purely
// observational — the passive half of the toolbar contract, the way
// BasicToolbar is the reference for the active half.
type CounterToolbar struct{}

// Items declares the counter read-out.
func (CounterToolbar) Items() []ToolbarItem {
	return []ToolbarItem{
		StatusItem{
			ID:     "counter",
			Label:  "wtext-counter-label",
			Format: "wtext-counter",
			Help:   "wtext-counter-help",
			Args:   CountDoc,
		},
	}
}

// CountDoc is the counter's Args: characters, letters and words of the
// whole document, in that order.
func CountDoc(core EditorCore) []any {
	chars, letters, words := Count(core.DocText())
	return []any{chars, letters, words}
}

// Count counts a plain text the way editors report it: chars are runes
// with spaces included but line breaks excluded (DocText marks block
// boundaries with "\n", and a paragraph mark is not a character the user
// typed inside the text); letters are the runes unicode.IsLetter accepts;
// words are whitespace-separated fields carrying at least one letter or
// digit — standalone punctuation (a comma between spaces, a lone dash) is
// not a word, the same rule the office suites apply.
func Count(text string) (chars, letters, words int) {
	for _, r := range text {
		if r == '\n' {
			continue
		}
		chars++
		if unicode.IsLetter(r) {
			letters++
		}
	}
	for _, f := range strings.Fields(text) {
		if strings.ContainsFunc(f, isWordRune) {
			words++
		}
	}
	return chars, letters, words
}

// isWordRune reports whether r makes a field count as a word.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

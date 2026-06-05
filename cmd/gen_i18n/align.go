package main

import "sort"

// matchKind classifies how a new source string relates to the previous catalog.
type matchKind uint8

const (
	matchNone  matchKind = iota // brand-new source: no translation to carry
	matchExact                  // identical source: carry translation + Revised
	matchFuzzy                  // edited source: reuse translation, flag review
)

// fuzzyThreshold is the minimum Levenshtein ratio for the fuzzy tier to treat
// an unmatched new source as an edit of an unmatched old source (reusing its
// translation, flagged revised=false). 0.6 favors reuse: the worst case of a
// spurious pair is one re-review by the translator, versus today's hard reset
// of the translation on any source edit.
const fuzzyThreshold = 0.6

// srcAnchor is one exact-match pair: position in the new source order and the
// index it descends from in the old catalog.
type srcAnchor struct {
	newPos int
	oldPos int
}

// alignSources maps each new source string to the old catalog index it
// descends from, distinguishing exact carry-overs from fuzzy (edited) ones.
//
// Both slices hold unique strings (gen_i18n dedups by content via resolveHash),
// so the exact tier is a position-independent global content match: a string
// that merely moved — even across files, shifting every index after it — keeps
// its old index regardless of distance. The exact matches then act as monotonic
// anchors (longest increasing subsequence by old index); the leftover orphans
// are paired only within the gap between two consecutive anchors, so document
// order disambiguates simultaneous edits. An orphan that was both edited and
// moved across an anchor stays unmatched (new + deleted) — the worst case is
// today's behavior.
//
// Returns parallel slices of length len(newSrc): the old index (or -1) and the
// match kind for each new source.
func alignSources(oldSrc, newSrc []string) (oldIdx []int, kind []matchKind) {
	oldIdx = make([]int, len(newSrc))
	kind = make([]matchKind, len(newSrc))
	for i := range oldIdx {
		oldIdx[i] = -1
	}

	// ── Tier 1: exact, global, position-independent ───────────────────────────
	oldByContent := make(map[string]int, len(oldSrc))
	for i, s := range oldSrc {
		if _, seen := oldByContent[s]; !seen {
			oldByContent[s] = i
		}
	}
	oldClaimed := make([]bool, len(oldSrc))
	var exact []srcAnchor
	for i, s := range newSrc {
		if oi, ok := oldByContent[s]; ok && !oldClaimed[oi] {
			oldIdx[i] = oi
			kind[i] = matchExact
			oldClaimed[oi] = true
			exact = append(exact, srcAnchor{newPos: i, oldPos: oi})
		}
	}

	// Monotonic anchor set. Exact matches keep their translation either way;
	// only the increasing subsequence (by old index) bounds the fuzzy gaps, so
	// a reordered-but-identical string never corrupts a gap's boundaries.
	anchors := lisByOld(exact)

	// ── Tier 2: fuzzy within each gap between consecutive anchors ──────────────
	prevNew, prevOld := -1, -1
	gap := func(hiNew, hiOld int) {
		var newOrphans, oldOrphans []int
		for n := prevNew + 1; n < hiNew; n++ {
			if kind[n] == matchNone {
				newOrphans = append(newOrphans, n)
			}
		}
		for o := prevOld + 1; o < hiOld; o++ {
			if !oldClaimed[o] {
				oldOrphans = append(oldOrphans, o)
			}
		}
		pairGap(newOrphans, oldOrphans, oldSrc, newSrc, oldIdx, kind, oldClaimed)
	}
	for _, a := range anchors {
		gap(a.newPos, a.oldPos)
		prevNew, prevOld = a.newPos, a.oldPos
	}
	gap(len(newSrc), len(oldSrc))
	return oldIdx, kind
}

// pairGap matches new orphans to old orphans within one gap by best Levenshtein
// ratio, one-to-one, above fuzzyThreshold. Candidates are sorted by descending
// ratio with deterministic tie-breaks, then assigned greedily.
func pairGap(newOrphans, oldOrphans []int, oldSrc, newSrc []string, oldIdx []int, kind []matchKind, oldClaimed []bool) {
	if len(newOrphans) == 0 || len(oldOrphans) == 0 {
		return
	}
	type cand struct {
		n, o  int
		ratio float64
	}
	var cands []cand
	for _, n := range newOrphans {
		for _, o := range oldOrphans {
			if r := levRatio(newSrc[n], oldSrc[o]); r >= fuzzyThreshold {
				cands = append(cands, cand{n: n, o: o, ratio: r})
			}
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].ratio != cands[j].ratio {
			return cands[i].ratio > cands[j].ratio
		}
		if cands[i].n != cands[j].n {
			return cands[i].n < cands[j].n
		}
		return cands[i].o < cands[j].o
	})
	usedNew := make(map[int]bool, len(newOrphans))
	for _, c := range cands {
		if usedNew[c.n] || oldClaimed[c.o] {
			continue
		}
		oldIdx[c.n] = c.o
		kind[c.n] = matchFuzzy
		oldClaimed[c.o] = true
		usedNew[c.n] = true
	}
}

// lisByOld returns the longest subsequence of a (already ordered by newPos)
// whose oldPos values strictly increase — the monotonic anchor set. Old/new
// strings are unique, so two distinct anchors never share an oldPos.
func lisByOld(a []srcAnchor) []srcAnchor {
	n := len(a)
	if n == 0 {
		return nil
	}
	dp := make([]int, n)   // length of the increasing run ending at i
	prev := make([]int, n) // predecessor index, -1 if none
	best := 0
	for i := 0; i < n; i++ {
		dp[i] = 1
		prev[i] = -1
		for j := 0; j < i; j++ {
			if a[j].oldPos < a[i].oldPos && dp[j]+1 > dp[i] {
				dp[i] = dp[j] + 1
				prev[i] = j
			}
		}
		if dp[i] > dp[best] {
			best = i
		}
	}
	var out []srcAnchor
	for k := best; k != -1; k = prev[k] {
		out = append(out, a[k])
	}
	for l, r := 0, len(out)-1; l < r; l, r = l+1, r-1 {
		out[l], out[r] = out[r], out[l]
	}
	return out
}

// levRatio is the normalized Levenshtein similarity of a and b in [0,1]:
// 1 for equal strings, 0 for fully dissimilar. Operates on runes so it is
// correct for multi-byte UTF-8 (UI strings often carry accents/emoji).
func levRatio(a, b string) float64 {
	if a == b {
		return 1
	}
	ra, rb := []rune(a), []rune(b)
	maxLen := len(ra)
	if len(rb) > maxLen {
		maxLen = len(rb)
	}
	if maxLen == 0 {
		return 1
	}
	return float64(maxLen-levenshtein(ra, rb)) / float64(maxLen)
}

// levenshtein returns the edit distance between two rune slices using the
// classic two-row dynamic-programming table.
func levenshtein(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

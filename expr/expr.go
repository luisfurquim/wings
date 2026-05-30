package expr

import (
	"fmt"
	"strconv"
	"unicode"
	"unicode/utf8"
)

// ── Token type constants ────────────────────────────────────────────────────

// TokenType represents the type of a token in the template parser.
type TokenType int8

// Token types produced by the expression lexer.
const (
	TokTxt       TokenType = 0  // literal text
	TokRef       TokenType = 1  // reference {{ }}
	TokStr       TokenType = 2  // string literal in quotes
	TokDot       TokenType = 3  // operator .
	TokOpen      TokenType = 4  // operator [
	TokClose     TokenType = 5  // operator ]
	TokNum       TokenType = 6  // integer number
	TokIdent     TokenType = 7  // identifier
	TokWSep      TokenType = 8  // internal state: waiting for separator
	TokExpr      TokenType = 9  // sub-expression (dynamic index access)
	TokAttr      TokenType = 10 // attribute node (DOMRefNode type)
	TokPctVar    TokenType = 11 // %ident  — count variable + emission
	TokAtVar     TokenType = 12 // @ident  — gender variable, no emission
	TokTildeWord TokenType = 13 // ~ident  — flex marker (build-time only)
	TokFlexIdx   TokenType = 14 // #N      — inflection rule index
	TokColon     TokenType = 15 // :       — format-name separator in %var:name
)

// ── Template parse structures ───────────────────────────────────────────────

// RefNode is a node in the parsed reference tree.
// For TokExpr, Sub contains the sub-expression (e.g. the index in arr[expr]).
type RefNode struct {
	Type   TokenType
	StrVal string
	IntVal int
	Sub    []RefNode // populated only when Type == TokExpr
}

// TextSegment is a template text segment: literal or reference.
type TextSegment struct {
	IsRef bool
	Lit   string    // if !IsRef: literal text
	Ref   []RefNode // if IsRef: parsed reference tree
}

// ── Template text parser ────────────────────────────────────────────────────

// ParseText splits a template string into literal segments and references.
// References are delimited by {{ and }}. Each reference is immediately
// tokenized and parsed into a RefNode tree.
func ParseText(s string) ([]TextSegment, error) {
	var segs []TextSegment
	i, start := 0, 0
	inRef := false

	for i < len(s) {
		if !inRef {
			if i+1 < len(s) && s[i] == '{' && s[i+1] == '{' {
				if i > start {
					segs = append(segs, TextSegment{Lit: s[start:i]})
				}
				i += 2
				start = i
				inRef = true
				continue
			}
		} else {
			if i+1 < len(s) && s[i] == '}' && s[i+1] == '}' {
				expr := s[start:i]
				toks := Tokenize(expr)
				// Flexion and formatting blocks skip ParseReference and keep
				// their raw token list — callers detect them via
				// IsFmtBlock/IsFlexBlock and use ParseFmtBlock/ParseFlexBlock.
				if IsFmtBlock(toks) || IsFlexBlock(toks) {
					segs = append(segs, TextSegment{IsRef: true, Ref: toks})
				} else {
					ref, err := ParseReference(&toks)
					if err != nil {
						return nil, fmt.Errorf("ParseText: %w", err)
					}
					segs = append(segs, TextSegment{IsRef: true, Ref: ref})
				}
				i += 2
				start = i
				inRef = false
				continue
			}
		}
		i++
	}

	if start < len(s) {
		segs = append(segs, TextSegment{Lit: s[start:]})
	}

	return segs, nil
}

// HasRef returns true if any segment is a reference.
func HasRef(segs []TextSegment) bool {
	for i := range segs {
		if segs[i].IsRef {
			return true
		}
	}
	return false
}

// IsPureTextSegs returns true if all segments are literals (no references).
func IsPureTextSegs(segs []TextSegment) bool {
	return !HasRef(segs)
}

// IsPureReference returns true if the tree contains no literals (str/num/txt),
// i.e., it is a pure path of identifiers/expr (useful for two-way binding).
func IsPureReference(tree []RefNode) bool {
	for i := range tree {
		switch tree[i].Type {
		case TokStr, TokNum, TokTxt:
			return false
		}
	}
	return true
}

// IsPureSegs returns true if there is exactly one IsRef segment with a pure reference.
func IsPureSegs(segs []TextSegment) bool {
	if len(segs) != 1 || !segs[0].IsRef {
		return false
	}
	return IsPureReference(segs[0].Ref)
}

// ── Reference expression tokenizer ──────────────────────────────────────────

// preToken is used internally by splitStrings.
type preToken struct {
	isStr bool
	val   string
}

// splitStrings separates quoted string literals from the rest of the code.
func splitStrings(s string) []preToken {
	var result []preToken
	i, start := 0, 0
	inStr := false
	var delim byte

	for i < len(s) {
		if !inStr {
			if s[i] == '\'' || s[i] == '"' {
				if i > start {
					result = append(result, preToken{val: s[start:i]})
				}
				delim = s[i]
				inStr = true
				start = i + 1
			}
		} else if s[i] == delim {
			result = append(result, preToken{isStr: true, val: s[start:i]})
			inStr = false
			start = i + 1
		}
		i++
	}

	if start < len(s) {
		result = append(result, preToken{val: s[start:]})
	}

	return result
}

// isWhitespace returns true for space, tab, newline, carriage return.
func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// isIdentStart returns true for characters that can start an identifier.
func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isIdentChar returns true for characters that can appear inside an identifier.
func isIdentChar(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

// consumeNumber reads a number (with optional sign and decimal part) from s
// starting at position i. Returns the RefNode and the new position.
func consumeNumber(s string, i int) (RefNode, int) {
	n := len(s)
	j := i
	if s[j] == '+' || s[j] == '-' {
		j++
	}
	for j < n && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	// decimal part (we only store the integer part via parseInt)
	if j < n && s[j] == '.' {
		j++
		for j < n && s[j] >= '0' && s[j] <= '9' {
			j++
		}
	}
	// parseInt replicates the JS behavior (truncates fraction)
	intStr := s[i:j]
	if idx := indexByte(intStr, '.'); idx >= 0 {
		intStr = intStr[:idx]
	}
	val, _ := strconv.Atoi(intStr)
	return RefNode{Type: TokNum, IntVal: val}, j
}

// consumeIdent reads an identifier from s starting at position i.
// Returns the RefNode and the new position.
func consumeIdent(s string, i int) (RefNode, int) {
	j := i + 1
	for j < len(s) && isIdentChar(s[j]) {
		j++
	}
	return RefNode{Type: TokIdent, StrVal: s[i:j]}, j
}

// splitSymbols tokenizes a code fragment without string literals.
// Recognizes: `.`, `[`, `]`, `#N` (inflection rule index), numbers, identifiers,
// and the i18n flexion sigils `%ident`, `@ident`, `~ident`.
// Doubled sigils (`%%`, `@@`, `~~`) are escapes: they emit a TokStr whose
// StrVal is the single-sigil form, so a webdev documenting the syntax can
// show it literally without wi18n/gen_i18n treating it as a marker.
func splitSymbols(s string) []RefNode {
	var toks []RefNode
	i := 0
	n := len(s)

	for i < n {
		c := s[i]

		if isWhitespace(c) {
			i++
			continue
		}

		switch c {
		case '.':
			toks = append(toks, RefNode{Type: TokDot})
			i++
		case '[':
			toks = append(toks, RefNode{Type: TokOpen})
			i++
		case ']':
			toks = append(toks, RefNode{Type: TokClose})
			i++
		case ':':
			toks = append(toks, RefNode{Type: TokColon})
			i++
		case '#':
			// #N → TokFlexIdx with IntVal=N (inflection rule index)
			if i+1 < n && s[i+1] >= '0' && s[i+1] <= '9' {
				j := i + 1
				for j < n && s[j] >= '0' && s[j] <= '9' {
					j++
				}
				val, _ := strconv.Atoi(s[i+1 : j])
				toks = append(toks, RefNode{Type: TokFlexIdx, IntVal: val})
				i = j
			} else {
				// Bare `#` (legacy) — keep old behavior.
				toks = append(toks, RefNode{Type: TokIdent, StrVal: "#"})
				i++
			}
		case '%', '@', '~':
			tok, next, ok := consumeSigil(s, i)
			if ok {
				toks = append(toks, tok)
			}
			i = next
		default:
			isSign := (c == '+' || c == '-') && i+1 < n && s[i+1] >= '0' && s[i+1] <= '9'
			switch {
			case isSign || (c >= '0' && c <= '9'):
				var tok RefNode
				tok, i = consumeNumber(s, i)
				toks = append(toks, tok)
			case isIdentStart(c):
				var tok RefNode
				tok, i = consumeIdent(s, i)
				toks = append(toks, tok)
			default:
				// Unknown character: skip
				i++
			}
		}
	}

	return toks
}

// consumeSigil reads one of the i18n flexion sigils (`%`, `@`, `~`) followed
// by an identifier. A doubled sigil (`%%ident`, `@@ident`, `~~ident`) is an
// escape: the returned token is a TokStr with the single-sigil form as value,
// so the output is rendered literally.
//
// `%var` and `@var` consume ASCII-only identifiers (variable names from app
// code). `~word` consumes a natural-language token: any sequence of Unicode
// letters/digits, plus the literal '|' character for the dual-lemma form
// (`~pai|mãe`). This lets templates carry accented Portuguese/Spanish/etc.
// words verbatim — without it, "~está" would split as `~est` + `á…`.
//
// Returns (tok, nextPos, ok). ok=false means no token was produced (bare
// sigil with no identifier — the char is skipped silently).
func consumeSigil(s string, i int) (RefNode, int, bool) {
	c := s[i]
	n := len(s)
	// Escape form: `%%`, `@@`, `~~`
	if i+1 < n && s[i+1] == c {
		j := i + 2
		if c == '~' {
			j = consumeTildeWord(s, j)
		} else {
			for j < n && isIdentChar(s[j]) {
				j++
			}
		}
		return RefNode{Type: TokStr, StrVal: s[i+1 : j]}, j, true
	}
	// Sigil form
	j := i + 1
	if c == '~' {
		k := consumeTildeWord(s, j)
		if k == j {
			return RefNode{}, i + 1, false
		}
		return RefNode{Type: TokTildeWord, StrVal: s[j:k]}, k, true
	}
	if j < n && isIdentStart(s[j]) {
		k := j + 1
		for k < n && isIdentChar(s[k]) {
			k++
		}
		var t TokenType
		switch c {
		case '%':
			t = TokPctVar
		case '@':
			t = TokAtVar
		}
		return RefNode{Type: t, StrVal: s[j:k]}, k, true
	}
	// Bare sigil with no identifier — skip.
	return RefNode{}, i + 1, false
}

// consumeTildeWord scans s starting at j, advancing past every byte that
// belongs to a Unicode letter, a Unicode digit, or the literal '|' (for the
// dual-lemma form `~m|f`). Returns the position of the first byte that does
// not belong — equal to j if no character was consumed.
func consumeTildeWord(s string, j int) int {
	k := j
	for k < len(s) {
		if s[k] == '|' {
			k++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[k:])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		k += size
	}
	return k
}

// indexByte finds the first index of b in s, or -1.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// Tokenize converts a reference expression into a list of RefNodes.
// First extracts string literals, then tokenizes the remaining fragments.
func Tokenize(s string) []RefNode {
	var toks []RefNode

	preToks := splitStrings(s)
	for _, pt := range preToks {
		if pt.isStr {
			toks = append(toks, RefNode{Type: TokStr, StrVal: pt.val})
		} else {
			toks = append(toks, splitSymbols(pt.val)...)
		}
	}

	return toks
}

// ── Reference parser ────────────────────────────────────────────────────────

// ParseReference builds the reference tree from a list of tokens.
// Consumes tokens from the beginning of the slice pointed to by toks.
func ParseReference(toks *[]RefNode) ([]RefNode, error) {
	if len(*toks) == 0 {
		return nil, fmt.Errorf("ParseReference: empty reference")
	}

	token := popRef(toks)

	// Standalone literal (num or str): returns immediately
	if token.Type == TokNum || token.Type == TokStr {
		if len(*toks) == 0 || (*toks)[0].Type == TokClose {
			if len(*toks) > 0 {
				popRef(toks) // consumes the ']'
			}
			return []RefNode{token}, nil
		}
	}

	if token.Type != TokIdent {
		return nil, fmt.Errorf("ParseReference: expected identifier, found type=%d val=%q", token.Type, token.StrVal)
	}

	tree := []RefNode{token}

	// State: stWSep = waiting for separator (. or [); stRef = waiting for ident/str
	const stWSep, stRef = 0, 1
	stat := stWSep

	for len(*toks) > 0 && (*toks)[0].Type != TokClose {
		token = popRef(toks)

		if stat == stWSep {
			if token.Type == TokOpen {
				sub, err := ParseReference(toks)
				if err != nil {
					return nil, err
				}
				tree = append(tree, RefNode{Type: TokExpr, Sub: sub})
				continue
			} else if token.Type != TokDot {
				return nil, fmt.Errorf("ParseReference: expected '.' or '[', found type=%d", token.Type)
			}
			stat = stRef
		} else { // stRef
			if token.Type == TokIdent || token.Type == TokStr {
				tree = append(tree, token)
				stat = stWSep
				continue
			}
			return nil, fmt.Errorf("ParseReference: expected identifier, found type=%d", token.Type)
		}
	}

	// Consumes closing ']' if present
	if len(*toks) > 0 && (*toks)[0].Type == TokClose {
		popRef(toks)
	}

	return tree, nil
}

// popRef removes and returns the first token from the slice.
func popRef(toks *[]RefNode) RefNode {
	t := (*toks)[0]
	*toks = (*toks)[1:]
	return t
}

// ── Flexion block ───────────────────────────────────────────────────────────

// FlexBlock holds the resolved pieces of an i18n flexion reference such as
// `{{@genero %qt #42}}` or `{{@genero %qt ~o ~aluno ...}}`.
//
// At runtime the rewritten form `{{@var %var #N}}` is what reaches the
// browser. Build-time (`gen_i18n`) also sees the `~word` form, so TildeWords
// is populated only when parsing the pre-rewrite template.
//
// Tokens preserves the full input token sequence in original order so that
// build-time dict-consult can re-interleave passthrough words (non-sigil
// tokens that appear between ~words, e.g., "do terceiro ano que" in
// "{{@s %q ~o ~aluno do terceiro ano que ~está ~aprovado}}") with the flexed
// forms pulled from the dictionary.
type FlexBlock struct {
	GenderVar  string    // @var root ident, "" when absent (degenerate-gender block)
	GenderPath []RefNode // full @-path (root ident + optional .ident/[expr] tail); nil when @var absent or bare
	CountVar   string    // %var root ident, "" when absent (pure-gender block)
	CountPath  []RefNode // full %-path; nil when %var absent or bare
	Idx        int       // #N rule index, -1 when absent (pre-gen_i18n form)
	TildeWords []string  // ~word lemmas in original order (build-time only)
	Tokens     []RefNode // input token sequence in original order; path-tail tokens consumed
	// by @var/%var are NOT re-emitted here (they are metadata attached to the
	// sigil RefNode that precedes them).
}

// IsFlexBlock reports whether the token slice begins with a flexion sigil
// (`%`, `@`, `~`, or `#N`), meaning ParseFlexBlock should be used instead of
// ParseReference. A lone `%var` (with optional path) is NOT a FlexBlock — it
// is a FmtBlock (see IsFmtBlock), routed to FmtPrinter for locale-aware
// formatting.
func IsFlexBlock(toks []RefNode) bool {
	if len(toks) == 0 {
		return false
	}
	switch toks[0].Type {
	case TokAtVar, TokTildeWord, TokFlexIdx:
		return true
	case TokPctVar:
		// %var alone (plus optional path tail) is a FmtBlock, not a FlexBlock.
		// When %var co-occurs with @/~/# or passthrough literals, it is the
		// count axis of a flexion — then it is a FlexBlock.
		return !IsFmtBlock(toks)
	}
	return false
}

// ParseFlexBlock consumes a flat sequence of flexion tokens and returns the
// composed FlexBlock. Enforces the unicity rules locked in the design:
//   - at most one %var per block (hard error on second)
//   - at most one @var per block (hard error on second)
//   - at most one #N per block (hard error on second)
//
// ~word tokens are collected in order; repetitions are allowed since
// adjectives/verbs can legitimately repeat a lemma.
// Identifier/number/string tokens that appear between sigils are accepted as
// passthrough words (see the FlexBlock doc for the "do terceiro ano"
// example). Structural tokens (`.`, `[`, `]`, sub-expressions) are an error —
// a flex block cannot mix sigil syntax with path-reference syntax.
func ParseFlexBlock(toks *[]RefNode) (FlexBlock, error) {
	fb := FlexBlock{Idx: -1}
	seenPct, seenAt, seenIdx := false, false, false

	for len(*toks) > 0 {
		t := popRef(toks)
		switch t.Type {
		case TokAtVar:
			if seenAt {
				return fb, fmt.Errorf("ParseFlexBlock: only one @var allowed per block, found %q after %q", t.StrVal, fb.GenderVar)
			}
			fb.GenderVar = t.StrVal
			tail, err := consumePathTail(toks)
			if err != nil {
				return fb, fmt.Errorf("ParseFlexBlock: @var path: %w", err)
			}
			if len(tail) > 0 {
				fb.GenderPath = append([]RefNode{{Type: TokIdent, StrVal: t.StrVal}}, tail...)
			}
			fb.Tokens = append(fb.Tokens, t)
			seenAt = true
		case TokPctVar:
			if seenPct {
				return fb, fmt.Errorf("ParseFlexBlock: only one %%var allowed per block, found %q after %q", t.StrVal, fb.CountVar)
			}
			fb.CountVar = t.StrVal
			tail, err := consumePathTail(toks)
			if err != nil {
				return fb, fmt.Errorf("ParseFlexBlock: %%var path: %w", err)
			}
			if len(tail) > 0 {
				fb.CountPath = append([]RefNode{{Type: TokIdent, StrVal: t.StrVal}}, tail...)
			}
			fb.Tokens = append(fb.Tokens, t)
			seenPct = true
		case TokTildeWord:
			fb.TildeWords = append(fb.TildeWords, t.StrVal)
			fb.Tokens = append(fb.Tokens, t)
		case TokFlexIdx:
			if seenIdx {
				return fb, fmt.Errorf("ParseFlexBlock: only one #N allowed per block, found #%d after #%d", t.IntVal, fb.Idx)
			}
			fb.Idx = t.IntVal
			fb.Tokens = append(fb.Tokens, t)
			seenIdx = true
		case TokIdent, TokStr, TokNum:
			// Passthrough literal word: kept in Tokens for build-time use.
			fb.Tokens = append(fb.Tokens, t)
		default:
			return fb, fmt.Errorf("ParseFlexBlock: unexpected token type=%d in flex block", t.Type)
		}
	}

	return fb, nil
}

// consumePathTail greedily consumes `.ident` / `[expr]` pairs from the head
// of toks, returning the accumulated RefNode sequence (not including the
// root ident). Stops at the first token that is not TokDot or TokOpen.
// Returns an empty slice when the next token is not a path continuation.
func consumePathTail(toks *[]RefNode) ([]RefNode, error) {
	var out []RefNode
	for len(*toks) > 0 {
		switch (*toks)[0].Type {
		case TokDot:
			popRef(toks)
			if len(*toks) == 0 {
				return out, fmt.Errorf("trailing '.' with no identifier")
			}
			next := popRef(toks)
			if next.Type != TokIdent && next.Type != TokStr {
				return out, fmt.Errorf("expected identifier after '.', got type=%d", next.Type)
			}
			out = append(out, next)
		case TokOpen:
			popRef(toks)
			sub, err := ParseReference(toks)
			if err != nil {
				return out, fmt.Errorf("in '[...]': %w", err)
			}
			out = append(out, RefNode{Type: TokExpr, Sub: sub})
		default:
			return out, nil
		}
	}
	return out, nil
}

// ── FmtBlock: lone %var for locale-aware formatting ─────────────────────────

// FmtBlock holds the resolved pieces of a locale-aware formatting reference
// such as `{{%preco}}`, `{{%cart[i].total}}`, or `{{%dist:km}}`. The value
// is resolved at sync time from the data context and rendered by
// wings.FmtPrinter, which chooses the output format from the Go type of the
// value, the current locale, and the optional format name.
//
// A FmtBlock is the lone-%var form, with an optional `:formatName` suffix.
// When %var co-occurs with @var/~word/#N (or passthrough literals) inside the
// same {{...}} block, it is a FlexBlock count axis instead — routed to
// SynPrinter.
type FmtBlock struct {
	Var        string    // %var root ident
	Path       []RefNode // full path (root ident + tail); nil when bare
	FormatName string    // name after ':' (e.g. "km" in %dist:km); "" when absent
}

// IsFmtBlock reports whether toks is a lone %var with an optional path tail
// and optional :formatName suffix, and nothing else. Malformed path tails
// (e.g. trailing `.`) return false so the caller surfaces the error via
// ParseFmtBlock.
func IsFmtBlock(toks []RefNode) bool {
	if len(toks) == 0 || toks[0].Type != TokPctVar {
		return false
	}
	rest := append([]RefNode(nil), toks[1:]...)
	if _, err := consumePathTail(&rest); err != nil {
		return false
	}
	if len(rest) == 0 {
		return true
	}
	return len(rest) == 2 && rest[0].Type == TokColon && rest[1].Type == TokIdent
}

// ParseFmtBlock consumes a lone-%var FmtBlock token sequence, including an
// optional :formatName suffix. Caller should gate on IsFmtBlock first;
// ParseFmtBlock returns an error on any mismatch so FlexBlocks mis-routed
// here surface loudly rather than silently.
func ParseFmtBlock(toks *[]RefNode) (FmtBlock, error) {
	if len(*toks) == 0 || (*toks)[0].Type != TokPctVar {
		return FmtBlock{}, fmt.Errorf("ParseFmtBlock: expected %%var at block start")
	}
	head := popRef(toks)
	fb := FmtBlock{Var: head.StrVal}
	tail, err := consumePathTail(toks)
	if err != nil {
		return fb, fmt.Errorf("ParseFmtBlock: %%var path: %w", err)
	}
	if len(tail) > 0 {
		fb.Path = append([]RefNode{{Type: TokIdent, StrVal: head.StrVal}}, tail...)
	}
	if len(*toks) >= 2 && (*toks)[0].Type == TokColon && (*toks)[1].Type == TokIdent {
		popRef(toks) // consume ':'
		fb.FormatName = popRef(toks).StrVal
	}
	if len(*toks) != 0 {
		return fb, fmt.Errorf("ParseFmtBlock: unexpected trailing tokens (this is a FlexBlock, not a FmtBlock)")
	}
	return fb, nil
}

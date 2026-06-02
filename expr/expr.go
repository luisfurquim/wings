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
	TokStarVar   TokenType = 16 // *ident  — CustomFlex object (engine/selector)
	TokDollarVar TokenType = 17 // $ident  — dynamic bind value, emitted verbatim
	TokFlexBind  TokenType = 18 // ~$ident — dynamic value to be inflected at runtime
	TokSpace     TokenType = 19 // collapsed whitespace run (\s+ → one space) in flex content
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
		case '%', '@', '~', '*', '$':
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

// consumeSigil reads one of the i18n flexion sigils (`%`, `@`, `~`, `*`, `$`)
// followed by an identifier. A doubled sigil (`%%ident`, `@@ident`, `~~ident`,
// `**ident`, `$$ident`) is an escape: the returned token is a TokStr with the
// single-sigil form as value, so the output is rendered literally.
//
// `%var`, `@var`, `*var` and `$var` consume ASCII-only identifiers (variable
// names from app code; only the root ident here — the `.field`/`[expr]` path
// tail is assembled later by ParseFlexBlock, same as for `@`/`%`). `~word`
// consumes a natural-language token: any sequence of Unicode letters/digits,
// plus the literal '|' character for the dual-lemma form (`~pai|mãe`). This
// lets templates carry accented words verbatim — without it, "~está" would
// split as `~est` + `á…`. The special form `~$ident` is a flexbind: a dynamic
// value (resolved from the data context) to be inflected at runtime.
//
// Returns (tok, nextPos, ok). ok=false means no token was produced (bare
// sigil with no identifier — the char is skipped silently).
func consumeSigil(s string, i int) (RefNode, int, bool) {
	c := s[i]
	n := len(s)
	// Escape form: `%%`, `@@`, `~~`, `**`, `$$`
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
	// `~` family: ~word (literal lemma) or ~$ident (flexbind dynamic value).
	if c == '~' {
		j := i + 1
		if j < n && s[j] == '$' {
			if k, ok := scanIdent(s, j+1); ok {
				return RefNode{Type: TokFlexBind, StrVal: s[j+1 : k]}, k, true
			}
			// `~$` with no identifier — skip the `~`, let `$` be handled next.
			return RefNode{}, i + 1, false
		}
		k := consumeTildeWord(s, j)
		if k == j {
			return RefNode{}, i + 1, false
		}
		return RefNode{Type: TokTildeWord, StrVal: s[j:k]}, k, true
	}
	// `%`, `@`, `*`, `$`: sigil + ASCII ident (root only; path tail later).
	if k, ok := scanIdent(s, i+1); ok {
		var t TokenType
		switch c {
		case '%':
			t = TokPctVar
		case '@':
			t = TokAtVar
		case '*':
			t = TokStarVar
		case '$':
			t = TokDollarVar
		}
		return RefNode{Type: t, StrVal: s[i+1 : k]}, k, true
	}
	// Bare sigil with no identifier — skip.
	return RefNode{}, i + 1, false
}

// scanIdent reads an identifier starting at j (must begin with an ident-start
// char). Returns the end position and true, or (j, false) if s[j] is not a
// valid identifier start.
func scanIdent(s string, j int) (int, bool) {
	if j >= len(s) || !isIdentStart(s[j]) {
		return j, false
	}
	k := j + 1
	for k < len(s) && isIdentChar(s[k]) {
		k++
	}
	return k, true
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

// ── Flex-content tokenizer ──────────────────────────────────────────────────

// TokenizeFlexContent tokenizes a flex block authored as inline content: a run
// of literal text interleaved with flex sigils. Unlike Tokenize (which discards
// whitespace, treating the block as a reference expression), this preserves the
// authored text faithfully, following HTML text-node semantics:
//
//   - a whitespace run (\s+: spaces, tabs, newlines) collapses to a single
//     TokSpace token (the browser collapses it anyway in a text node);
//   - a run of non-whitespace, non-sigil characters becomes one TokTxt token,
//     preserving punctuation and non-ASCII (e.g. Japanese) verbatim;
//   - a sigil ($var, ~$var, ~word, @var, %var, *var, #N) becomes its token;
//     path-bearing sigils ($/*/~$/@/%) carry their `.field`/`[expr]` tail in
//     Sub. `$$`/`**`/`~~`/`%%`/`@@` are escapes → literal text.
//
// A sigil character that does not form a valid sigil (e.g. a lone `$` before a
// digit, as in a price "US$ 5") is treated as literal text. This is the
// tokenizer for the per-locale content phrase of a programmable flex rule; the
// legacy `{{@g %c ~word}}` path keeps using Tokenize + ParseFlexBlock.
func TokenizeFlexContent(s string) []RefNode {
	var toks []RefNode
	var lit []byte
	flush := func() {
		if len(lit) > 0 {
			toks = append(toks, RefNode{Type: TokTxt, StrVal: string(lit)})
			lit = lit[:0]
		}
	}
	i, n := 0, len(s)
	for i < n {
		c := s[i]
		switch {
		case isWhitespace(c):
			flush()
			j := i + 1
			for j < n && isWhitespace(s[j]) {
				j++
			}
			toks = append(toks, RefNode{Type: TokSpace, StrVal: " "})
			i = j
		case c == '#' && i+1 < n && s[i+1] >= '0' && s[i+1] <= '9':
			flush()
			j := i + 1
			for j < n && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			val, _ := strconv.Atoi(s[i+1 : j])
			toks = append(toks, RefNode{Type: TokFlexIdx, IntVal: val})
			i = j
		case c == '@' || c == '%' || c == '~' || c == '*' || c == '$':
			tok, next, ok := consumeSigil(s, i)
			if !ok {
				lit = append(lit, c) // not a valid sigil → literal char
				i++
				continue
			}
			if tok.Type == TokStr {
				lit = append(lit, tok.StrVal...) // escape → literal text
				i = next
				continue
			}
			switch tok.Type {
			case TokDollarVar, TokStarVar, TokFlexBind, TokAtVar, TokPctVar:
				tok.Sub, next = consumeContentPathTail(s, next, tok.StrVal)
			}
			flush()
			toks = append(toks, tok)
			i = next
		default:
			lit = append(lit, c)
			i++
		}
	}
	flush()
	return toks
}

// consumeContentPathTail consumes the `.field` / `[expr]` tail of a
// path-bearing sigil inside flex content, starting at position i (just past
// the root ident). A `.` is a path accessor only when immediately followed by
// an identifier-start char — otherwise (end, space, punctuation) it is left for
// the literal scanner, so "comprou $produto." keeps its trailing period. A `[`
// is consumed only when it has a matching `]` whose contents parse as a
// reference. Returns the full path (root ident + tail) and the new position, or
// (nil, i) when there is no tail.
func consumeContentPathTail(s string, i int, root string) ([]RefNode, int) {
	var tail []RefNode
	n := len(s)
	for i < n {
		if s[i] == '.' && i+1 < n && isIdentStart(s[i+1]) {
			end, _ := scanIdent(s, i+1)
			tail = append(tail, RefNode{Type: TokIdent, StrVal: s[i+1 : end]})
			i = end
			continue
		}
		if s[i] == '[' {
			depth, j := 1, i+1
			for j < n && depth > 0 {
				switch s[j] {
				case '[':
					depth++
				case ']':
					depth--
				}
				j++
			}
			if depth != 0 {
				break // unbalanced → leave '[' to the literal scanner
			}
			sub := Tokenize(s[i+1 : j-1])
			ref, err := ParseReference(&sub)
			if err != nil {
				break
			}
			tail = append(tail, RefNode{Type: TokExpr, Sub: ref})
			i = j
			continue
		}
		break
	}
	if len(tail) == 0 {
		return nil, i
	}
	return append([]RefNode{{Type: TokIdent, StrVal: root}}, tail...), i
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
	// sigil RefNode that precedes them). For *var/$var/~$var the path tail is
	// stored in the token's Sub field, so Tokens carries enough for runtime
	// assembly in order.
	StarVars   []FlexVarRef // *var participants (CustomFlex engine/selector), in order
	DollarVars []FlexVarRef // $var plain dynamic binds (emitted verbatim), in order
	FlexBinds  []FlexVarRef // ~$var dynamic values to be inflected at runtime, in order
}

// FlexVarRef is a resolved variable reference inside a flex block for one of
// the path-bearing sigils (*var, $var, ~$var). Var is the root identifier;
// Path is the full reference (root ident + `.field`/`[expr]` tail), nil when
// the reference is a bare name. Resolution at runtime goes through wings.Solve
// when Path is non-nil, or the cheap single-level lookup for bare Var.
type FlexVarRef struct {
	Var  string
	Path []RefNode
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
	case TokAtVar, TokTildeWord, TokFlexIdx, TokStarVar, TokDollarVar, TokFlexBind:
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
		case TokStarVar:
			path, err := buildFlexPath(t.StrVal, toks)
			if err != nil {
				return fb, fmt.Errorf("ParseFlexBlock: *var path: %w", err)
			}
			t.Sub = path
			fb.StarVars = append(fb.StarVars, FlexVarRef{Var: t.StrVal, Path: path})
			fb.Tokens = append(fb.Tokens, t)
		case TokDollarVar:
			path, err := buildFlexPath(t.StrVal, toks)
			if err != nil {
				return fb, fmt.Errorf("ParseFlexBlock: $var path: %w", err)
			}
			t.Sub = path
			fb.DollarVars = append(fb.DollarVars, FlexVarRef{Var: t.StrVal, Path: path})
			fb.Tokens = append(fb.Tokens, t)
		case TokFlexBind:
			path, err := buildFlexPath(t.StrVal, toks)
			if err != nil {
				return fb, fmt.Errorf("ParseFlexBlock: ~$var path: %w", err)
			}
			t.Sub = path
			fb.FlexBinds = append(fb.FlexBinds, FlexVarRef{Var: t.StrVal, Path: path})
			fb.Tokens = append(fb.Tokens, t)
		default:
			// Any other token is literal content of the phrase — a word
			// (TokIdent/TokStr/TokNum) or punctuation (`:`, `.`, …). It is kept
			// in Tokens for build-time use; ParseFlexBlock only extracts the
			// control sigils, and the per-locale phrase is assembled separately
			// by TokenizeFlexContent (which already treats these as literal). So
			// a literal colon — or a mistyped sigil — degrades to visible text
			// instead of aborting the block's rewrite.
			fb.Tokens = append(fb.Tokens, t)
		}
	}

	return fb, nil
}

// buildFlexPath consumes the `.field`/`[expr]` path tail for a path-bearing
// flex sigil (*var/$var/~$var) and returns the full reference (root ident +
// tail), or nil when the reference is a bare name. Mirrors the GenderPath /
// CountPath assembly used for @var/%var.
func buildFlexPath(root string, toks *[]RefNode) ([]RefNode, error) {
	tail, err := consumePathTail(toks)
	if err != nil {
		return nil, err
	}
	if len(tail) == 0 {
		return nil, nil
	}
	return append([]RefNode{{Type: TokIdent, StrVal: root}}, tail...), nil
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

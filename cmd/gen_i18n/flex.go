package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"

	"github.com/luisfurquim/wings/expr"
	"github.com/luisfurquim/wings/wi18n"
)

// ── Flex block state (parallel to txt/occurrences) ──────────────────────────

// flexBlocks holds each distinct flex block found during the HTML walk.
// The index into this slice is the stable rule index "#N" that gen_i18n
// emits into the rewritten .i18n.html and that SynPrinter uses at runtime.
//
// Stability across runs is *by canonical key, not by slice index*: indices
// may shift when blocks are added or removed, but the catalog remap step
// (analogous to the text catalog remap) preserves translations as long as
// the canonical key survives.
var flexBlocks []expr.FlexBlock

// flexKeys is the canonical inner-form of each flex block, parallel to
// flexBlocks by index.
var flexKeys []string

// flexKeyIdx is the reverse index: canonical key → position in flexBlocks.
var flexKeyIdx = map[string]int32{}

// flexOccurrences records where each flex block appears in the HTML source,
// keyed by its position in flexBlocks.
var flexOccurrences = map[int32][]Occurrence{}

// flexContent holds the deflang content phrase of every *programmable* flex
// block (one whose source uses *var/$var/~$var), keyed by its position in
// flexBlocks. A block's presence in this map is what marks it programmable:
// it gets a Content-based catalog entry instead of the gender×CLDR cells grid.
// Legacy gender/count blocks never appear here. (project_customflex_design)
var flexContent = map[int32]string{}

// isProgrammableFlex reports whether a flex block carries any of the
// CustomFlex sigils (*var engine/selector, $var verbatim bind, ~$var dynamic
// flexbind). Such blocks are assembled at runtime from a per-locale Content
// phrase rather than looked up in the gender×CLDR cells grid.
func isProgrammableFlex(fb expr.FlexBlock) bool {
	return len(fb.StarVars) > 0 || len(fb.DollarVars) > 0 || len(fb.FlexBinds) > 0
}

// canonicalFlexKey normalises the inner content of a {{...}} flex block so
// that semantically-identical blocks written with different whitespace
// collapse to the same key.
func canonicalFlexKey(inner string) string {
	return strings.Join(strings.Fields(inner), " ")
}

// resolveFlex returns the stable rule index for a flex block. A block with a
// previously-seen canonical key reuses its existing index; a new block is
// appended.
func resolveFlex(canonical string, fb expr.FlexBlock) int32 {
	if idx, ok := flexKeyIdx[canonical]; ok {
		return idx
	}
	idx := int32(len(flexBlocks))
	flexBlocks = append(flexBlocks, fb)
	flexKeys = append(flexKeys, canonical)
	flexKeyIdx[canonical] = idx
	return idx
}

// rewriteFlexBlocks scans text for every {{...}} block and, for those whose
// token stream begins with a flexion sigil, replaces the block with its
// canonical runtime form {{@var %var #N}}. Non-flex blocks pass through
// unchanged.
//
// recordOccurrence is called with the assigned index whenever a flex block
// is encountered, so the caller can track per-block source positions.
// Returns (rewritten, changed) — changed=true when at least one flex block
// was rewritten.
func rewriteFlexBlocks(text string, recordOccurrence func(idx int32)) (string, bool) {
	var b strings.Builder
	b.Grow(len(text))
	changed := false
	i := 0
	n := len(text)
	for i < n {
		if i+1 < n && text[i] == '{' && text[i+1] == '{' {
			// Locate the matching `}}`.
			j := i + 2
			for j+1 < n && !(text[j] == '}' && text[j+1] == '}') {
				j++
			}
			if j+1 >= n {
				// Unclosed — emit the rest verbatim.
				b.WriteString(text[i:])
				i = n
				continue
			}
			inner := text[i+2 : j]
			toks := expr.Tokenize(inner)
			if !expr.IsFlexBlock(toks) {
				b.WriteString(text[i : j+2])
				i = j + 2
				continue
			}
			toksCopy := append([]expr.RefNode(nil), toks...)
			fb, err := expr.ParseFlexBlock(&toksCopy)
			if err != nil {
				G.Logf(2, "warn: flex block parse error in %q: %v (left as-is)", inner, err)
				b.WriteString(text[i : j+2])
				i = j + 2
				continue
			}
			canonical := canonicalFlexKey(inner)
			idx := resolveFlex(canonical, fb)
			if recordOccurrence != nil {
				recordOccurrence(idx)
			}
			// Programmable block: capture the per-locale content phrase
			// (literal text + $var/~$var/~word/%count, control sigils stripped)
			// from the raw inner — which still has its whitespace, unlike the
			// Tokenize-d token stream above. First occurrence of a canonical key
			// wins, matching the index assignment.
			if isProgrammableFlex(fb) {
				if _, ok := flexContent[idx]; !ok {
					flexContent[idx] = buildFlexContent(inner)
				}
			}
			// Emit runtime form. Path-valued vars keep their full path
			// (`@user.gender`, `%cart[i].qty`) so the browser-side parser
			// reconstructs the same GenderPath/CountPath after rewrite.
			b.WriteString("{{")
			sep := ""
			if fb.GenderVar != "" {
				b.WriteString("@")
				if len(fb.GenderPath) > 0 {
					b.WriteString(pathToStr(fb.GenderPath))
				} else {
					b.WriteString(fb.GenderVar)
				}
				sep = " "
			}
			if fb.CountVar != "" {
				b.WriteString(sep)
				b.WriteString("%")
				if len(fb.CountPath) > 0 {
					b.WriteString(pathToStr(fb.CountPath))
				} else {
					b.WriteString(fb.CountVar)
				}
				sep = " "
			}
			// *engine candidates: kept in the runtime control block so the
			// browser-side election (highest Priority wins) sees them. $var and
			// ~$var are NOT emitted here — they live in the Content phrase and
			// are resolved from there at assembly time.
			for _, sv := range fb.StarVars {
				b.WriteString(sep)
				b.WriteString("*")
				if len(sv.Path) > 0 {
					b.WriteString(pathToStr(sv.Path))
				} else {
					b.WriteString(sv.Var)
				}
				sep = " "
			}
			b.WriteString(sep)
			b.WriteString("#")
			b.WriteString(strconv.Itoa(int(idx)))
			b.WriteString("}}")
			changed = true
			i = j + 2
			continue
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String(), changed
}

// buildFlexContent serialises the per-locale Content phrase of a programmable
// flex block from its raw inner text. It keeps the content tokens (literal
// text, whitespace, $var, ~$var, ~word, %count) and drops the control sigils
// (@var, *var, #N), which live in the rewritten runtime block instead.
//
// inner is tokenised with TokenizeFlexContent (NOT Tokenize) so authored
// whitespace survives — the JP-vs-PT spacing decision lives in this phrase.
// Whitespace orphaned by a removed control sigil is collapsed: leading,
// trailing, and doubled TokSpace runs fold to nothing / a single space. The
// browser collapses runs in a text node anyway, so this is lossless for
// display and keeps the translator-facing phrase clean.
func buildFlexContent(inner string) string {
	var kept []expr.RefNode
	for _, t := range expr.TokenizeFlexContent(inner) {
		switch t.Type {
		case expr.TokAtVar, expr.TokStarVar, expr.TokFlexIdx:
			// control sigil — stripped from the content phrase
		case expr.TokSpace:
			// drop leading/doubled space (a removed control sigil may have
			// left an orphan)
			if len(kept) == 0 || kept[len(kept)-1].Type == expr.TokSpace {
				continue
			}
			kept = append(kept, t)
		default:
			kept = append(kept, t)
		}
	}
	// Trim a trailing space left after the last content token.
	for len(kept) > 0 && kept[len(kept)-1].Type == expr.TokSpace {
		kept = kept[:len(kept)-1]
	}

	var b strings.Builder
	for _, t := range kept {
		switch t.Type {
		case expr.TokTxt:
			b.WriteString(t.StrVal)
		case expr.TokSpace:
			b.WriteByte(' ')
		case expr.TokDollarVar:
			b.WriteByte('$')
			b.WriteString(flexVarStr(t))
		case expr.TokFlexBind:
			b.WriteString("~$")
			b.WriteString(flexVarStr(t))
		case expr.TokTildeWord:
			b.WriteByte('~')
			b.WriteString(t.StrVal)
		case expr.TokPctVar:
			b.WriteByte('%')
			b.WriteString(flexVarStr(t))
		}
	}
	return b.String()
}

// flexVarStr serialises a path-bearing content token ($var/~$var/%count) back
// to its source form. TokenizeFlexContent stores the full path (root ident +
// `.field`/`[expr]` tail) in Sub when one is present; otherwise the bare root
// is in StrVal. The serialisation round-trips through TokenizeFlexContent at
// runtime to the same reference.
func flexVarStr(t expr.RefNode) string {
	if len(t.Sub) > 0 {
		return pathToStr(t.Sub)
	}
	return t.StrVal
}

// ── Translator-facing label ─────────────────────────────────────────────────

// flexLabel builds a human-readable stem from a FlexBlock's original token
// stream:
//   - @var        → (removed; gender is metadata-only)
//   - %var        → {n} placeholder; path-valued %var becomes `{n:path}`
//     so two blocks differing only in the counted variable's path do not
//     collide in the remap map (which is keyed by Label).
//   - ~word       → the literal word (tilde stripped)
//   - ~m|f        → "m|f" (dual-lemma kept for translator context)
//   - passthrough → emitted as-is
//
// Tokens not previously seen (escape-form TokStr from `%%`/`@@`/`~~`) render
// as literal text.
//
// NOTE: @var path differences are still invisible in the label (gender
// stays metadata-only). Two blocks differing only in the @-path would
// remap to the same stored translation — acceptable for v1, revisit if
// it bites in practice.
func flexLabel(fb expr.FlexBlock) string {
	var out []string
	for _, t := range fb.Tokens {
		switch t.Type {
		case expr.TokAtVar:
			// gender discriminator — metadata only
		case expr.TokPctVar:
			if len(fb.CountPath) > 0 {
				out = append(out, "{n:"+pathToStr(fb.CountPath)+"}")
			} else {
				out = append(out, "{n}")
			}
		case expr.TokTildeWord, expr.TokIdent, expr.TokStr:
			out = append(out, t.StrVal)
		case expr.TokNum:
			out = append(out, strconv.Itoa(t.IntVal))
		}
	}
	return strings.Join(out, " ")
}

// flexLabelFor returns the remap key / translator-facing label for the flex
// block at index i. For a programmable block the label IS its deflang Content
// phrase: locale-invariant, stored in every language's entry, so prior
// translations remap by content across runs (the source phrase changing drops
// the stale translation, exactly like the text catalog). Legacy gender/count
// blocks fall back to the sigil-stripped flexLabel stem.
func flexLabelFor(i int32, fb expr.FlexBlock) string {
	if c, ok := flexContent[i]; ok {
		return c
	}
	return flexLabel(fb)
}

// pathToStr serialises a FlexBlock Gender/Count path back to its source-
// level form (e.g. `user.gender`, `cart[i].qty`). Mirrors the subset of the
// tokenizer that splitSymbols accepts for path tails after a @/% sigil.
func pathToStr(path []expr.RefNode) string {
	var b strings.Builder
	for i, r := range path {
		switch r.Type {
		case expr.TokIdent, expr.TokStr:
			if i > 0 {
				b.WriteByte('.')
			}
			b.WriteString(r.StrVal)
		case expr.TokExpr:
			b.WriteByte('[')
			b.WriteString(pathToStr(r.Sub))
			b.WriteByte(']')
		case expr.TokNum:
			if i > 0 {
				b.WriteByte('.')
			}
			b.WriteString(strconv.Itoa(r.IntVal))
		}
	}
	return b.String()
}

// ── Gender inventory + CLDR categories ──────────────────────────────────────

// cldrCategories is the fixed set of CLDR plural category names. Every
// inflections catalog cell grid enumerates all six per gender; unused ones
// stay empty strings.
var cldrCategories = []string{"zero", "one", "two", "few", "many", "other"}

// genderInventory returns the gender blocks a language needs. This is a
// deliberately small, hand-curated table: languages not listed fall back to
// the degenerate single-block form (empty prefix), which matches English,
// Japanese, Chinese, Korean, and most of the CJK/analytic family.
func genderInventory(lang string) []string {
	tag, err := language.Parse(lang)
	if err != nil {
		return []string{""}
	}
	base, _ := tag.Base()
	switch base.String() {
	case "pt", "es", "fr", "it", "ca", "ro", "gl", "oc":
		return []string{"m", "f"}
	case "de":
		return []string{"m", "f", "n"}
	case "ru", "uk", "pl", "cs", "sk", "sr", "hr", "bg":
		return []string{"m", "f", "n"}
	default:
		return []string{""}
	}
}

// emptyCells initialises the (gender × CLDR-category) grid with empty
// strings, restricted to the categories the locale's CLDR rules actually
// produce. "zero" is always included so translators can supply an explicit
// zero-override even when CLDR folds 0 into another category.
func emptyCells(lang string) map[string]string {
	out := map[string]string{}
	for _, g := range genderInventory(lang) {
		for _, c := range activeCLDRCategories(lang) {
			out[g+"."+c] = ""
		}
	}
	return out
}

// activeCLDRCategories discovers which CLDR plural categories the locale
// uses by sampling representative cardinal values. The set is always
// canonical-ordered and always contains "zero" so a translator can
// hand-supply a zero-override that SynPrinter prefers when count==0.
//
// Sampling beats hardcoding per-language tables: the sample values cover
// every common CLDR rule branch (1, 2, few-range, many-range, hundreds),
// and adding a locale only requires it to be valid in golang.org/x/text.
//
// An unparseable tag falls back to the full six-category set — losing
// catalog cleanliness but never dropping a cell the runtime might query.
func activeCLDRCategories(lang string) []string {
	tag, err := language.Parse(lang)
	if err != nil {
		return cldrCategories
	}
	seen := map[string]bool{"zero": true}
	samples := []int{0, 1, 2, 3, 4, 5, 6, 10, 11, 21, 22, 100}
	for _, n := range samples {
		seen[cldrFormName(plural.Cardinal.MatchPlural(tag, n, 0, 0, 0, 0))] = true
	}
	out := make([]string, 0, len(cldrCategories))
	for _, cat := range cldrCategories {
		if seen[cat] {
			out = append(out, cat)
		}
	}
	return out
}

// cldrFormName converts a plural.Form into its CLDR category name. Mirrors
// wi18n/syn.go::cldrCategory but on the gen_i18n side.
func cldrFormName(f plural.Form) string {
	switch f {
	case plural.Zero:
		return "zero"
	case plural.One:
		return "one"
	case plural.Two:
		return "two"
	case plural.Few:
		return "few"
	case plural.Many:
		return "many"
	}
	return "other"
}

// ── Per-language inflections catalog I/O ────────────────────────────────────

// loadFlexJSON reads a <lang>.inflections.json catalog and its sibling
// <lang>.inflections.meta.json, merging them into the in-memory
// []wi18n.FlexEntry form. Legacy combined files migrate transparently on
// the next save — see loadJSON for the mirror-image logic on the text side.
func loadFlexJSON(path string) ([]wi18n.FlexEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var entries []wi18n.FlexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse flex json: %w", err)
	}
	metas, err := loadFlexMetas(metaPath(path))
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if i < len(metas) {
			entries[i].Context = metas[i].Context
			entries[i].Ctxdetail = metas[i].Ctxdetail
		}
	}
	return entries, nil
}

func loadFlexMetas(path string) ([]wi18n.FlexEntryMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var metas []wi18n.FlexEntryMeta
	if err := json.Unmarshal(data, &metas); err != nil {
		return nil, fmt.Errorf("parse flex meta json: %w", err)
	}
	return metas, nil
}

// saveFlexJSON writes the data half to path and the meta half to the
// sibling meta path.
func saveFlexJSON(path string, entries []wi18n.FlexEntry) error {
	datas := make([]wi18n.FlexEntryData, len(entries))
	metas := make([]wi18n.FlexEntryMeta, len(entries))
	for i, e := range entries {
		datas[i] = e.FlexEntryData
		metas[i] = e.FlexEntryMeta
	}
	if err := writeIndentedJSON(path, datas); err != nil {
		return err
	}
	return writeIndentedJSON(metaPath(path), metas)
}

// firstFlexContext and flexCtxdetail mirror formatFirstContext/formatCtxdetail
// but draw from flexOccurrences.
func firstFlexContext(i int32) string {
	occs := flexOccurrences[i]
	if len(occs) == 0 {
		return ""
	}
	o := occs[0]
	return fmt.Sprintf("%s:%d:%d", o.Path, o.Line, o.Col)
}

func flexCtxdetail(i int32) string {
	occs := flexOccurrences[i]
	if len(occs) == 0 {
		return ""
	}
	parts := make([]string, len(occs))
	for j, o := range occs {
		parts[j] = fmt.Sprintf("%s@%s:%d:%d", o.Tag, o.Path, o.Line, o.Col)
	}
	return strings.Join(parts, "<br/>")
}

// lintFlexBlocks emits diagnostic warnings when the deflang has no gender
// axis (e.g. en-US) yet at least one target locale does (e.g. pt-BR, de-DE)
// AND some flex block lacks an @var. Without @var the catalog collapses to
// a single degenerate gender column — the translator into the gendered
// locale has nowhere to place masculine/feminine forms without the webdev
// revisiting the template. Flagging it here saves a round-trip.
//
// No-op when the deflang itself has multiple genders (webdev already writes
// @var naturally) or when no target locale demands gender.
func lintFlexBlocks(langs map[string]bool, defLang string) {
	if len(flexBlocks) == 0 {
		return
	}
	if len(genderInventory(defLang)) > 1 {
		return
	}
	var gendered []string
	for lang := range langs {
		if lang == defLang {
			continue
		}
		if len(genderInventory(lang)) > 1 {
			gendered = append(gendered, lang)
		}
	}
	if len(gendered) == 0 {
		return
	}
	sort.Strings(gendered)
	for i, fb := range flexBlocks {
		if fb.GenderVar != "" {
			continue
		}
		loc := firstFlexContext(int32(i))
		if loc == "" {
			loc = "<unknown>"
		}
		G.Logf(2, "lint: flex block #%d %q at %s has no @var; target locales %v need gender — consider adding @<var> to the template",
			i, flexLabel(fb), loc, gendered)
	}
}

// lintProgrammableFlex emits a soft diagnostic for any programmable block that
// has a ~$var (dynamic flexbind) but no *var engine candidate. Such a block
// has nothing to inflect the dynamic value with, so at runtime it falls back to
// the verbatim value (a webdev-owned, visible degradation). It is a soft lint,
// not a hard error: a future ~$var-as-its-own-engine (deferred to v0.15.1)
// could supply the engine via Priority, which the build cannot see.
func lintProgrammableFlex() {
	for i, fb := range flexBlocks {
		if len(fb.FlexBinds) > 0 && len(fb.StarVars) == 0 {
			loc := firstFlexContext(int32(i))
			if loc == "" {
				loc = "<unknown>"
			}
			G.Logf(2, "lint: flex block #%d %q at %s has ~$ flexbind but no *engine; dynamic value rendered verbatim — add a *<engine> to inflect it",
				i, flexLabelFor(int32(i), fb), loc)
		}
	}
}

// emitFlexCatalogs writes one <lang>.inflections.json per language currently
// present in i18nDir (using the text catalogs as the language set) plus the
// deflang catalog. Existing cells are remapped by canonical key.
//
// deflang is always processed first so its cells are available as source text
// for the translator pass on non-deflang languages.
func emitFlexCatalogs(i18nDir, defLang string) error {
	// Discover the language set from existing text catalogs.
	langFiles, err := filepath.Glob(filepath.Join(i18nDir, "*.json"))
	if err != nil {
		return fmt.Errorf("listing languages: %w", err)
	}
	langs := map[string]bool{defLang: true}
	for _, f := range langFiles {
		base := filepath.Base(f)
		if strings.Contains(base, ".inflections.") {
			continue
		}
		lang := strings.TrimSuffix(base, ".json")
		if strings.HasSuffix(lang, ".meta") {
			continue
		}
		langs[lang] = true
	}

	lintFlexBlocks(langs, defLang)
	lintProgrammableFlex()

	// If no flex blocks were found, prune any orphan inflections files so we
	// don't leave stale rules behind.
	if len(flexBlocks) == 0 {
		for lang := range langs {
			p := filepath.Join(i18nDir, lang+".inflections.json")
			_ = os.Remove(p)
			_ = os.Remove(metaPath(p))
		}
		return nil
	}

	// Process deflang first to get source cells for the translator.
	defOut, err := buildFlexEntriesForLang(i18nDir, defLang, defLang)
	if err != nil {
		return err
	}
	defPath := filepath.Join(i18nDir, defLang+".inflections.json")
	if err := saveFlexJSON(defPath, defOut); err != nil {
		return fmt.Errorf("save %s: %w", defPath, err)
	}

	for lang := range langs {
		if lang == defLang {
			continue
		}
		out, err := buildFlexEntriesForLang(i18nDir, lang, defLang)
		if err != nil {
			return err
		}
		applyFlexTranslations(out, defOut, defLang, lang)
		path := filepath.Join(i18nDir, lang+".inflections.json")
		if err := saveFlexJSON(path, out); err != nil {
			return fmt.Errorf("save %s: %w", path, err)
		}
	}
	return nil
}

// buildFlexEntriesForLang builds the []wi18n.FlexEntry slice for one language:
// carry from the previous run, then the dict pass (when -auto-flex is set).
// The translator pass is NOT applied here — the caller does that after deflang
// is available as source.
func buildFlexEntriesForLang(i18nDir, lang, defLang string) ([]wi18n.FlexEntry, error) {
	path := filepath.Join(i18nDir, lang+".inflections.json")
	old, err := loadFlexJSON(path)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	oldByKey := map[string]wi18n.FlexEntry{}
	for i := range old {
		oldByKey[old[i].Label] = old[i]
	}

	var dict *Dict
	if autoFlex {
		dictPath := filepath.Join(dictDir, lang+".db")
		d, err := loadDict(dictPath)
		if err != nil {
			G.Logf(2, "warn: failed to load %s: %v (auto-flex skipped for %s)", dictPath, err, lang)
		} else if d == nil {
			G.Logf(2, "warn: no dict at %s (auto-flex skipped for %s)", dictPath, lang)
		} else {
			dict = d
		}
	}

	out := make([]wi18n.FlexEntry, len(flexBlocks))
	for i, fb := range flexBlocks {
		// Programmable block: a per-locale Content phrase, no gender×CLDR grid
		// and no dict auto-fill. The deflang carries the source phrase; other
		// locales start empty and remap their translation by content (Label).
		if content, ok := flexContent[int32(i)]; ok {
			label := content
			outContent := ""
			revised := false
			if prev, ok := oldByKey[label]; ok {
				outContent = prev.Content
				revised = prev.Revised
			}
			if lang == defLang {
				outContent = content
			}
			out[i] = wi18n.FlexEntry{
				FlexEntryData: wi18n.FlexEntryData{
					Label:   label,
					Content: outContent,
					Revised: revised,
				},
				FlexEntryMeta: wi18n.FlexEntryMeta{
					Context:   firstFlexContext(int32(i)),
					Ctxdetail: flexCtxdetail(int32(i)),
				},
			}
			continue
		}

		label := flexLabel(fb)
		cells := emptyCells(lang)
		revised := false
		sources := map[string]string{}
		if prev, ok := oldByKey[label]; ok {
			// Preserve every previous cell and its per-cell source, even if
			// its gender prefix doesn't fit the current inventory — losing
			// translator work is worse than carrying extra keys. Lint will
			// flag mismatched inventories separately.
			for k, v := range prev.Cells {
				cells[k] = v
			}
			for k, v := range prev.Sources {
				sources[k] = v
			}
			revised = prev.Revised
		}
		if dict != nil {
			filledSrcs, err := autoFillCells(dict, fb, cells, lang)
			if err != nil {
				G.Fatalf(1, "error: %v\n  in flex block: %s\n  source: %s", err, label, firstFlexContext(int32(i)))
			}
			for k, v := range filledSrcs {
				sources[k] = v
			}
		}
		var sourcesOut map[string]string
		if len(sources) > 0 {
			sourcesOut = sources
		}
		out[i] = wi18n.FlexEntry{
			FlexEntryData: wi18n.FlexEntryData{
				Label:   label,
				Cells:   cells,
				Revised: revised,
				Sources: sourcesOut,
			},
			FlexEntryMeta: wi18n.FlexEntryMeta{
				Context:   firstFlexContext(int32(i)),
				Ctxdetail: flexCtxdetail(int32(i)),
			},
		}
	}
	return out, nil
}

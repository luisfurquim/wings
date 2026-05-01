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

	"github.com/luisfurquim/wprana/expr"
	"github.com/luisfurquim/wprana/wi18n"
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
				fmt.Fprintf(os.Stderr, "warn: flex block parse error in %q: %v (left as-is)\n", inner, err)
				b.WriteString(text[i : j+2])
				i = j + 2
				continue
			}
			canonical := canonicalFlexKey(inner)
			idx := resolveFlex(canonical, fb)
			if recordOccurrence != nil {
				recordOccurrence(idx)
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
		fmt.Fprintf(os.Stderr,
			"lint: flex block #%d %q at %s has no @var; target locales %v need gender — consider adding @<var> to the template\n",
			i, flexLabel(fb), loc, gendered)
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
	defOut, err := buildFlexEntriesForLang(i18nDir, defLang)
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
		out, err := buildFlexEntriesForLang(i18nDir, lang)
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
func buildFlexEntriesForLang(i18nDir, lang string) ([]wi18n.FlexEntry, error) {
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
			fmt.Fprintf(os.Stderr, "warn: failed to load %s: %v (auto-flex skipped for %s)\n", dictPath, err, lang)
		} else if d == nil {
			fmt.Fprintf(os.Stderr, "warn: no dict at %s (auto-flex skipped for %s)\n", dictPath, lang)
		} else {
			dict = d
		}
	}

	out := make([]wi18n.FlexEntry, len(flexBlocks))
	for i, fb := range flexBlocks {
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
				fmt.Fprintf(os.Stderr, "error: %v\n  in flex block: %s\n  source: %s\n", err, label, firstFlexContext(int32(i)))
				os.Exit(1)
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

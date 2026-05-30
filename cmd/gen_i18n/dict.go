package main

import (
	"encoding/gob"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/luisfurquim/wings/expr"
)

// ── CLI-tied globals ────────────────────────────────────────────────────────

// autoFlex is the resolved value of the -auto-flex flag. When false, the
// dict-consulting code paths are skipped entirely and gen_i18n produces the
// same output it always did (empty cells).
var autoFlex bool

// dictDir is where loadDict looks for <lang>.db files. Resolved from
// -dict-dir, defaulting to the wprana module's bundled dict directory.
var dictDir string

// dictSource is the value stamped into FlexEntryData.Source when a cell is
// auto-populated. Provenance for translators / the wlate GUI. The "@<sha>"
// suffix is reserved for when dictbuild starts fetching from a known
// upstream commit and can stamp the SHA on its output.
var dictSource = "dict:unitex-lingua"

// defaultDictDir returns the bundled dict directory. Looks up the module
// path of github.com/luisfurquim/wings and joins cmd/gen_i18n/dicts.
// Falls back to a relative path when go/build cannot resolve the module
// (e.g. running from a tarball with no GOPATH).
func defaultDictDir() string {
	pkg, err := build.Import("github.com/luisfurquim/wings/cmd/gen_i18n", "", build.FindOnly)
	if err != nil || pkg.Dir == "" {
		return filepath.Join("cmd", "gen_i18n", "dicts")
	}
	return filepath.Join(pkg.Dir, "dicts")
}

// ── Dict wire format ────────────────────────────────────────────────────────
//
// These types MUST stay byte-for-byte compatible with cmd/dictbuild/main.go
// because gob decodes by field name and shape — duplicating the
// declarations across packages is the deliberate trade for keeping the two
// commands independent of each other (no shared Go package, no import
// cycle, easy stand-alone evolution).

// Dict is the in-memory shape of a compiled flexion dictionary (.db file).
type Dict struct {
	Lemmas    map[string]*Lemma
	FormIndex map[string][]FormRef
}

// FormRef links an inflected form back to its lemma and grammatical features.
type FormRef struct {
	Lemma string
	Class string
	Genre string
	Count string
}

// Lemma is a base word together with its inflected forms.
type Lemma struct {
	Category string
	Forms    map[string]Inflect
}

// Inflect encodes one inflected form as a suffix diff against the lemma.
type Inflect struct {
	DiffPos int    // common-prefix length WITH the lemma, in RUNES
	Suffix  string // replaces the lemma from DiffPos onward
}

// ── Loader ──────────────────────────────────────────────────────────────────

// loadDict reads a <lang>.db produced by cmd/dictbuild. Returns (nil, nil)
// when the file does not exist — auto-fill is opt-in per language.
func loadDict(path string) (*Dict, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var d Dict
	if err := gob.NewDecoder(f).Decode(&d); err != nil {
		return nil, fmt.Errorf("decode dict %s: %w", path, err)
	}
	return &d, nil
}

// ── Surface-form reconstruction ─────────────────────────────────────────────

// reconstructForm returns the inflected surface form by joining the
// rune-prefix of the lemma (length DiffPos) with the stored Suffix.
// Always operates in runes because Portuguese and other Romance languages
// share accented characters where byte-prefixing would corrupt the form.
func reconstructForm(lemma string, in Inflect) string {
	runes := []rune(lemma)
	if in.DiffPos > len(runes) {
		return lemma + in.Suffix
	}
	return string(runes[:in.DiffPos]) + in.Suffix
}

// ── Per-tilde resolution ────────────────────────────────────────────────────

// cldrToCount maps a CLDR plural category to the DELAF count letter.
//   - one, zero → s (singular form; zero may be hand-overridden)
//   - everything else (other, two, few, many) → p
func cldrToCount(cat string) string {
	switch cat {
	case "one", "zero":
		return "s"
	default:
		return "p"
	}
}

// dualLemmaPick returns the lemma that matches cellGenre when word uses the
// pipe-separated dual form (e.g. "pai|mãe" → m=pai, f=mãe). The position
// order is masculine, feminine, neuter — matching the gender column order
// in the inflections.json grid.
func dualLemmaPick(word, cellGenre string) (string, bool) {
	parts := strings.Split(word, "|")
	genderOrder := []string{"m", "f", "n"}
	for i, p := range parts {
		if i >= len(genderOrder) {
			break
		}
		if genderOrder[i] == cellGenre {
			return p, true
		}
	}
	return "", false
}

// resolveTilde returns the inflected form of `word` for the (cellGenre,
// cellCount) cell. word is either a single lemma or "m|f"/"m|f|n" for
// dual-lemma gender splits.
//
// Returns ("", false, nil) when the dict has no entry — caller skips the cell.
// Returns ("", false, err) for true homographs: multiple noun-class (Class="")
// refs pointing to different lemmas. The caller must treat this as a fatal
// error and report the conflicting lemmas to the developer.
func resolveTilde(dict *Dict, word, cellGenre, cellCount string) (string, bool, error) {
	lookupWord := word
	if strings.Contains(word, "|") {
		picked, ok := dualLemmaPick(word, cellGenre)
		if !ok {
			return "", false, nil
		}
		lookupWord = picked
	}

	refs := dict.FormIndex[lookupWord]
	if len(refs) == 0 {
		return "", false, nil
	}

	// Collect all noun-class (Class="") refs and deduplicate by lemma. Two
	// kinds of multi-ref words need separate treatment:
	//
	//   1. Epicene words (e.g. "abacial" adj): same lemma, refs for both m
	//      and f. Safe — pick the ref whose Genre matches cellGenre so that
	//      the target key is computed against the correct paradigm column.
	//
	//   2. True homographs (e.g. "adutora" N fs from "adutora" AND from
	//      "adutor"): different lemmas that produce different inflection sets.
	//      We cannot pick reliably — hard error so the developer uses ~m|f.
	var nounRefs []int
	seen := map[string]struct{}{}
	for i := range refs {
		if refs[i].Class == "" {
			nounRefs = append(nounRefs, i)
			seen[refs[i].Lemma] = struct{}{}
		}
	}
	if len(seen) > 1 {
		lemmas := make([]string, 0, len(seen))
		for l := range seen {
			lemmas = append(lemmas, l)
		}
		sort.Strings(lemmas)
		return "", false, fmt.Errorf(
			"ambiguous homograph ~%s: matches lemmas %s; use the ~m|f dual form to disambiguate",
			word, strings.Join(lemmas, ", "),
		)
	}

	// Pick the best noun-class ref: for epicenes, prefer the one whose Genre
	// matches cellGenre (avoids a mismatch in the target key). Fall back to the
	// first noun ref, then to refs[0] when no noun ref exists (pure verb/adj).
	var chosen *FormRef
	for _, i := range nounRefs {
		if refs[i].Genre == cellGenre {
			chosen = &refs[i]
			break
		}
	}
	if chosen == nil && len(nounRefs) > 0 {
		chosen = &refs[nounRefs[0]]
	}
	if chosen == nil {
		chosen = &refs[0]
	}

	// Verb-like refs (Class non-empty, Genre empty) ignore the cell's
	// gender — verb conjugations don't agree with subject gender in any
	// of the languages we support. The target is purely (Class, Count).
	targetGenre := cellGenre
	if chosen.Genre == "" {
		targetGenre = ""
	}
	targetKey := chosen.Class + targetGenre + cellCount

	lem, ok := dict.Lemmas[chosen.Lemma]
	if !ok {
		return "", false, nil
	}
	if in, ok := lem.Forms[targetKey]; ok {
		return reconstructForm(chosen.Lemma, in), true, nil
	}
	// Singulare-tantum fallback: invariant words (e.g. some adverbs) have
	// only the bare-class form. Use the singular cell's value for both.
	if cellCount == "s" {
		if in, ok := lem.Forms[chosen.Class+targetGenre]; ok {
			return reconstructForm(chosen.Lemma, in), true, nil
		}
	}
	return "", false, nil
}

// ── Per-block auto-fill ─────────────────────────────────────────────────────

// autoFillCells walks fb.Tokens to compose one sentence per (gender,
// CLDR-category) cell. cells is mutated in place: only EMPTY cells are
// touched, so any prior translator-supplied content survives untouched.
//
// A cell is filled only when every ~word in the block resolves; if any
// word fails to resolve, that cell is left empty rather than emit a
// half-flexed sentence. Returns (true, nil) when at least one cell was
// populated. Returns (false, err) on the first hard homograph error —
// the caller must treat this as fatal.
// autoFillCells fills empty cells in the (gender × CLDR-category) grid using
// the dictionary. Returns a map of cellKey → provenance string for every cell
// that was newly populated; an empty map means nothing was filled. The caller
// merges the returned map into FlexEntryData.Sources.
func autoFillCells(dict *Dict, fb expr.FlexBlock, cells map[string]string, lang string) (map[string]string, error) {
	if dict == nil {
		return nil, nil
	}
	filled := map[string]string{}
	for _, g := range genderInventory(lang) {
		for _, cat := range activeCLDRCategories(lang) {
			key := g + "." + cat
			if cells[key] != "" {
				continue
			}
			sentence, ok, err := composeCell(dict, fb, g, cldrToCount(cat))
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			cells[key] = sentence
			filled[key] = dictSource
		}
	}
	return filled, nil
}

// composeCell builds the sentence for one cell. Returns ("", false, nil) when
// any ~word fails to resolve — the caller leaves the cell empty. Returns
// ("", false, err) on a hard homograph error that must not be swallowed.
func composeCell(dict *Dict, fb expr.FlexBlock, cellGenre, cellCount string) (string, bool, error) {
	var sb strings.Builder
	sep := ""
	for _, t := range fb.Tokens {
		switch t.Type {
		case expr.TokAtVar, expr.TokFlexIdx:
			// metadata only, no emission
		case expr.TokPctVar:
			sb.WriteString(sep)
			sb.WriteString("{n}")
			sep = " "
		case expr.TokTildeWord:
			inflected, ok, err := resolveTilde(dict, t.StrVal, cellGenre, cellCount)
			if err != nil {
				return "", false, err
			}
			if !ok {
				return "", false, nil
			}
			sb.WriteString(sep)
			sb.WriteString(inflected)
			sep = " "
		case expr.TokIdent, expr.TokStr:
			sb.WriteString(sep)
			sb.WriteString(t.StrVal)
			sep = " "
		}
	}
	return sb.String(), true, nil
}

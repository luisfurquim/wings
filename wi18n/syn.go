//go:build js && wasm

package wi18n

import (
	"strconv"
	"strings"
	"sync"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/expr"
)

// ── Flex catalog state ──────────────────────────────────────────────────────

var (
	flexMu    sync.RWMutex
	flexTable []FlexEntry
	flexLang  language.Tag
)

// setFlexCatalog atomically installs a freshly loaded table + parsed tag.
func setFlexCatalog(entries []FlexEntry, tag language.Tag) {
	flexMu.Lock()
	flexTable = entries
	flexLang = tag
	flexMu.Unlock()
}

// ── SynPrinter: runtime substitution of {{@g %c #N}} ────────────────────────

// synPrinter is installed into wings.SynPrinter once the inflections
// catalog is available.
//
// Flow:
//  1. Parse the tokens into a FlexBlock.
//  2. Resolve @genderVar and %countVar against the live data context.
//  3. Compute the CLDR plural category from the count value.
//  4. Look up flexTable[Idx].Cells["<gender>.<category>"].
//  5. Fallback chain: empty zero → try one; any other empty → try other;
//     still empty → return the rule's Label (translator-facing stem) so
//     the page remains readable instead of blank.
//  6. Substitute every "{n}" in the chosen cell with the numeric count.
func synPrinter(toks []wings.RefNode, ctx wings.Ctx) string {
	toksCopy := append([]wings.RefNode(nil), toks...)
	fb, err := expr.ParseFlexBlock(&toksCopy)
	if err != nil {
		wings.G.Logf(1, "wi18n: SynPrinter parse error: %v\n", err)
		return ""
	}

	flexMu.RLock()
	table := flexTable
	tag := flexLang
	flexMu.RUnlock()

	if fb.Idx < 0 || fb.Idx >= len(table) {
		wings.G.Logf(2, "wi18n: flex rule #%d out of range (table=%d)\n", fb.Idx, len(table))
		return ""
	}
	entry := table[fb.Idx]
	cells := entry.Cells
	if cells == nil {
		return entry.Label
	}

	// Resolve variable values from context. Path-valued vars
	// (`@user.gender`, `%cart[i].qty`) go through wings.Solve; bare names
	// use the cheap single-level fallback.
	genderVal := resolveFlexVar(fb.GenderVar, fb.GenderPath, ctx)
	countVal := resolveFlexVar(fb.CountVar, fb.CountPath, ctx)

	countInt := asInt(countVal)
	cat := cldrCategory(tag, countInt)

	gender := firstNonEmpty(asStr(genderVal), "")

	// Explicit-zero override: when count is exactly 0 and the translator
	// supplied a `<gender>.zero` cell, use it even in locales where CLDR
	// folds 0 into `one` (e.g. pt-BR). Empty zero cells fall through to the
	// CLDR-derived category.
	if fb.CountVar != "" && countInt == 0 {
		if v, ok := cells[gender+".zero"]; ok && v != "" {
			cat = "zero"
		}
	}

	key := gender + "." + cat

	cell, ok := cells[key]
	if !ok || cell == "" {
		// Fallback chain.
		if cat == "zero" {
			if v, ok := cells[gender+".one"]; ok && v != "" {
				cell = v
			}
		}
		if cell == "" {
			if v, ok := cells[gender+".other"]; ok && v != "" {
				cell = v
			}
		}
		if cell == "" {
			// Last resort: the translator-facing label. Not a locale-clean
			// string, but better than blank.
			cell = entry.Label
		}
	}

	if fb.CountVar != "" && strings.Contains(cell, "{n}") {
		// Render {n} through FmtPrinter so the count honors the locale's
		// grouping and decimal conventions (e.g. "os 1.234 alunos" in pt-BR
		// vs "the 1,234 students" in en-US). countVal carries the original
		// Go typed value — the FmtPrinter type switch picks the right path.
		cell = strings.ReplaceAll(cell, "{n}", wings.FmtPrinter(countVal, wings.Locale, ""))
	}
	return cell
}

// resolveFlexVar resolves a flex sigil variable. When path is populated
// (webdev wrote `@user.gender` or `%cart[i].qty`), it delegates to
// wings.Solve, which understands `.ident` and sub-expressions and sweeps
// the context stack like the main template resolver. The bare case
// (just `@name` / `%name`) keeps the cheap single-level map lookup to
// avoid the reflection overhead for the overwhelmingly common path.
//
// Empty name (and nil path) means the axis is absent in this block —
// callers treat that as a degenerate axis.
func resolveFlexVar(name string, path []wings.RefNode, ctx wings.Ctx) any {
	if len(path) > 0 {
		for _, layer := range ctx {
			if v := wings.Solve(path, layer, ctx); v != nil {
				return v
			}
		}
		return nil
	}
	if name == "" {
		return nil
	}
	for _, layer := range ctx {
		if v := fieldOf(layer, name); v != nil {
			return v
		}
	}
	return nil
}

// fieldOf performs a cheap single-level lookup on map[string]any; other
// shapes return nil. Used only for the bare-ident case of flex vars.
func fieldOf(obj any, key string) any {
	if m, ok := obj.(map[string]any); ok {
		return m[key]
	}
	return nil
}

// asInt coerces a value to an int (0 on failure) for count variables.
func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return 0
}

// asStr coerces a value to a string for gender discriminators.
func asStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func firstNonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// cldrCategory returns the canonical CLDR plural category name
// ("zero"/"one"/"two"/"few"/"many"/"other") for the given locale and
// cardinal count.
func cldrCategory(tag language.Tag, n int) string {
	form := plural.Cardinal.MatchPlural(tag, n, 0, 0, 0, 0)
	switch form {
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

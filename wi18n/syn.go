//go:build js && wasm

package wi18n

import (
	"fmt"
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

	// Programmable rule: assemble the per-locale flex-content phrase, inflecting
	// ~word/~$var via the elected CustomFlex engine. (project_customflex_design)
	if entry.Content != "" {
		return assembleFlexContent(fb, entry.Content, ctx)
	}

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
	// supplied a zero cell, use it even in locales where CLDR folds 0 into
	// `one` (e.g. pt-BR). Checks the gendered key and the gender-degenerate
	// one, so an en-US ".zero" still wins. Empty zero cells fall through.
	if fb.CountVar != "" && countInt == 0 {
		if cells[gender+".zero"] != "" || cells[".zero"] != "" {
			cat = "zero"
		}
	}

	// Pick the cell for the resolved gender. When the locale does not inflect
	// by gender (e.g. en-US has ".one"/".other", never "m."/"f."), the gendered
	// key never matches — retry with the empty gender prefix before falling
	// back to the translator-facing label.
	cell := pickCell(cells, gender, cat)
	if cell == "" && gender != "" {
		cell = pickCell(cells, "", cat)
	}
	if cell == "" {
		// Last resort: the translator-facing label. Not a locale-clean
		// string, but better than blank.
		cell = entry.Label
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

// pickCell returns the best filled cell for (gender, cat): the exact form,
// then the "one" form when a "zero" is missing (CLDR folds 0 into one), then
// the "other" form. Returns "" when none is filled, letting the caller retry
// with a different gender prefix or fall back to the label.
func pickCell(cells map[string]string, gender, cat string) string {
	if v := cells[gender+"."+cat]; v != "" {
		return v
	}
	if cat == "zero" {
		if v := cells[gender+".one"]; v != "" {
			return v
		}
	}
	return cells[gender+".other"]
}

// ── Programmable flex (CustomFlex) ──────────────────────────────────────────

// assembleFlexContent renders a programmable flex rule. fb is the parsed
// control block (`{{@g %c *engine #N}}`); content is the per-locale phrase from
// the catalog. It elects the engine and builds the selector list, then walks
// the content tokens (expr.TokenizeFlexContent): literal text and whitespace go
// out verbatim, $var is resolved and emitted verbatim, %count is locale-
// formatted, and each ~word/~$var is inflected by the engine.
func assembleFlexContent(fb expr.FlexBlock, content string, ctx wings.Ctx) string {
	selectors, engine := flexParticipants(fb, ctx)

	toks := expr.TokenizeFlexContent(content)

	// $var is both emitted (in the walk below) and offered to the engine as a
	// selector, mirroring %count. Unlike @/%/*, $vars live in the content (not
	// the control block), so collect them up front — before any ~word is
	// inflected, since selectors must be complete on the first Flex call.
	for _, tk := range toks {
		if tk.Type == expr.TokDollarVar {
			selectors = append(selectors, wings.FlexSelector{
				Sigil: '$', Name: tk.StrVal, Value: resolveFlexVar(tk.StrVal, tk.Sub, ctx),
			})
		}
	}

	var sb strings.Builder
	for _, tk := range toks {
		switch tk.Type {
		case expr.TokTxt:
			sb.WriteString(tk.StrVal)
		case expr.TokSpace:
			sb.WriteByte(' ')
		case expr.TokDollarVar:
			sb.WriteString(sprintVal(resolveFlexVar(tk.StrVal, tk.Sub, ctx)))
		case expr.TokPctVar:
			cv := resolveFlexVar(tk.StrVal, tk.Sub, ctx)
			sb.WriteString(wings.FmtPrinter(cv, wings.Locale, ""))
		case expr.TokTildeWord:
			sb.WriteString(inflectWord(engine, tk.StrVal, selectors))
		case expr.TokFlexBind:
			word := sprintVal(resolveFlexVar(tk.StrVal, tk.Sub, ctx))
			sb.WriteString(inflectWord(engine, word, selectors))
		}
	}
	return sb.String()
}

// flexParticipants resolves the control-block bindings into the FlexSelector
// list handed to the engine, and elects the engine from the *var candidates
// (highest Priority wins; a tie logs and yields no engine, so words fall back
// to verbatim — a visible, webdev-owned failure). A *var that does not resolve
// to a wings.CustomFlex is logged and skipped.
func flexParticipants(fb expr.FlexBlock, ctx wings.Ctx) ([]wings.FlexSelector, wings.CustomFlex) {
	var selectors []wings.FlexSelector
	if fb.GenderVar != "" || len(fb.GenderPath) > 0 {
		selectors = append(selectors, wings.FlexSelector{
			Sigil: '@', Name: fb.GenderVar, Value: resolveFlexVar(fb.GenderVar, fb.GenderPath, ctx),
		})
	}
	if fb.CountVar != "" || len(fb.CountPath) > 0 {
		selectors = append(selectors, wings.FlexSelector{
			Sigil: '%', Name: fb.CountVar, Value: resolveFlexVar(fb.CountVar, fb.CountPath, ctx),
		})
	}

	var winners []wings.CustomFlex
	var bestPrio uint
	for _, sv := range fb.StarVars {
		v := resolveFlexVar(sv.Var, sv.Path, ctx)
		cf, ok := v.(wings.CustomFlex)
		if !ok {
			wings.G.Logf(1, "wi18n: flex *%s does not implement wings.CustomFlex\n", sv.Var)
			continue
		}
		selectors = append(selectors, wings.FlexSelector{Sigil: '*', Name: sv.Var, Value: v})
		p := priorityOf(cf)
		switch {
		case winners == nil || p > bestPrio:
			winners, bestPrio = []wings.CustomFlex{cf}, p
		case p == bestPrio:
			winners = append(winners, cf)
		}
	}

	switch len(winners) {
	case 1:
		return selectors, winners[0]
	case 0:
		return selectors, nil
	default:
		wings.G.Logf(1, "wi18n: flex engine tie (%d at priority %d); words rendered verbatim\n", len(winners), bestPrio)
		return selectors, nil
	}
}

// priorityOf returns a CustomFlex's engine priority (0 when it does not
// implement wings.Prioritized).
func priorityOf(cf wings.CustomFlex) uint {
	if p, ok := cf.(wings.Prioritized); ok {
		return p.Priority()
	}
	return 0
}

// inflectWord runs word through the engine, falling back to the verbatim word
// (plus a log) when there is no engine or Flex errors — a missing inflection
// stays visible instead of blank, and is the webdev's responsibility to fix by
// supplying an engine.
func inflectWord(engine wings.CustomFlex, word string, selectors []wings.FlexSelector) string {
	if engine == nil {
		wings.G.Logf(2, "wi18n: flex word %q has no engine; rendered verbatim\n", word)
		return word
	}
	out, err := engine.Flex(word, selectors...)
	if err != nil {
		wings.G.Logf(1, "wi18n: flex engine error on %q: %v\n", word, err)
		return word
	}
	return out
}

// sprintVal stringifies a resolved $var/~$var value ("" for nil).
func sprintVal(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
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

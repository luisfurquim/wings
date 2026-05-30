//go:build js && wasm

// Package wldata holds the data types, on-disk loaders, and presentation
// helpers shared by the wlate shell (wp-wlate) and its per-tab editor
// modules (wl-text-editor, wl-flex-editor).
//
// It also declares the contract the shell uses to drive whichever editor is
// active: TextEditor and FlexEditor. Each editor module registers itself with
// the shell at Render time (see the @register trigger), handing over a value
// that satisfies one of these interfaces. The shell then pushes the current
// record into the editor (Display), pulls edits back out (Harvest), or blanks
// it (Clear) — without reaching across the editor's shadow boundary itself.
package wldata

import (
	"sort"
	"strings"

	"github.com/luisfurquim/wings/wi18n"
)

// ── Data types ─────────────────────────────────────────────────────────────

// Config is the relevant slice of wings.json for wlate.
type Config struct {
	DefaultLang string      `json:"defaultLang"`
	Languages   []string    `json:"languages"`
	Wlate       WlateConfig `json:"wlate"`
}

type WlateConfig struct {
	Keys map[string]string `json:"keys"`
}

// TextRecord and InflectionRecord are thin aliases over the wi18n merged
// in-memory views. wlate reads data+meta from split files on disk (data on
// the browser bundle path, meta on the sibling .meta.json), merges them, and
// writes back only the data half on save.
type TextRecord = wi18n.Entry
type InflectionRecord = wi18n.FlexEntry

// ── Shell↔editor contract ──────────────────────────────────────────────────

// TextEditor is the text tab's side of the contract, implemented by the
// wl-text-editor module and consumed by the shell.
type TextEditor interface {
	// Display renders the reference (left) and editable (right) records.
	Display(left, right TextRecord)
	// Harvest reads the editor's DOM back into right, returning true if the
	// content changed (so the shell can mark the session dirty).
	Harvest(right *TextRecord) bool
	// Clear blanks the editor (no current record).
	Clear()
}

// FlexEditor is the inflection tab's side of the contract, implemented by the
// wl-flex-editor module and consumed by the shell.
type FlexEditor interface {
	Display(left, right InflectionRecord)
	Harvest(right *InflectionRecord) bool
	Clear()
}

// ── Context detail humanization ────────────────────────────────────────────

var ctxLabels = map[string]string{
	"title":      "Título da página",
	"header":     "Cabeçalho",
	"footer":     "Rodapé",
	"caption":    "Legenda de tabela",
	"button":     "Botão",
	"label":      "Rótulo de campo",
	"th":         "Cabeçalho de coluna",
	"nav":        "Área de navegação",
	"legend":     "Legenda de formulário",
	"figcaption": "Legenda de figura",
	"summary":    "Resumo expansível",
	"a":          "Texto de link",
	"option":     "Opção de seleção",
	"abbr":       "Abreviação",
	"dt":         "Termo de definição",
}

// HumanizeCtx maps a raw context-detail tag to a translator-friendly label,
// falling back to the tag itself when unknown.
func HumanizeCtx(detail string) string {
	if label, ok := ctxLabels[detail]; ok {
		return label
	}
	return detail
}

// ── Badge helpers ──────────────────────────────────────────────────────────

// SourceToAvatarURL converts a provenance tag to an avatar URL for wlate
// badges. Returns "" for empty, "manual", or unrecognised tags (hides the badge).
//
//	"dict:unitex-lingua"  →  "/avatar/unitex-lingua"
//	"llm:gemma4"          →  "/avatar/llm-gemma4"
func SourceToAvatarURL(s string) string {
	switch {
	case strings.HasPrefix(s, "dict:"):
		return "/avatar/" + strings.TrimPrefix(s, "dict:")
	case strings.HasPrefix(s, "llm:"):
		return "/avatar/llm-" + strings.TrimPrefix(s, "llm:")
	}
	return ""
}

// ── Header label helpers ───────────────────────────────────────────────────

// GenderHeaderLabel returns the short visible label for a gender column.
// Empty string (degenerate inventory) stays visually blank; the aria label
// carries the semantic meaning for screen readers.
func GenderHeaderLabel(g string) string {
	switch g {
	case "m":
		return "M"
	case "f":
		return "F"
	case "n":
		return "N"
	}
	return g
}

// GenderAriaLabel returns the verbose a11y string for a gender column.
func GenderAriaLabel(g string) string {
	switch g {
	case "m":
		return "Masculino"
	case "f":
		return "Feminino"
	case "n":
		return "Neutro"
	case "":
		return "forma única"
	}
	return g
}

// CategoryAriaLabel returns a short Portuguese hint for the CLDR category
// name, for screen readers that won't recognise the technical term.
func CategoryAriaLabel(c string) string {
	switch c {
	case "zero":
		return "zero (forma explícita para nenhum)"
	case "one":
		return "singular"
	case "two":
		return "dual"
	case "few":
		return "poucos"
	case "many":
		return "muitos"
	case "other":
		return "plural geral"
	}
	return c
}

// ── Sort helpers ───────────────────────────────────────────────────────────

// SortedKeys returns the keys of a set in ascending order.
func SortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var cldrOrder = map[string]int{
	"zero": 0, "one": 1, "two": 2, "few": 3, "many": 4, "other": 5,
}

// SortedCLDR sorts CLDR plural categories in canonical order, with any
// unknown categories appended alphabetically.
func SortedCLDR(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		oi, oki := cldrOrder[out[i]]
		oj, okj := cldrOrder[out[j]]
		if oki && okj {
			return oi < oj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return out[i] < out[j]
	})
	return out
}

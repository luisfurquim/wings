package wi18n

// ── Text catalog schema ─────────────────────────────────────────────────────

// EntryData is the wire-format half that ships to the browser. It holds only
// the fields the runtime needs to render a localised string.
//
// On disk: one array of EntryData per language in i18n/<lang>.json.
type EntryData struct {
	Content string `json:"content"`
	Revised bool   `json:"revised"`
}

// EntryMeta is the wire-format half the runtime does NOT need: source
// positions used by translators and tooling. Kept out of the browser bundle.
//
// On disk: one array of EntryMeta per language in i18n/<lang>.meta.json,
// parallel-indexed to the companion <lang>.json (data[i] ↔ meta[i]).
type EntryMeta struct {
	// Context is the first occurrence of the string, formatted as
	// "<path>:<line>:<col>" with the path relative to the gen_i18n --path
	// root (forward-slashed, platform-independent).
	Context string `json:"context"`
	// Ctxdetail lists every occurrence joined by "<br/>", each formatted as
	// "<tag>@<path>:<line>:<col>". The leading tag is the nearest HTML
	// ancestor element; it may carry an "[attr]" suffix when the string was
	// extracted from a translatable attribute (see gen_i18n --attrs).
	Ctxdetail string `json:"ctxdetail"`
}

// Entry is the merged view used in memory by gen_i18n and wlate. It is not
// the wire format — do not marshal []Entry to disk directly. Use EntryData
// (browser bundle) and EntryMeta (server-only companion) instead.
//
// Field semantics:
//   - For the default language, Content is the source string extracted from
//     the template.
//   - For every other language, Content is the translation. Empty string
//     means "not translated yet".
//   - Revised is translator-maintained, flipped in wlate once a human has
//     reviewed the entry. Preserved across gen_i18n runs when the source
//     string has not changed.
type Entry struct {
	EntryData
	EntryMeta
}

// ── Inflection (flex) catalog schema ────────────────────────────────────────

// FlexEntryData is the browser-side half of one inflection rule. Holds the
// translator-filled cell grid used by SynPrinter at runtime, plus the
// translator-facing label for debuggability.
//
// On disk: one array of FlexEntryData per language in
// i18n/<lang>.inflections.json.
type FlexEntryData struct {
	// Label is the translator-facing hint — the non-sigil stem of the
	// original template block. Not used at runtime but kept in the data
	// file (small, and carrying it avoids matching rules by index alone
	// when translators diff JSON files).
	Label string `json:"label"`
	// Cells maps "<gender>.<cldr_category>" to the inflected string (e.g.
	// "m.one", "f.other"). For degenerate-gender templates the gender
	// prefix is empty, so keys look like ".one", ".other".
	Cells map[string]string `json:"cells"`
	// Revised flag, flipped in wlate.
	Revised bool `json:"revised"`
	// Source, when non-empty, marks automatic provenance (e.g.
	// "dict:unitex-lingua@<commit>"), shown in wlate as a small icon.
	Source string `json:"source,omitempty"`
}

// FlexEntryMeta holds the source-position metadata for an inflection rule.
// On disk: i18n/<lang>.inflections.meta.json, parallel-indexed to
// <lang>.inflections.json.
type FlexEntryMeta struct {
	Context   string `json:"context"`
	Ctxdetail string `json:"ctxdetail"`
}

// FlexEntry is the merged in-memory view. Not the wire format.
type FlexEntry struct {
	FlexEntryData
	FlexEntryMeta
}

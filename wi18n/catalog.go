package wi18n

// Entry is one translatable string in a language catalog.
//
// The file on disk is a JSON array of Entry values: i18n/<lang>.json. The
// slice index is the stable identifier that gen_i18n embeds as decimal text
// in the .i18n.html templates; at runtime wi18n uses it to look up the
// translation.
//
// The shape is the same across all roles:
//
//   - For the default language (the one passed to gen_i18n --deflang),
//     Content is the source string extracted from the template.
//   - For every other language, Content is the translation. Empty string
//     means "not translated yet".
//   - Revised is a translator-maintained flag, flipped in the wlate GUI
//     once a human has reviewed the entry. Preserved across gen_i18n runs
//     when the underlying source string has not changed.
//   - Context is the first occurrence of the string in the source tree,
//     formatted as "<path>:<line>:<col>" with the path relative to the
//     gen_i18n --path root. Paths use forward slashes on every OS so the
//     catalog diffs cleanly across platforms.
//   - Ctxdetail lists every occurrence (not just the first), formatted as
//     "<tag>@<path>:<line>:<col>" joined by "<br/>". The leading tag is the
//     nearest HTML ancestor element and gives translators the semantic
//     context (e.g. "button" vs. "th" vs. "legend") which often determines
//     wording, capitalization, and length constraints.
type Entry struct {
	Content   string `json:"content"`
	Revised   bool   `json:"revised"`
	Context   string `json:"context"`
	Ctxdetail string `json:"ctxdetail"`
}

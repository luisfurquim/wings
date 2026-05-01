// Package translator provides a backend-agnostic interface for machine
// translation of wprana i18n catalog entries. Two implementations are
// available: OpenAI-compatible chat completions (openai.go) and LibreTranslate
// (libretranslate.go). Both are selected via the gen_i18n.json config file.
package translator

import "context"

// SimpleTextKey is the Cells map key used for plain text catalog entries
// (non-flex). Flex entries use "<gender>.<cldr_category>" keys instead.
const SimpleTextKey = ""

// Entry is one translatable unit. For simple text catalog entries Cells has a
// single key "" (SimpleTextKey) mapping to the source string. For flex entries
// keys follow the "<gender>.<cldr_category>" convention (e.g. "m.one",
// "f.other"). Only cells that need translation are included — already-filled
// cells are omitted by the caller.
type Entry struct {
	Label   string            // translator-facing label or short description
	Context string            // source-position hint (e.g. "index.html:42:7")
	Cells   map[string]string // source text per cell key
}

// Response holds translated cells for a batch. Entries is parallel to the
// entries slice passed to Translate. Failed records per-cell validation
// failures keyed by "<label>:<cellKey>" (or "<label>:" for simple text),
// mapping to a human-readable reason string.
type Response struct {
	Entries []Entry
	Failed  map[string]string
}

// DefaultSystemPrompt is the built-in LLM system prompt. It can be overridden
// via the system_prompt field in gen_i18n.json.
const DefaultSystemPrompt = `You are a professional UI translator.
Translate the given strings from the source language to the target language.

Rules (STRICT — violations cause the translation to be discarded):
1. Tokens matching the pattern {{...}} are TEMPLATE REFERENCES — copy them verbatim, never translate or alter them.
2. Tokens matching %word (e.g. %n, %count) are NUMERIC PLACEHOLDERS — copy them verbatim.
3. Tokens starting with ~ (e.g. ~aluno, ~arquivo) are INFLECTABLE WORDS — translate AND inflect them according to the gender and grammatical number indicated by the cell key suffix (m=masculine, f=feminine; one/other/zero/few/many = CLDR plural category).
4. Respond ONLY with a JSON object whose keys are the cell keys from the input and whose values are the translated strings. No explanation, no markdown fences.
5. If a source cell is empty, omit that key from the response.`

// Translator translates batches of catalog entries from one language to another.
type Translator interface {
	// Translate sends a batch of source entries and returns their translations.
	// Response.Entries is parallel to entries. Cell-level validation failures
	// are reported in Response.Failed; only hard transport or protocol errors
	// are returned as err.
	Translate(ctx context.Context, srcLang, dstLang string, entries []Entry) (Response, error)

	// SourceTag returns the provenance string written into FlexEntryData.Sources
	// and text Entry metadata, e.g. "llm:gemma4" or "libretranslate".
	SourceTag() string
}

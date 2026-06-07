package main

import "golang.org/x/text/language"

// langSource describes where the LGPLLR DELAF for a given BCP-47 tag lives in
// the unitex-lingua GitHub repository. The repository organises each language
// under a per-tag subdirectory (e.g. "pt-BR/", "pt-PT/") containing a Dela/
// folder with the compiled `.bin` plus its companion `.inf` table. The
// uncompressed text DELAF is NOT distributed — it must be reconstituted
// locally with UnitexToolLogger, which is why dictbuild needs both files.
type langSource struct {
	// Subdir is the top-level directory inside the unitex-lingua repository.
	// Equal to the BCP-47 tag for every entry today, but kept explicit in case
	// upstream ever diverges.
	Subdir string
	// Base is the file stem under <Subdir>/Dela/ (without extension).
	// dictbuild downloads <Base>.bin and <Base>.inf, then asks UnitexToolLogger
	// to produce <Base>.dic from them.
	//
	// Special characters are intentionally preserved verbatim — fetch.go
	// URL-escapes them when constructing download URLs. Examples in the wild:
	// "2011+proprium" (Norwegian), "Georgian (Ancient)_may2009" (Old Georgian),
	// "dela_pl-" (Polish, trailing hyphen).
	Base string
}

// langSources is the per-locale registry of dictbuild's auto-fetch source.
// Coverage matches every directory under https://github.com/UnitexGramLab/unitex-lingua
// that ships at least one usable DELAF `.bin` + `.inf` pair. For each
// language we picked the most general/inflected dictionary available;
// compound-only (DELACF), proper-noun-only, abbreviation-only, and tagger
// data files were skipped because they don't contribute the gender/count
// flexions gen_i18n consumes.
//
// The parser inside dictbuild is still PT-centric (aliasHomograph rewrites
// PT-BR-specific 1s→3s tenses). Producing a `.db` for any other language
// will succeed structurally — the FormIndex / Lemma / Inflect shapes are
// language-neutral — but downstream gen_i18n consumption may need
// language-specific tweaks to interpret the verbal-class codes correctly.
// That's an open task tracked separately from this fetch table.
//
// Languages NOT covered (with rationale):
//
//   - "ar" — Arabic ships separate DELAF_N (nouns) and DELAF_V (verbs) bins;
//     the current single-bin pipeline can't merge them. Add multi-bin
//     support or pick one before re-enabling.
//   - "ko" — Korean's Dela/ directory is empty in upstream.
//   - "zxx-*"  — placeholder/skeleton entries, no real data.
//
// Several entries are samples or partial dictionaries (el's "30percent",
// nn's "Dela-sample", grc's "AG_demo_dico"). They are the only artefacts
// upstream ships, so they are listed; expect lower coverage there.
var langSources = map[string]langSource{
	"de":      {Subdir: "de", Base: "dela"},
	"el":      {Subdir: "el", Base: "dela-30percent"},
	"en":      {Subdir: "en", Base: "dela-en-public"},
	"es":      {Subdir: "es", Base: "delaf"},
	"fi":      {Subdir: "fi", Base: "pien_DELAF_sanasto"},
	"fr":      {Subdir: "fr", Base: "Dela_fr"},
	"grc":     {Subdir: "grc", Base: "AG_demo_dico"},
	"it":      {Subdir: "it", Base: "mini-delaf"},
	"la":      {Subdir: "la", Base: "perseus-lewis-short"},
	"mg":      {Subdir: "mg", Base: "free-DEMA-VSflx"},
	"nn":      {Subdir: "nn", Base: "Dela-sample"},
	"no":      {Subdir: "no", Base: "2011+proprium"},
	"oge":     {Subdir: "oge", Base: "Georgian (Ancient)_may2009"},
	"pl":      {Subdir: "pl", Base: "dela_pl-"},
	"pt-BR":   {Subdir: "pt-BR", Base: "DELAF_PB_2018"},
	"pt-PT":   {Subdir: "pt-PT", Base: "Delaf_V2"},
	"ru":      {Subdir: "ru", Base: "CISLEXru_igrok"},
	"sr-Cyrl": {Subdir: "sr-Cyrl", Base: "cirdelaf-SrpskiU"},
	"sr-Latn": {Subdir: "sr-Latn", Base: "latdelaf-SrpskiU"},
	"th":      {Subdir: "th", Base: "dela"},
	"zh":      {Subdir: "zh", Base: "segdic_unitex_pinyin_2017"},
}

// providerAvatarURLs maps the dict-provider name (the part after "dict:" in the
// gen_i18n Sources values) to the public URL of its avatar image. dictbuild
// downloads each avatar alongside the dictionary files and caches it under
// <state-dir>/avatars/<provider>.png so serve.go can serve it offline.
//
// Add an entry here whenever a new dict provider is added to gen_i18n.
var providerAvatarURLs = map[string]string{
	"unitex-lingua": "https://avatars.githubusercontent.com/u/7904881?s=48&v=4",
}

// resolveLangSource looks up a tag in langSources, accepting both the exact
// keys above and BCP-47 normalised variants (e.g. "PT-BR" → "pt-BR"). It
// returns the source, the key used to address files on disk
// (binPath/infPath/cache subdir, and ultimately the <key>.db filename), and
// ok=false when the locale is unknown.
//
// When a region/script variant has no dedicated dictionary, it falls back to
// the base language ("en-US" → "en", "es-AR" → "es"): the returned key is then
// the base, so the output is honestly named "en.db" (NOT "en-US.db" — that
// would pretend a localised dictionary exists). The loader side mirrors this
// region→base fallback when it looks the file up. The caller can detect the
// fallback by comparing the returned key against the requested tag.
//
// The raw-key path is tried first so non-standard codes such as "oge"
// (Old Georgian, ISO 639-3) work even if golang.org/x/text/language wouldn't
// canonicalise them the same way the upstream directory is named.
func resolveLangSource(tag string) (langSource, string, bool) {
	if src, ok := langSources[tag]; ok {
		return src, tag, true
	}
	t, err := language.Parse(tag)
	if err != nil {
		return langSource{}, "", false
	}
	// BCP-47 canonicalisation so case/format variants ("PT-BR", "pt-br")
	// resolve to the canonical registry key ("pt-BR"). Tried after the
	// raw-key path so non-standard ISO 639-3 codes such as "oge" — which
	// language.Parse would canonicalise away — still resolve via their
	// verbatim key.
	if canon := t.String(); canon != tag {
		if src, ok := langSources[canon]; ok {
			return src, canon, true
		}
	}
	// Region/script fallback: no dedicated dictionary for this exact locale,
	// so use the base language ("en-US" → "en", "fr-CA" → "fr"). pt-BR/pt-PT
	// have dedicated entries and matched above, so Portuguese never reaches
	// here; "sr" (no script) has no base entry and stays unresolved, which is
	// correct — Serbian requires a script.
	if base, conf := t.Base(); conf != language.No {
		if src, ok := langSources[base.String()]; ok {
			return src, base.String(), true
		}
	}
	return langSource{}, "", false
}

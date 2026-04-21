// dictbuild converts a Unitex/GramLab DELAF dictionary (UTF-16 text) into a
// compact two-layer lookup structure (Lemmas + reverse FormIndex) used by
// gen_i18n at build time to auto-populate the plural/flexion CSVs for the
// i18n pipeline. The companion inspector is cmd/dictlookup.
//
// Usage:
//
//	dictbuild <input.dic> <lang-tag>
//
// Produces <lang-tag>.db in the current working directory. The .db is a gob
// encoding of the Dict type declared below.
//
// Filters applied while reading the DELAF:
//   - entries marked +Pr   (proper names)    → dropped
//   - entries marked +PRO  (enclitic pronoun) → dropped
//   - imperative            (code Y*)         → dropped
//   - finite verbal forms in 1st/2nd person   → dropped
//     (the count-controlled i18n use case only needs 3rd person + infinitive
//     + gerund + participle; see project_i18n_roadmap memory for the reasoning)
//
// Inflection codes are parsed right-to-left into (Class, Genre, Count):
//
//	"ms"  → Class="",  Genre="m", Count="s"
//	"fp"  → Class="",  Genre="f", Count="p"
//	"P3s" → Class="P3", Genre="", Count="s"
//	"Kmp" → Class="K", Genre="m", Count="p"
//	"W"   → Class="W", Genre="", Count=""
package main

import (
	"bufio"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/language"
	"golang.org/x/text/transform"
)

// Dict is the top-level structure persisted to <lang>.db.
//
//   - Lemmas maps a canonical form (for verbs, the infinitive; for
//     nouns/adjectives, the masculine singular or whichever form DELAF lists
//     as lemma) to the set of inflections that lemma produces.
//   - FormIndex is the reverse lookup: an inflected surface form maps to the
//     lemma(s) it could belong to, already decomposed into Class/Genre/Count
//     so gen_i18n can query "same class+genre, different count".
type Dict struct {
	Lemmas    map[string]*Lemma
	FormIndex map[string][]FormRef
}

// FormRef points from an inflected surface form back into Lemmas, with the
// flexion code already decomposed into its three axes.
type FormRef struct {
	Lemma string // key into Dict.Lemmas
	Class string // tense/mood stem, or "" for pure gender/count flexion
	Genre string // "m", "f", "n", or ""
	Count string // "s", "p", or ""
}

// Lemma groups every kept inflection of a lexical entry. The Forms map key is
// the reassembled code (Class+Genre+Count), e.g. "ms", "fp", "P3s", "Kmp".
type Lemma struct {
	Category string // DELAF grammatical code: "N", "V", "A", "ADV", ...
	Forms    map[string]Inflect
}

// Inflect stores the divergence between a lemma and one of its inflected forms
// in a compact "common prefix + suffix" shape. DiffPos is counted in runes, not
// bytes, so that accented characters (common in Portuguese) are handled
// correctly.
type Inflect struct {
	DiffPos int
	Suffix  string
}

var errSkipped = errors.New("filtered")

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: dic2tree <input.dic> <lang-tag>")
		os.Exit(1)
	}
	inputPath := os.Args[1]
	langCode := os.Args[2]

	tag, err := language.Parse(langCode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid language tag %q: %v\n", langCode, err)
		os.Exit(1)
	}
	langCode = tag.String()

	f, err := os.Open(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", inputPath, err)
		os.Exit(1)
	}
	defer f.Close()

	// Decode UTF-16 with BOM detection; BOMOverride lets the file declare its
	// endianness through the initial FF FE / FE FF marker.
	dec := unicode.BOMOverride(unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder())
	reader := transform.NewReader(f, dec)

	dict := &Dict{
		Lemmas:    map[string]*Lemma{},
		FormIndex: map[string][]FormRef{},
	}

	scanner := bufio.NewScanner(reader)
	// DELAF lines can occasionally exceed bufio's default 64KB (mainly via long
	// comments). Give it a generous ceiling.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	var lineNum, kept, skipped, bad int
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "/") {
			continue
		}
		switch err := processLine(line, dict); {
		case err == nil:
			kept++
		case errors.Is(err, errSkipped):
			skipped++
		default:
			bad++
			if bad < 20 {
				fmt.Fprintf(os.Stderr, "line %d: %v\n", lineNum, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "scan: %v\n", err)
		os.Exit(1)
	}

	outPath := langCode + ".db"
	out, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create %s: %v\n", outPath, err)
		os.Exit(1)
	}
	defer out.Close()
	if err := gob.NewEncoder(out).Encode(dict); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("done: %d kept, %d filtered, %d malformed; %d lemmas, %d form entries → %s\n",
		kept, skipped, bad, len(dict.Lemmas), len(dict.FormIndex), outPath)
}

// processLine parses a single DELAF line and inserts its kept flexions into dict.
// Returns errSkipped when the line was filtered out by the +Pr / +PRO / person
// filters, or a descriptive error when the line is malformed.
func processLine(line string, dict *Dict) error {
	inflected, rest, ok := splitUnescaped(line, ',')
	if !ok {
		return fmt.Errorf("missing comma")
	}
	lemmaStr, codes, ok := splitUnescaped(rest, '.')
	if !ok {
		return fmt.Errorf("missing period")
	}
	inflected = unescape(inflected)
	lemmaStr = unescape(lemmaStr)
	if lemmaStr == "" {
		lemmaStr = inflected
	}
	codes = stripComment(codes)

	// codes is "CAT+sem1+sem2...:flex1:flex2:..."
	parts := strings.Split(codes, ":")
	gramSem := parts[0]
	flexCodes := parts[1:]

	gramParts := strings.Split(gramSem, "+")
	if gramParts[0] == "" {
		return fmt.Errorf("missing category")
	}
	category := gramParts[0]
	for _, s := range gramParts[1:] {
		if s == "Pr" || s == "PRO" {
			return errSkipped
		}
	}

	// A DELAF entry with no inflection codes (e.g. an ADV or PREP) still
	// deserves a lemma record: gen_i18n needs to know the word is invariant.
	if len(flexCodes) == 0 {
		flexCodes = []string{""}
	}

	anyKept := false
	for _, code := range flexCodes {
		class, genre, count := parseCode(code)
		class = aliasHomograph(class, count)
		if !keepClass(class) {
			continue
		}
		fullKey := class + genre + count

		lem, ok := dict.Lemmas[lemmaStr]
		if !ok {
			lem = &Lemma{Category: category, Forms: map[string]Inflect{}}
			dict.Lemmas[lemmaStr] = lem
		}
		// First writer wins — duplicates across DELAF lines are silently
		// discarded. The DELAF factorization already guarantees uniqueness
		// when the category and codes match; same-key collisions from
		// different lines only happen with homographs and both resolve to
		// identical inflections anyway.
		if _, exists := lem.Forms[fullKey]; !exists {
			lem.Forms[fullKey] = computeInflect(lemmaStr, inflected)
		}

		newRef := FormRef{Lemma: lemmaStr, Class: class, Genre: genre, Count: count}
		refs := dict.FormIndex[inflected]
		dup := false
		for _, r := range refs {
			if r == newRef {
				dup = true
				break
			}
		}
		if !dup {
			dict.FormIndex[inflected] = append(refs, newRef)
		}
		anyKept = true
	}
	if !anyKept {
		return errSkipped
	}
	return nil
}

// parseCode walks an inflection code right-to-left. The rightmost character is
// tested as a count marker (s/p). If it matched, the next character to its left
// is tested as a gender marker (m/f/n). Whatever remains to the left is the
// "class" — typically a tense/mood stem for verbs or empty for pure
// gender/count flexion on nouns and adjectives.
//
// This function is intentionally small and language-neutral at the axis level;
// the set of valid count/gender characters is hardcoded to what pt_BR uses
// today, but the structure generalizes to other alphabets by extending the
// isCount / isGenre tests.
func parseCode(code string) (class, genre, count string) {
	r := []rune(code)
	n := len(r)
	if n == 0 {
		return
	}
	end := n
	if isCount(r[end-1]) {
		count = string(r[end-1])
		end--
		if end > 0 && isGenre(r[end-1]) {
			genre = string(r[end-1])
			end--
		}
	}
	class = string(r[:end])
	return
}

func isCount(r rune) bool { return r == 's' || r == 'p' }
func isGenre(r rune) bool { return r == 'm' || r == 'f' || r == 'n' }

// aliasHomograph rewrites 1st-person-singular class markers to 3rd-person for
// the PT-BR tenses where the two are orthographic homographs and DELAF_PB
// factorizes the pair under the 1s key. Without this alias the 3s form — the
// one a webdev actually writes in a template meaning "(ele/ela) passava" —
// would be absent from the dict and gen_i18n could not pluralize it.
//
// Tenses aliased (PT-BR specific):
//
//	I  imperfeito indicativo      (ele passava  ≡ eu passava)
//	S  presente subjuntivo        (ele passe    ≡ eu passe)
//	T  imperfeito subjuntivo      (ele passasse ≡ eu passasse)
//	C  condicional / futuro pret. (ele passaria ≡ eu passaria)
//	Q  pretérito mais-que-perfeito (ele passara ≡ eu passara)
//
// F (future) and J (simple past) are NOT aliased: "eu passarei" ≠ "ele passará"
// and "eu passei" ≠ "ele passou". The plural-person case (1p/2p/2s) is never
// aliased — those forms are genuinely distinct and correctly dropped.
func aliasHomograph(class, count string) string {
	if count != "s" || len(class) != 2 || class[1] != '1' {
		return class
	}
	switch class[0] {
	case 'I', 'S', 'T', 'C', 'Q':
		return string(class[0]) + "3"
	}
	return class
}

// keepClass implements the person/mood filter: drop imperatives wholesale, and
// for any person-indexed finite form keep only 3rd person. Non-finite classes
// (W infinitive, G gerund, K participle) and empty class (noun/adj
// gender-count flexion) are always kept.
//
// The rule "any digit in the class marks a person-indexed form" is
// deliberately generic: besides the Table 3.3 tenses (P/I/S/T/F/C/J), the
// DELAF_PB dictionary uses PT-specific tense letters (Q for
// pretérito-mais-que-perfeito, U for infinitivo pessoal, W1..W3* for futuro do
// subjuntivo) that share the same "<letter><person><number>" shape, and any
// future additions should be handled uniformly without touching this code.
func keepClass(class string) bool {
	if class == "" || class == "W" || class == "G" || class == "K" {
		return true
	}
	if strings.HasPrefix(class, "Y") {
		return false
	}
	for _, c := range class {
		if c >= '0' && c <= '9' {
			return c == '3'
		}
	}
	// No digit and no special letter: unknown invariant code, keep defensively.
	return true
}

// computeInflect builds the (DiffPos, Suffix) compact representation for the
// divergence between a lemma and one of its inflected forms. DiffPos is in
// runes to make Portuguese accented characters behave predictably.
func computeInflect(lemma, form string) Inflect {
	lr := []rune(lemma)
	fr := []rune(form)
	i := 0
	for i < len(lr) && i < len(fr) && lr[i] == fr[i] {
		i++
	}
	return Inflect{DiffPos: i, Suffix: string(fr[i:])}
}

// splitUnescaped splits s at the first occurrence of delim that is NOT preceded
// by a backslash. DELAF escapes commas and periods inside fields with \, so a
// naive strings.IndexByte would cut "1\,000" in the wrong place.
func splitUnescaped(s string, delim byte) (head, tail string, ok bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == delim {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// stripComment truncates s at the first unescaped '/'. DELAF allows a trailing
// "/free-form comment" that must be excluded from parsing.
func stripComment(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == '/' {
			return s[:i]
		}
	}
	return s
}

// unescape removes DELAF backslash-escapes so fields contain their literal
// characters. Applied AFTER the structural splitting so \, and \. don't
// confuse the delimiter scanners.
func unescape(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

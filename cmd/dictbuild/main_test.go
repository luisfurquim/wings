package main

import (
	"errors"
	"testing"
)

func TestResolveLangSource(t *testing.T) {
	cases := []struct {
		name     string
		tag      string
		wantKey  string
		wantOK   bool
		wantBase string // expected langSource.Base, checked only when wantOK
	}{
		// Exact registry keys.
		{"exact pt-BR", "pt-BR", "pt-BR", true, "DELAF_PB_2018"},
		{"exact de", "de", "de", true, "dela"},
		// BCP-47 case/format variants must canonicalise to the registry key
		// (the documented contract: "PT-BR" → "pt-BR").
		{"uppercase region", "PT-BR", "pt-BR", true, "DELAF_PB_2018"},
		{"all lowercase", "pt-br", "pt-BR", true, "DELAF_PB_2018"},
		{"mixed case en", "EN", "en", true, "dela-en-public"},
		// Non-standard ISO 639-3 code resolves via the raw-key path, never
		// canonicalised away.
		{"iso639-3 oge", "oge", "oge", true, "Georgian (Ancient)_may2009"},
		// Unknown / unsupported locales.
		{"unknown", "xx-YY", "", false, ""},
		{"empty", "", "", false, ""},
		{"known base, unlisted region", "de-DE", "", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src, key, ok := resolveLangSource(c.tag)
			if ok != c.wantOK || key != c.wantKey {
				t.Fatalf("resolveLangSource(%q) = (%+v, %q, %v), want key=%q ok=%v",
					c.tag, src, key, ok, c.wantKey, c.wantOK)
			}
			if c.wantOK && src.Base != c.wantBase {
				t.Errorf("resolveLangSource(%q).Base = %q, want %q", c.tag, src.Base, c.wantBase)
			}
		})
	}
}

func TestParseCode(t *testing.T) {
	cases := []struct {
		code                string
		class, genre, count string
	}{
		{"", "", "", ""},
		{"ms", "", "m", "s"},   // masculine singular noun/adj
		{"fp", "", "f", "p"},   // feminine plural
		{"P3s", "P3", "", "s"}, // present 3rd-person singular verb
		{"I1s", "I1", "", "s"}, // imperfeito 1st-person (aliased elsewhere)
		{"s", "", "", "s"},     // bare count, no gender
		{"K", "K", "", ""},     // participle, no count
		{"ns", "", "n", "s"},   // neuter singular
		{"W", "W", "", ""},     // infinitive
		{"mp", "", "m", "p"},   // gender precedes count
	}
	for _, c := range cases {
		class, genre, count := parseCode(c.code)
		if class != c.class || genre != c.genre || count != c.count {
			t.Errorf("parseCode(%q) = (%q,%q,%q), want (%q,%q,%q)",
				c.code, class, genre, count, c.class, c.genre, c.count)
		}
	}
}

func TestIsCountIsGenre(t *testing.T) {
	for _, r := range "sp" {
		if !isCount(r) {
			t.Errorf("isCount(%q) = false, want true", r)
		}
	}
	for _, r := range "mfn" {
		if !isGenre(r) {
			t.Errorf("isGenre(%q) = false, want true", r)
		}
	}
	for _, r := range "xPK3" {
		if isCount(r) {
			t.Errorf("isCount(%q) = true, want false", r)
		}
		if isGenre(r) {
			t.Errorf("isGenre(%q) = true, want false", r)
		}
	}
}

func TestAliasHomograph(t *testing.T) {
	cases := []struct {
		class, count string
		want         string
	}{
		// 1s homograph tenses rewritten to 3s.
		{"I1", "s", "I3"},
		{"S1", "s", "S3"},
		{"T1", "s", "T3"},
		{"C1", "s", "C3"},
		{"Q1", "s", "Q3"},
		// Future/simple-past 1s NOT aliased.
		{"F1", "s", "F1"},
		{"J1", "s", "J1"},
		// Already 3rd person: untouched.
		{"I3", "s", "I3"},
		// Plural count never aliased even for homograph tenses.
		{"I1", "p", "I1"},
		// Empty / noun classes pass through.
		{"", "s", ""},
		{"P", "s", "P"},
	}
	for _, c := range cases {
		if got := aliasHomograph(c.class, c.count); got != c.want {
			t.Errorf("aliasHomograph(%q,%q) = %q, want %q",
				c.class, c.count, got, c.want)
		}
	}
}

func TestKeepClass(t *testing.T) {
	keep := []string{"", "W", "G", "K", "P3", "I3", "unknownInvariant"}
	drop := []string{"Y", "Y1", "P1", "P2", "I1", "S2"}
	for _, c := range keep {
		if !keepClass(c) {
			t.Errorf("keepClass(%q) = false, want true (kept)", c)
		}
	}
	for _, c := range drop {
		if keepClass(c) {
			t.Errorf("keepClass(%q) = true, want false (dropped)", c)
		}
	}
}

func TestComputeInflect(t *testing.T) {
	cases := []struct {
		lemma, form string
		wantPos     int
		wantSuffix  string
	}{
		{"passar", "passou", 4, "ou"},
		{"passar", "passar", 6, ""}, // identical → full common prefix, empty suffix
		{"ir", "vou", 0, "vou"},     // no common prefix
		{"café", "cafezinho", 3, "ezinho"},
	}
	for _, c := range cases {
		inf := computeInflect(c.lemma, c.form)
		if inf.DiffPos != c.wantPos || inf.Suffix != c.wantSuffix {
			t.Errorf("computeInflect(%q,%q) = {%d,%q}, want {%d,%q}",
				c.lemma, c.form, inf.DiffPos, inf.Suffix, c.wantPos, c.wantSuffix)
		}
	}
}

// computeInflect followed by reconstruction must be lossless. (reconstruct
// lives in dictlookup; we inline the equivalent here to keep the package
// self-contained.)
func TestComputeInflectRoundTrip(t *testing.T) {
	pairs := [][2]string{
		{"passar", "passou"},
		{"café", "cafezinho"},
		{"ir", "vou"},
	}
	for _, p := range pairs {
		inf := computeInflect(p[0], p[1])
		lr := []rune(p[0])
		got := string(lr[:inf.DiffPos]) + inf.Suffix
		if got != p[1] {
			t.Errorf("round-trip %q→%q = %q", p[0], p[1], got)
		}
	}
}

func TestSplitUnescaped(t *testing.T) {
	cases := []struct {
		in         string
		delim      byte
		head, tail string
		ok         bool
	}{
		{"a,b", ',', "a", "b", true},
		{"abc", ',', "abc", "", false},
		{`1\,000,rest`, ',', `1\,000`, "rest", true}, // escaped comma skipped
		{`only\,escaped`, ',', `only\,escaped`, "", false},
		{"a.b.c", '.', "a", "b.c", true}, // splits at first only
		{"", ',', "", "", false},
		{`\,`, ',', `\,`, "", false}, // lone escaped delim at end, no real delim
	}
	for _, c := range cases {
		head, tail, ok := splitUnescaped(c.in, c.delim)
		if head != c.head || tail != c.tail || ok != c.ok {
			t.Errorf("splitUnescaped(%q,%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.in, c.delim, head, tail, ok, c.head, c.tail, c.ok)
		}
	}
}

func TestStripComment(t *testing.T) {
	cases := []struct{ in, want string }{
		{"code", "code"},
		{"code/comment", "code"},
		{`code\/notcomment`, `code\/notcomment`},
		{`a\/b/real`, `a\/b`},
		{"/leading", ""},
	}
	for _, c := range cases {
		if got := stripComment(c.in); got != c.want {
			t.Errorf("stripComment(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUnescape(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{`a\,b`, "a,b"},
		{`a\.b\.c`, "a.b.c"},
		{`\\`, `\`},
		{`trailing\`, `trailing\`}, // lone trailing backslash kept verbatim
	}
	for _, c := range cases {
		if got := unescape(c.in); got != c.want {
			t.Errorf("unescape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// processLine is the heart of the DELAF parser: it splits a line into
// inflected/lemma/codes, applies the person & homograph filters, and populates
// both the Lemmas and FormIndex maps.
func TestProcessLine(t *testing.T) {
	newDict := func() *Dict {
		return &Dict{Lemmas: map[string]*Lemma{}, FormIndex: map[string][]FormRef{}}
	}

	t.Run("noun gender count", func(t *testing.T) {
		d := newDict()
		// alunas is feminine plural of aluno (category N).
		if err := processLine("alunas,aluno.N+z1:fp", d); err != nil {
			t.Fatalf("processLine: %v", err)
		}
		lem, ok := d.Lemmas["aluno"]
		if !ok {
			t.Fatal("lemma 'aluno' not recorded")
		}
		if lem.Category != "N" {
			t.Errorf("category = %q, want N", lem.Category)
		}
		if _, ok := lem.Forms["fp"]; !ok {
			t.Errorf("form key 'fp' missing, got keys %v", keys(lem.Forms))
		}
		refs := d.FormIndex["alunas"]
		if len(refs) != 1 || refs[0].Lemma != "aluno" || refs[0].Genre != "f" || refs[0].Count != "p" {
			t.Errorf("FormIndex[alunas] = %+v", refs)
		}
	})

	t.Run("empty lemma defaults to inflected", func(t *testing.T) {
		d := newDict()
		// Empty lemma field: the inflected form is its own lemma.
		if err := processLine("rapido,.A:ms", d); err != nil {
			t.Fatalf("processLine: %v", err)
		}
		if _, ok := d.Lemmas["rapido"]; !ok {
			t.Errorf("expected lemma to default to inflected form 'rapido', got %v", keysLemmas(d.Lemmas))
		}
	})

	t.Run("verb keeps only 3rd person", func(t *testing.T) {
		d := newDict()
		// Mixed flex codes: P1s (1st, dropped) and P3s (3rd, kept).
		if err := processLine("passa,passar.V:P1s:P3s", d); err != nil {
			t.Fatalf("processLine: %v", err)
		}
		lem := d.Lemmas["passar"]
		if lem == nil {
			t.Fatal("lemma 'passar' missing")
		}
		if _, ok := lem.Forms["P3s"]; !ok {
			t.Errorf("P3s should be kept, got %v", keys(lem.Forms))
		}
		if _, ok := lem.Forms["P1s"]; ok {
			t.Errorf("P1s should be dropped, got %v", keys(lem.Forms))
		}
	})

	t.Run("homograph alias 1s to 3s", func(t *testing.T) {
		d := newDict()
		// I1s is imperfeito 1st-person, a homograph aliased to I3.
		if err := processLine("passava,passar.V:I1s", d); err != nil {
			t.Fatalf("processLine: %v", err)
		}
		lem := d.Lemmas["passar"]
		if _, ok := lem.Forms["I3s"]; !ok {
			t.Errorf("I1s should be aliased to I3s, got %v", keys(lem.Forms))
		}
	})

	t.Run("proper noun skipped", func(t *testing.T) {
		d := newDict()
		err := processLine("Brasil,Brasil.N+Pr:ms", d)
		if !errors.Is(err, errSkipped) {
			t.Errorf("proper noun (+Pr): err = %v, want errSkipped", err)
		}
		if len(d.Lemmas) != 0 {
			t.Errorf("no lemma should be recorded for skipped line, got %v", keysLemmas(d.Lemmas))
		}
	})

	t.Run("all forms filtered out", func(t *testing.T) {
		d := newDict()
		// Only a 1st-person finite verb form: nothing kept → errSkipped.
		err := processLine("passo,passar.V:P1s", d)
		if !errors.Is(err, errSkipped) {
			t.Errorf("err = %v, want errSkipped", err)
		}
	})

	t.Run("invariant word with no flex codes", func(t *testing.T) {
		d := newDict()
		// Adverb: no flex codes after the category.
		if err := processLine("rapidamente,.ADV", d); err != nil {
			t.Fatalf("processLine: %v", err)
		}
		lem := d.Lemmas["rapidamente"]
		if lem == nil {
			t.Fatal("invariant lemma missing")
		}
		if _, ok := lem.Forms[""]; !ok {
			t.Errorf("invariant should record bare ('') form, got %v", keys(lem.Forms))
		}
	})

	t.Run("malformed lines", func(t *testing.T) {
		d := newDict()
		if err := processLine("nocomma", d); err == nil {
			t.Error("missing comma: expected error")
		}
		if err := processLine("a,noperiod", d); err == nil {
			t.Error("missing period: expected error")
		}
		if err := processLine("a,b.:ms", d); err == nil {
			t.Error("missing category: expected error")
		}
	})

	t.Run("escaped comma in field", func(t *testing.T) {
		d := newDict()
		// The inflected form literally contains a comma, escaped as \,
		if err := processLine(`1\,5,1\,5.N:ms`, d); err != nil {
			t.Fatalf("processLine: %v", err)
		}
		if _, ok := d.FormIndex["1,5"]; !ok {
			t.Errorf("escaped comma should unescape to '1,5', got index keys %v", keysIndex(d.FormIndex))
		}
	})

	t.Run("duplicate form is deduplicated", func(t *testing.T) {
		d := newDict()
		_ = processLine("alunas,aluno.N:fp", d)
		_ = processLine("alunas,aluno.N:fp", d)
		if got := len(d.FormIndex["alunas"]); got != 1 {
			t.Errorf("duplicate FormRef should be deduplicated, got %d refs", got)
		}
	})
}

// --- small helpers for deterministic-free assertions ---

func keys(m map[string]Inflect) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func keysLemmas(m map[string]*Lemma) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func keysIndex(m map[string][]FormRef) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

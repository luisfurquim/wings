// dictlookup inspects a <lang>.db produced by cmd/dic2tree. It prints every
// FormIndex hit for the queried word, resolves each hit to its parent Lemma,
// and reconstructs the surface form of every kept inflection so the output is
// human-readable without any post-processing.
//
// Usage:
//
//	dictlookup <file.db> <word>
//
// Example:
//
//	dictlookup pt-BR.db passou
package main

import (
	"encoding/gob"
	"fmt"
	"os"
	"sort"
)

// NOTE: Dict / Lemma / FormRef / Inflect MUST stay in sync with
// cmd/dic2tree.go. Gob decodes structs by field name/type, so as long as the
// shapes match these duplicated declarations work regardless of package path.

type Dict struct {
	Lemmas    map[string]*Lemma
	FormIndex map[string][]FormRef
}

type FormRef struct {
	Lemma string
	Class string
	Genre string
	Count string
}

type Lemma struct {
	Category string
	Forms    map[string]Inflect
}

type Inflect struct {
	DiffPos int
	Suffix  string
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: dictlookup <file.db> <word>")
		os.Exit(1)
	}
	path := os.Args[1]
	word := os.Args[2]

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", path, err)
		os.Exit(1)
	}
	defer f.Close()

	var dict Dict
	if err := gob.NewDecoder(f).Decode(&dict); err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("loaded %s: %d lemmas, %d form entries\n\n",
		path, len(dict.Lemmas), len(dict.FormIndex))

	found := false

	if refs, ok := dict.FormIndex[word]; ok {
		found = true
		fmt.Printf("FormIndex[%q] → %d reference(s):\n", word, len(refs))
		for i, r := range refs {
			fmt.Printf("  [%d] Lemma=%q Class=%q Genre=%q Count=%q\n",
				i, r.Lemma, r.Class, r.Genre, r.Count)
			if lem, ok := dict.Lemmas[r.Lemma]; ok {
				printLemma("      ", r.Lemma, lem)
			}
		}
		fmt.Println()
	}

	if lem, ok := dict.Lemmas[word]; ok {
		// Avoid re-printing if we already displayed this lemma via a FormRef above
		// whose lemma happens to equal the queried word.
		alreadyShown := false
		if refs, ok := dict.FormIndex[word]; ok {
			for _, r := range refs {
				if r.Lemma == word {
					alreadyShown = true
					break
				}
			}
		}
		if !alreadyShown {
			found = true
			fmt.Printf("Lemmas[%q] (direct):\n", word)
			printLemma("  ", word, lem)
			fmt.Println()
		}
	}

	if !found {
		fmt.Printf("no entries for %q\n", word)
		os.Exit(2)
	}
}

// printLemma dumps a single Lemma's Forms map in deterministic order and
// reconstructs the surface inflection next to the compact (DiffPos, Suffix)
// representation so the output is readable at a glance.
func printLemma(indent, lemma string, lem *Lemma) {
	fmt.Printf("%sCategory=%q, %d form(s):\n", indent, lem.Category, len(lem.Forms))
	keys := make([]string, 0, len(lem.Forms))
	for k := range lem.Forms {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		inf := lem.Forms[k]
		surface := reconstruct(lemma, inf)
		label := k
		if label == "" {
			label = "(bare)"
		}
		fmt.Printf("%s  %-6s → %-20s  [DiffPos=%d Suffix=%q]\n",
			indent, label, surface, inf.DiffPos, inf.Suffix)
	}
}

// reconstruct rebuilds an inflected surface form from its lemma and the
// compact (DiffPos, Suffix) representation stored in the .db. DiffPos is in
// runes, so slicing goes through []rune rather than byte slicing.
func reconstruct(lemma string, inf Inflect) string {
	lr := []rune(lemma)
	pos := inf.DiffPos
	if pos > len(lr) {
		pos = len(lr)
	}
	return string(lr[:pos]) + inf.Suffix
}

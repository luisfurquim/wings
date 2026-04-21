package main

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/csv"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/text/language"

	"github.com/luisfurquim/wprana/wi18n"
)

// Node is a trie node for octal-keyed strings.
// Next contains indices into the arena slice (0 = no child).
// Data holds the index into txt (-1 = no data).
type Node struct {
	Next [8]int32
	Data int32
}

var txt []string

// arena is the contiguous backing store for all trie nodes.
// Index 0 is the root; child index 0 means "empty" since no node
// ever points back to the root.
var arena = []Node{{Data: -1}}

// Occurrence records one location where a translatable text was found in
// the HTML source. occurrences[i] holds every appearance of txt[i] in the
// order they were encountered during the directory walk.
type Occurrence struct {
	Path string // relative to --path, forward slashes
	Line int    // 1-based
	Col  int    // 1-based
	Tag  string // nearest HTML ancestor element
}

var occurrences = map[int32][]Occurrence{}

// validateLangTag parses the tag as a BCP 47 language tag.
// Returns the canonical form if valid, or "en-US" as fallback.
func validateLangTag(tag string) string {
	if tag != "" {
		if t, err := language.Parse(tag); err == nil {
			return t.String()
		}
	}
	return "en-US"
}

func main() {
	pathFlag := flag.String("path", "", "root directory to traverse")
	deflangFlag := flag.String("deflang", "", "default language tag (e.g. pt-BR, en-US, de-DE)")
	attrsFlag := flag.String("attrs", "", "comma-separated HTML attributes whose values are translated; overrides the default list")
	addAttrsFlag := flag.String("add-attrs", "", "comma-separated attributes to add to the default list")
	noAttrsFlag := flag.String("no-attrs", "", "comma-separated attributes to remove from the default list")
	flag.Parse()

	if *pathFlag == "" {
		fmt.Fprintln(os.Stderr, "usage: gen_i18n --path <directory> [-deflang <lang>] [-attrs <list>] [-add-attrs <list>] [-no-attrs <list>]")
		os.Exit(1)
	}

	rootDir := *pathFlag
	defLang := validateLangTag(*deflangFlag)
	attrSet := buildAttrSet(*attrsFlag, *addAttrsFlag, *noAttrsFlag)

	// Capture the current epoch once for the entire run.
	version := time.Now().Unix()

	// Ensure the i18n output directory exists.
	i18nDir := filepath.Join(rootDir, "i18n")
	if err := os.MkdirAll(i18nDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating i18n dir: %v\n", err)
		os.Exit(1)
	}

	// Map hash → original text (used for collision detection during processing).
	dbMap := map[string]string{}

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".html") {
			return nil
		}
		// Skip files we already generated.
		if strings.HasSuffix(strings.ToLower(d.Name()), ".i18n.html") {
			return nil
		}
		relPath := filepath.ToSlash(mustRel(rootDir, path))
		return processHTMLFile(path, relPath, dbMap, attrSet)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error walking directory: %v\n", err)
		os.Exit(1)
	}

	// Opportunistically convert any legacy <lang>.csv files to <lang>.json so
	// the remapping step below can treat them uniformly.
	if err := migrateCSVToJSON(i18nDir); err != nil {
		fmt.Fprintf(os.Stderr, "error migrating csv→json: %v\n", err)
		os.Exit(1)
	}

	// Load old deflang to preserve Revised flags and to drive translation
	// remapping for other languages.
	oldDefPath := filepath.Join(i18nDir, defLang+".json")
	oldDef, err := loadJSON(oldDefPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading old deflang json: %v\n", err)
		os.Exit(1)
	}
	oldSourceToIdx := map[string]int{}
	for i, e := range oldDef {
		if _, seen := oldSourceToIdx[e.Content]; !seen {
			oldSourceToIdx[e.Content] = i
		}
	}

	// Build and save the new deflang catalog.
	defEntries := buildEntries(txt, func(i int) (string, bool) {
		if oldIdx, ok := oldSourceToIdx[txt[i]]; ok && oldIdx < len(oldDef) {
			return "", oldDef[oldIdx].Revised
		}
		return "", false
	}, func(i int) string {
		// Default language: Content is the source string itself.
		return txt[i]
	})
	if err := saveJSON(oldDefPath, defEntries); err != nil {
		fmt.Fprintf(os.Stderr, "error saving deflang json: %v\n", err)
		os.Exit(1)
	}

	// Remap every other <lang>.json file (except the default) to the new
	// index order, preserving translations whose source string is still
	// present in the catalog.
	langFiles, err := filepath.Glob(filepath.Join(i18nDir, "*.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing languages: %v\n", err)
		os.Exit(1)
	}
	for _, langFile := range langFiles {
		base := filepath.Base(langFile)
		lang := strings.TrimSuffix(base, ".json")
		if lang == defLang || strings.Contains(base, ".inflections.") {
			continue
		}
		oldLang, err := loadJSON(langFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading %s: %v\n", langFile, err)
			os.Exit(1)
		}
		langEntries := buildEntries(txt, func(i int) (content string, revised bool) {
			oldIdx, ok := oldSourceToIdx[txt[i]]
			if !ok || oldIdx >= len(oldLang) {
				return "", false
			}
			return oldLang[oldIdx].Content, oldLang[oldIdx].Revised
		}, nil)
		if err := saveJSON(langFile, langEntries); err != nil {
			fmt.Fprintf(os.Stderr, "error saving %s: %v\n", langFile, err)
			os.Exit(1)
		}
	}

	// Save the current tree to i18n.db.
	dbPath := filepath.Join(rootDir, "i18n.db")
	if err := saveDB(dbPath, version); err != nil {
		fmt.Fprintf(os.Stderr, "error saving db: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("done: %d entries, version %d\n", len(dbMap), version)
	fmt.Printf("Arena: %d nodes\n", len(arena))
	fmt.Printf("Txt: %d entries\n", len(txt))
}

// buildEntries assembles the [wi18n.Entry] slice for one language.
//
// carry returns (Content, Revised) for entry i from the previous run. For
// the default language, the caller should also pass a contentOverride that
// forces Content = txt[i] (the source string), since the default-language
// catalog always holds the sources; carry's Content is discarded in that
// case. For translated languages, contentOverride is nil.
func buildEntries(
	src []string,
	carry func(i int) (content string, revised bool),
	contentOverride func(i int) string,
) []wi18n.Entry {
	out := make([]wi18n.Entry, len(src))
	for i := range src {
		content, revised := "", false
		if carry != nil {
			content, revised = carry(i)
		}
		if contentOverride != nil {
			content = contentOverride(i)
		}
		out[i] = wi18n.Entry{
			Content:   content,
			Revised:   revised,
			Context:   formatFirstContext(int32(i)),
			Ctxdetail: formatCtxdetail(int32(i)),
		}
	}
	return out
}

func formatFirstContext(i int32) string {
	occs := occurrences[i]
	if len(occs) == 0 {
		return ""
	}
	o := occs[0]
	return fmt.Sprintf("%s:%d:%d", o.Path, o.Line, o.Col)
}

func formatCtxdetail(i int32) string {
	occs := occurrences[i]
	if len(occs) == 0 {
		return ""
	}
	parts := make([]string, len(occs))
	for j, o := range occs {
		parts[j] = fmt.Sprintf("%s@%s:%d:%d", o.Tag, o.Path, o.Line, o.Col)
	}
	return strings.Join(parts, "<br/>")
}

func mustRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

// loadJSON reads a <lang>.json catalog. Returns nil if the file does not
// exist.
func loadJSON(path string) ([]wi18n.Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var entries []wi18n.Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return entries, nil
}

// saveJSON writes entries as an indented JSON array. HTML escaping is
// disabled so the <br/> separator inside Ctxdetail stays human-readable in
// the on-disk file (JSON decoders handle both forms identically).
func saveJSON(path string, entries []wi18n.Entry) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// migrateCSVToJSON converts every <lang>.csv in i18nDir to <lang>.json when
// the JSON file does not exist yet. The CSV is read with the same rules as
// the legacy loader: rows may start with a '!' marker which is stripped.
// Each row becomes a wi18n.Entry with only the Content field populated.
func migrateCSVToJSON(i18nDir string) error {
	csvFiles, err := filepath.Glob(filepath.Join(i18nDir, "*.csv"))
	if err != nil {
		return err
	}
	for _, csvPath := range csvFiles {
		jsonPath := strings.TrimSuffix(csvPath, ".csv") + ".json"
		if _, err := os.Stat(jsonPath); err == nil {
			continue
		}
		rows, err := loadLegacyCSV(csvPath)
		if err != nil {
			return fmt.Errorf("migrate %s: %w", csvPath, err)
		}
		entries := make([]wi18n.Entry, len(rows))
		for i, s := range rows {
			entries[i] = wi18n.Entry{Content: s}
		}
		if err := saveJSON(jsonPath, entries); err != nil {
			return fmt.Errorf("write %s: %w", jsonPath, err)
		}
		fmt.Printf("migrated: %s → %s\n", csvPath, jsonPath)
	}
	return nil
}

func loadLegacyCSV(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "!") {
			lines[i] = line[1:]
		}
	}
	cleaned := strings.Join(lines, "\n")
	records, err := csv.NewReader(strings.NewReader(cleaned)).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}
	result := make([]string, len(records))
	for i, rec := range records {
		if len(rec) == 0 {
			continue
		}
		result[i] = unescapeControl(rec[0])
	}
	return result, nil
}

// loadDB reads the previous version and arena from the db file.
// Returns zero version and nil arena if the file does not exist.
func loadDB(path string) (int64, []Node, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, nil
		}
		return 0, nil, err
	}
	defer f.Close()

	var version int64
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		return 0, nil, fmt.Errorf("read version: %w", err)
	}

	var oldArena []Node
	if err := gob.NewDecoder(f).Decode(&oldArena); err != nil {
		return 0, nil, fmt.Errorf("decode arena: %w", err)
	}

	return version, oldArena, nil
}

// saveDB writes the current version and arena to the db file.
func saveDB(path string, version int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := binary.Write(f, binary.LittleEndian, version); err != nil {
		return fmt.Errorf("write version: %w", err)
	}

	if err := gob.NewEncoder(f).Encode(arena); err != nil {
		return fmt.Errorf("encode arena: %w", err)
	}

	return nil
}

// posTracker walks an HTML source buffer in forward-only order, returning
// the 1-based line and column where a given text first appears at or after
// the current cursor. Used to attach source positions to parser-tree text
// nodes, which html.Parse does not expose directly.
type posTracker struct {
	src    []byte
	cursor int
}

func newPosTracker(src []byte) *posTracker {
	return &posTracker{src: src}
}

// find advances the cursor to the first occurrence of text at or after the
// current position and returns the 1-based (line, col) of that occurrence.
// Returns (0, 0) if the text cannot be located — which should not happen
// for text that came from html.Parse of the same source, but may occur for
// strings containing decoded HTML entities.
func (p *posTracker) find(text string) (line, col int) {
	idx := bytes.Index(p.src[p.cursor:], []byte(text))
	if idx < 0 {
		return 0, 0
	}
	absIdx := p.cursor + idx
	line, col = 1, 1
	for i := 0; i < absIdx; i++ {
		if p.src[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	p.cursor = absIdx + len(text)
	return
}

// processHTMLFile parses an HTML file, replaces TextNode contents with
// txt indices, writes the .i18n.html output, and populates dbMap.
func processHTMLFile(path, relPath string, dbMap map[string]string, attrSet map[string]bool) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	doc, err := html.Parse(bytes.NewReader(src))
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	tracker := newPosTracker(src)
	replaceTextNodes(doc, dbMap, relPath, tracker, attrSet)

	// Build output path: dir/name.html → dir/name.i18n.html
	ext := filepath.Ext(path)
	outPath := path[:len(path)-len(ext)] + ".i18n" + ext

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer out.Close()

	if err := html.Render(out, doc); err != nil {
		return fmt.Errorf("render %s: %w", outPath, err)
	}

	fmt.Printf("processed: %s → %s\n", path, outPath)
	return nil
}

// replaceTextNodes walks the HTML tree and replaces every TextNode's
// content with the txt index (as a decimal string), recording the original
// in dbMap and appending a source Occurrence to the per-entry list. For
// ElementNodes, it also translates attribute values whose attribute name is
// in attrSet (typically title/placeholder/alt/aria-label).
func replaceTextNodes(n *html.Node, dbMap map[string]string, relPath string, tracker *posTracker, attrSet map[string]bool) {
	if n.Type == html.TextNode {
		// Skip text nodes inside <style> and <script> tags.
		if n.Parent != nil && n.Parent.Type == html.ElementNode && (n.Parent.Data == "style" || n.Parent.Data == "script") {
			return
		}
		original := n.Data
		// Skip whitespace-only text nodes.
		if isBlank(original) {
			return
		}
		txtIdx := resolveHash(original, dbMap)

		line, col := tracker.find(original)
		tag := ""
		if n.Parent != nil && n.Parent.Type == html.ElementNode {
			tag = n.Parent.Data
		}
		occurrences[txtIdx] = append(occurrences[txtIdx], Occurrence{
			Path: relPath,
			Line: line,
			Col:  col,
			Tag:  tag,
		})

		n.Data = fmt.Sprintf("%d", txtIdx)
		return
	}

	if n.Type == html.ElementNode && len(attrSet) > 0 {
		for i := range n.Attr {
			a := &n.Attr[i]
			if !attrSet[strings.ToLower(a.Key)] {
				continue
			}
			if a.Val == "" || isBlank(a.Val) {
				continue
			}
			txtIdx := resolveHash(a.Val, dbMap)
			line, col := tracker.find(a.Val)
			occurrences[txtIdx] = append(occurrences[txtIdx], Occurrence{
				Path: relPath,
				Line: line,
				Col:  col,
				Tag:  n.Data + "[" + a.Key + "]",
			})
			a.Val = fmt.Sprintf("%d", txtIdx)
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		replaceTextNodes(c, dbMap, relPath, tracker, attrSet)
	}
}

// defaultTranslatableAttrs is the set of HTML attributes whose values are
// extracted by default. Must stay in sync with wprana.TranslatableAttrs.
var defaultTranslatableAttrs = []string{"title", "placeholder", "alt", "aria-label"}

// buildAttrSet resolves the three flags into a lowercase attribute set.
// --attrs overrides the default list entirely; --add-attrs appends to
// whichever base list is active (default or override); --no-attrs removes
// from it afterwards.
func buildAttrSet(override, add, remove string) map[string]bool {
	out := map[string]bool{}
	base := defaultTranslatableAttrs
	if strings.TrimSpace(override) != "" {
		base = splitList(override)
	}
	for _, a := range base {
		out[strings.ToLower(a)] = true
	}
	for _, a := range splitList(add) {
		out[strings.ToLower(a)] = true
	}
	for _, a := range splitList(remove) {
		delete(out, strings.ToLower(a))
	}
	return out
}

func splitList(list string) []string {
	var out []string
	for _, part := range strings.Split(list, ",") {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// resolveHash computes the MD5-based key for text, handling collisions.
// It uses the left half of the octal hash as the base key. On collision,
// it appends one character at a time from the right half of the octal hash.
// Returns the index into the txt slice where the text is stored.
func resolveHash(text string, dbMap map[string]string) int32 {
	full := computeMD5(text)
	half := len(full) / 2
	base := full[:half]
	rest := full[half:]

	// Try the bare left-half hash first.
	if existing, ok := dbMap[base]; !ok {
		dbMap[base] = text
		return treeSet(base, text)
	} else if existing == text {
		return treeGet(base)
	}

	// Collision: append characters from the right half one at a time.
	for i := 1; i <= len(rest); i++ {
		candidate := base + rest[:i]
		if existing, ok := dbMap[candidate]; !ok {
			dbMap[candidate] = text
			return treeSet(candidate, text)
		} else if existing == text {
			return treeGet(candidate)
		}
	}

	// Extremely unlikely: exhausted all right-half characters and still colliding.
	// Fall back to appending a numeric suffix to the full hash.
	for i := 0; ; i++ {
		candidate := fmt.Sprintf("%s%d", full, i)
		if existing, ok := dbMap[candidate]; !ok {
			dbMap[candidate] = text
			return treeSet(candidate, text)
		} else if existing == text {
			return treeGet(candidate)
		}
	}
}

// computeMD5 returns the octal-encoded MD5 of s.
func computeMD5(s string) string {
	h := md5.Sum([]byte(s))
	var b strings.Builder
	for _, v := range h {
		fmt.Fprintf(&b, "%o", v)
	}
	return b.String()
}

// unescapeControl reverses the legacy gen_i18n CSV escaping rules. Only
// called when converting old *.csv files to *.json during migration.
func unescapeControl(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\':
				b.WriteByte('\\')
				i += 2
			case 'n':
				b.WriteByte('\n')
				i += 2
			case 'r':
				b.WriteByte('\r')
				i += 2
			case 't':
				b.WriteByte('\t')
				i += 2
			case 'x':
				if i+3 < len(s) {
					var val byte
					if _, err := fmt.Sscanf(s[i+2:i+4], "%02x", &val); err == nil {
						b.WriteByte(val)
						i += 4
						continue
					}
				}
				b.WriteByte(s[i])
				i++
			default:
				b.WriteByte(s[i])
				i++
			}
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// isBlank returns true if s contains only whitespace.
func isBlank(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// treeSet inserts a key-value pair into the arena-backed trie.
// The string is appended to txt and the txt index is stored in the node's Data field.
// Returns the txt index where the string was stored.
func treeSet(key string, val string) int32 {
	idx := int32(0) // start at root
	for _, ch := range key {
		ci := ch - '0'
		if arena[idx].Next[ci] == 0 {
			arena = append(arena, Node{Data: -1})
			arena[idx].Next[ci] = int32(len(arena) - 1)
		}
		idx = arena[idx].Next[ci]
	}
	arena[idx].Data = int32(len(txt))
	txt = append(txt, val)
	return arena[idx].Data
}

// treeGet retrieves the txt index stored at the given key in the trie.
func treeGet(key string) int32 {
	idx := int32(0)
	for _, ch := range key {
		ci := ch - '0'
		idx = arena[idx].Next[ci]
	}
	return arena[idx].Data
}

// Ensure loadDB stays referenced so future incremental logic can use it.
var _ = loadDB

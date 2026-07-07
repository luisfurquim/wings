package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/md5"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/text/language"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings/wi18n"
	"github.com/luisfurquim/wings/wsign"
)

// G is this binary's goose alert. Default 2 keeps info-level output visible;
// project's wings.json may raise it via wi18n.SetConfig + ConfigureGoose.
var G goose.Alert = goose.Alert(2)

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
	autoFlexFlag := flag.Bool("auto-flex", false, "consult per-language dictionaries to auto-fill empty inflection cells (LGPLLR-derivative output — see README)")
	dictDirFlag := flag.String("dict-dir", "", "directory holding <lang>.db files produced by cmd/dictbuild (default: cmd/gen_i18n/dicts under the wings module)")
	dictStrictFlag := flag.Bool("dict-strict", false, "require an exact-locale dictionary (en-US.db) for -auto-flex; without it, fall back to the base language (en.db). Off by default")
	autoTranslateFlag := flag.Bool("auto-translate", false, "use the configured LLM/MT backend (gen_i18n.json) to pre-fill entries that the dictionary pass could not fill; output is flagged for human review")
	genKeyFlag := flag.Bool("genkey", false, "generate a fresh ed25519 signing keypair (gen_i18n.ed25519.key + gen_i18n.ed25519.pub) and exit")
	genKeyDirFlag := flag.String("genkey-dir", ".", "directory to write the generated keypair when using -genkey")
	signKeyFlag := flag.String("sign-key", "", "path to gen_i18n.ed25519.key; when set, sign every output catalog with .json.sig sidecar files")
	signKeyPassFlag := flag.String("sign-key-password", "", "password for the private key file specified by -sign-key")
	flag.Parse()

	// ── Key generation mode (standalone; does not need -path) ────────────────
	if *genKeyFlag {
		dir := *genKeyDirFlag
		keyFile := dir + "/" + wsign.DefaultKeyFile
		pubFile := dir + "/" + wsign.DefaultPubFile
		pass := *signKeyPassFlag
		if pass == "" {
			G.Fatalf(1, "error: -genkey requires -sign-key-password")
		}
		if err := wsign.GenerateSigningKey(keyFile, pubFile, pass); err != nil {
			G.Fatalf(1, "error generating keypair: %v", err)
		}
		G.Logf(2, "keypair written:")
		G.Logf(2, "  private: %s", keyFile)
		G.Logf(2, "  public:  %s", pubFile)
		G.Logf(2, "Embed the public key in your WASM app:")
		G.Logf(2, "  //go:embed %s", pubFile)
		G.Logf(2, "  var catalogPubKeyPEM []byte")
		G.Logf(2, "  // in main(): wi18n.SetCatalogPublicKey(catalogPubKeyPEM)")
		os.Exit(0)
	}

	if *pathFlag == "" {
		G.Logf(1, "usage: gen_i18n --path <directory> [-deflang <lang>] [-attrs <list>] [-add-attrs <list>] [-no-attrs <list>] [-auto-flex [-dict-dir <dir>]] [-auto-translate] [-sign-key <file> -sign-key-password <pass>]")
		G.Fatalf(1, "       gen_i18n -genkey [-genkey-dir <dir>] -sign-key-password <pass>")
	}

	// ── Load signing key if requested ─────────────────────────────────────────
	var signingKey ed25519.PrivateKey
	if *signKeyFlag != "" {
		if *signKeyPassFlag == "" {
			G.Fatalf(1, "error: -sign-key requires -sign-key-password")
		}
		var err error
		signingKey, err = wsign.LoadSigningKey(*signKeyFlag, *signKeyPassFlag)
		if err != nil {
			G.Fatalf(1, "error loading signing key: %v", err)
		}
	}

	rootDir := *pathFlag

	// Apply project-wide debug settings from wings.json (if present).
	if data, err := os.ReadFile(filepath.Join(rootDir, "wings.json")); err == nil {
		if err := wi18n.SetConfig(data); err != nil {
			G.Logf(1, "wings.json: %v", err)
		}
		wi18n.ConfigureGoose(&G)
	}
	defLang := validateLangTag(*deflangFlag)
	attrSet := buildAttrSet(*attrsFlag, *addAttrsFlag, *noAttrsFlag)
	autoFlex = *autoFlexFlag
	dictStrict = *dictStrictFlag
	dictDir = *dictDirFlag
	if dictDir == "" {
		dictDir = defaultDictDir()
	}
	autoTranslate = *autoTranslateFlag
	if autoTranslate {
		if err := initTranslator(rootDir); err != nil {
			G.Fatalf(1, "error: %v", err)
		}
	}

	// Ensure the i18n output directory exists.
	i18nDir := filepath.Join(rootDir, "i18n")
	if err := os.MkdirAll(i18nDir, 0755); err != nil {
		G.Fatalf(1, "error creating i18n dir: %v", err)
	}

	// Map hash → original text (used for collision detection during processing).
	dbMap := map[string]string{}

	// Pre-pass: register every `=name` flex definition before the rewrite walk,
	// so a `#name` reuse resolves regardless of which file is walked first
	// (names are global per catalog). See collectFlexNames.
	if err := collectFlexNames(rootDir, attrSet); err != nil {
		G.Fatalf(1, "error collecting flex names: %v", err)
	}

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
		G.Fatalf(1, "error walking directory: %v", err)
	}

	// Opportunistically convert any legacy <lang>.csv files to <lang>.json so
	// the remapping step below can treat them uniformly.
	if err := migrateCSVToJSON(i18nDir); err != nil {
		G.Fatalf(1, "error migrating csv→json: %v", err)
	}

	// Load old deflang to preserve Revised flags and to drive translation
	// remapping for other languages.
	oldDefPath := filepath.Join(i18nDir, defLang+".json")
	oldDef, err := loadJSON(oldDefPath)
	if err != nil {
		G.Fatalf(1, "error loading old deflang json: %v", err)
	}
	// Align the new source order against the previous deflang catalog so
	// translations survive edits, insertions, moves, and deletions (see
	// alignSources). Exact matches carry over verbatim (Revised preserved);
	// fuzzy matches reuse the old translation but flag it for re-review.
	oldSrc := make([]string, len(oldDef))
	for i, e := range oldDef {
		oldSrc[i] = e.Content
	}
	align, alignKind := alignSources(oldSrc, txt)

	// Build and save the new deflang catalog.
	defEntries := buildEntries(txt, func(i int) (string, bool) {
		if alignKind[i] == matchExact {
			return "", oldDef[align[i]].Revised
		}
		// New or edited source: needs human review.
		return "", false
	}, func(i int) string {
		// Default language: Content is the source string itself.
		return txt[i]
	})
	if err := saveJSON(oldDefPath, defEntries); err != nil {
		G.Fatalf(1, "error saving deflang json: %v", err)
	}
	if err := maybeSignJSON(signingKey, oldDefPath); err != nil {
		G.Fatalf(1, "error signing deflang json: %v", err)
	}

	// Remap every other <lang>.json file (except the default) to the new
	// index order, preserving translations whose source string is still
	// present in the catalog.
	langFiles, err := filepath.Glob(filepath.Join(i18nDir, "*.json"))
	if err != nil {
		G.Fatalf(1, "error listing languages: %v", err)
	}
	for _, langFile := range langFiles {
		base := filepath.Base(langFile)
		lang := strings.TrimSuffix(base, ".json")
		if lang == defLang || strings.Contains(base, ".inflections.") || strings.HasSuffix(lang, ".meta") {
			continue
		}
		oldLang, err := loadJSON(langFile)
		if err != nil {
			G.Fatalf(1, "error loading %s: %v", langFile, err)
		}
		langEntries := buildEntries(txt, func(i int) (content string, revised bool) {
			oi := align[i]
			if oi < 0 || oi >= len(oldLang) {
				return "", false
			}
			if alignKind[i] == matchFuzzy {
				// Source was edited: reuse the translation but force re-review.
				return oldLang[oi].Content, false
			}
			return oldLang[oi].Content, oldLang[oi].Revised
		}, nil)
		applyTextTranslations(langEntries, defEntries, defLang, lang)
		if err := saveJSON(langFile, langEntries); err != nil {
			G.Fatalf(1, "error saving %s: %v", langFile, err)
		}
		if err := maybeSignJSON(signingKey, langFile); err != nil {
			G.Fatalf(1, "error signing %s: %v", langFile, err)
		}
	}

	// Emit <lang>.inflections.json for every discovered language, including
	// deflang. Translations are remapped across runs by canonical label.
	if err := emitFlexCatalogs(i18nDir, defLang); err != nil {
		G.Fatalf(1, "error emitting flex catalogs: %v", err)
	}
	if err := signSecondaryCatalogs(signingKey, i18nDir); err != nil {
		G.Fatalf(1, "error signing secondary catalogs: %v", err)
	}

	G.Logf(2, "done: %d entries", len(dbMap))
	G.Logf(2, "Arena: %d nodes", len(arena))
	G.Logf(2, "Txt: %d entries", len(txt))
	G.Logf(2, "Flex: %d rules", len(flexBlocks))
}

// signSecondaryCatalogs signs the non-text catalogs the runtime also enforces:
// every emitted <lang>.inflections.json plus any hand-authored <lang>.fmt.json
// found in i18nDir. The runtime requires a valid .sig on every catalog that
// loads while a public key is configured, so leaving these unsigned would make
// it reject its own build output. No-op without a signing key.
func signSecondaryCatalogs(key ed25519.PrivateKey, i18nDir string) error {
	if key == nil {
		return nil
	}
	for _, pattern := range []string{"*.inflections.json", "*.fmt.json"} {
		files, err := filepath.Glob(filepath.Join(i18nDir, pattern))
		if err != nil {
			return err
		}
		for _, f := range files {
			if err := maybeSignJSON(key, f); err != nil {
				return err
			}
		}
	}
	return nil
}

// maybeSignJSON signs jsonFile and writes jsonFile+".sig" if signingKey is set.
func maybeSignJSON(key ed25519.PrivateKey, jsonFile string) error {
	if key == nil {
		return nil
	}
	content, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("reading %s for signing: %w", jsonFile, err)
	}
	return wsign.SignCatalog(key, content, jsonFile+".sig")
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
			EntryData: wi18n.EntryData{
				Content: content,
				Revised: revised,
			},
			EntryMeta: wi18n.EntryMeta{
				Context:   formatFirstContext(int32(i)),
				Ctxdetail: formatCtxdetail(int32(i)),
			},
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

// metaPath derives the sibling meta-file path for a data-file path:
// foo/i18n/pt-BR.json                     → foo/i18n/pt-BR.meta.json
// foo/i18n/pt-BR.inflections.json         → foo/i18n/pt-BR.inflections.meta.json
func metaPath(dataPath string) string {
	return strings.TrimSuffix(dataPath, ".json") + ".meta.json"
}

// loadJSON reads a <lang>.json catalog and its sibling <lang>.meta.json,
// merging them into the in-memory []wi18n.Entry form. Returns nil when the
// data file does not exist.
//
// Handles two on-disk formats transparently:
//   - Split (new): data file has only content/revised; meta file has
//     context/ctxdetail. Both JSON decoders produce populated fields via
//     their respective tags; the merge step copies meta into entry.
//   - Legacy combined (old): data file has all four fields inline; no meta
//     file exists. json.Unmarshal fills everything from the single file and
//     the meta read returns nil, so the entries are already complete.
//
// In both cases the returned slice is indistinguishable. On the next save
// gen_i18n writes the split format, so legacy files migrate on first run.
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
	metas, err := loadEntryMetas(metaPath(path))
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if i < len(metas) {
			entries[i].Context = metas[i].Context
			entries[i].Ctxdetail = metas[i].Ctxdetail
		}
	}
	return entries, nil
}

func loadEntryMetas(path string) ([]wi18n.EntryMeta, error) {
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
	var metas []wi18n.EntryMeta
	if err := json.Unmarshal(data, &metas); err != nil {
		return nil, fmt.Errorf("parse meta json: %w", err)
	}
	return metas, nil
}

// saveJSON writes the data half to path and the meta half to the sibling
// meta path. HTML escaping is disabled in both so "<br/>" separators stay
// human-readable; JSON decoders handle the escaped form identically.
func saveJSON(path string, entries []wi18n.Entry) error {
	datas := make([]wi18n.EntryData, len(entries))
	metas := make([]wi18n.EntryMeta, len(entries))
	for i, e := range entries {
		datas[i] = e.EntryData
		metas[i] = e.EntryMeta
	}
	if err := writeIndentedJSON(path, datas); err != nil {
		return err
	}
	return writeIndentedJSON(metaPath(path), metas)
}

// writeIndentedJSON encodes v as indented JSON with HTML escaping disabled
// and writes it to path.
func writeIndentedJSON(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
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
			entries[i] = wi18n.Entry{EntryData: wi18n.EntryData{Content: s}}
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
	replaceTextNodes(doc, dbMap, relPath, tracker, attrSet, false)

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

// collectFlexNames does a pre-pass over every HTML source, registering each
// `=name` flex definition in flexNameTable. It must run before the main rewrite
// walk so a `#name` reuse can resolve a `=name` defined in any file (names are
// global per catalog) no matter the file-processing order — gen_i18n's main
// walk writes each .i18n.html inline, so it cannot resolve a forward reference
// to a definition in a not-yet-walked file.
func collectFlexNames(rootDir string, attrSet map[string]bool) error {
	return filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if !strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".i18n.html") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		doc, err := html.Parse(bytes.NewReader(src))
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		collectNamesNode(doc, attrSet, false)
		return nil
	})
}

// collectNamesNode mirrors replaceTextNodes' traversal — honoring translate="no"
// subtrees and skipping <style>/<script> — but only scans text nodes and
// translatable attributes for `=name` definitions, leaving the tree untouched.
func collectNamesNode(n *html.Node, attrSet map[string]bool, noTranslate bool) {
	if n.Type == html.ElementNode {
		for _, a := range n.Attr {
			if strings.EqualFold(a.Key, "translate") {
				switch strings.ToLower(strings.TrimSpace(a.Val)) {
				case "no":
					noTranslate = true
				case "yes", "":
					noTranslate = false
				}
				break
			}
		}
	}

	if n.Type == html.TextNode {
		if n.Parent != nil && n.Parent.Type == html.ElementNode && (n.Parent.Data == "style" || n.Parent.Data == "script") {
			return
		}
		if !noTranslate && !isBlank(n.Data) {
			scanFlexNames(n.Data)
		}
		return
	}

	if n.Type == html.ElementNode && len(attrSet) > 0 && !noTranslate {
		for _, a := range n.Attr {
			if attrSet[strings.ToLower(a.Key)] && a.Val != "" && !isBlank(a.Val) {
				scanFlexNames(a.Val)
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectNamesNode(c, attrSet, noTranslate)
	}
}

// replaceTextNodes walks the HTML tree and replaces every TextNode's
// content with the txt index (as a decimal string), recording the original
// in dbMap and appending a source Occurrence to the per-entry list. For
// ElementNodes, it also translates attribute values whose attribute name is
// in attrSet (typically title/placeholder/alt/aria-label).
func replaceTextNodes(n *html.Node, dbMap map[string]string, relPath string, tracker *posTracker, attrSet map[string]bool, noTranslate bool) {
	// Honor the standard HTML `translate` attribute (inherited down the tree):
	// translate="no" excludes this element and its subtree from extraction; a
	// nested translate="yes" re-enables it. This is a build-time concern only —
	// the wi18n runtime still substitutes any numeric index it finds, so a demo
	// can keep literal indices under translate="no" and still render them live
	// (see live-demo i18ntab's table), and verbatim content (e.g. the tabsdemo
	// prose) can opt out of the pt-BR catalog entirely.
	if n.Type == html.ElementNode {
		for _, a := range n.Attr {
			if strings.EqualFold(a.Key, "translate") {
				switch strings.ToLower(strings.TrimSpace(a.Val)) {
				case "no":
					noTranslate = true
				case "yes", "":
					noTranslate = false
				}
				break
			}
		}
	}

	if n.Type == html.TextNode {
		// Skip text nodes inside <style> and <script> tags.
		if n.Parent != nil && n.Parent.Type == html.ElementNode && (n.Parent.Data == "style" || n.Parent.Data == "script") {
			return
		}
		if noTranslate {
			return
		}
		original := n.Data
		// Skip whitespace-only text nodes.
		if isBlank(original) {
			return
		}

		line, col := tracker.find(original)
		tag := ""
		if n.Parent != nil && n.Parent.Type == html.ElementNode {
			tag = n.Parent.Data
		}

		// Rewrite flex blocks (e.g. {{@s %q ~o ~aluno}} → {{@s %q #N}})
		// before hashing, so the txt catalog stores the stable runtime form.
		rewritten, hasFlex := rewriteFlexBlocks(original, func(idx int32) {
			flexOccurrences[idx] = append(flexOccurrences[idx], Occurrence{
				Path: relPath, Line: line, Col: col, Tag: tag,
			})
		})
		// A text node that is nothing but plain {{...}} bindings (no flex block,
		// no words) is a runtime placeholder, not catalog content — leave it
		// verbatim. Indexing it would pollute the catalog and, worse, make
		// non-deflang locales render a bare index where the binding should be
		// (the binding's translation is empty, so lookup falls back to the index).
		if !hasFlex && isPureBindings(original) {
			return
		}
		txtIdx := resolveHash(rewritten, dbMap)

		occurrences[txtIdx] = append(occurrences[txtIdx], Occurrence{
			Path: relPath,
			Line: line,
			Col:  col,
			Tag:  tag,
		})

		n.Data = fmt.Sprintf("%d", txtIdx)
		return
	}

	if n.Type == html.ElementNode && len(attrSet) > 0 && !noTranslate {
		for i := range n.Attr {
			a := &n.Attr[i]
			if !attrSet[strings.ToLower(a.Key)] {
				continue
			}
			if a.Val == "" || isBlank(a.Val) {
				continue
			}
			line, col := tracker.find(a.Val)
			attrTag := n.Data + "[" + a.Key + "]"
			rewritten, hasFlex := rewriteFlexBlocks(a.Val, func(idx int32) {
				flexOccurrences[idx] = append(flexOccurrences[idx], Occurrence{
					Path: relPath, Line: line, Col: col, Tag: attrTag,
				})
			})
			if !hasFlex && isPureBindings(a.Val) {
				continue
			}
			txtIdx := resolveHash(rewritten, dbMap)
			occurrences[txtIdx] = append(occurrences[txtIdx], Occurrence{
				Path: relPath,
				Line: line,
				Col:  col,
				Tag:  attrTag,
			})
			a.Val = fmt.Sprintf("%d", txtIdx)
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		replaceTextNodes(c, dbMap, relPath, tracker, attrSet, noTranslate)
	}
}

// isPureBindings reports whether s consists solely of {{...}} binding
// expressions and surrounding whitespace — i.e. it carries no human-readable
// words to translate. Flex blocks are rewritten and indexed before this is
// consulted, so the only {{...}} reaching here are plain bindings
// ({{count}}, {{%price}}, {{%dist:km}}), which are runtime placeholders rather
// than catalog content. A node with an unclosed or absent {{...}} returns false.
func isPureBindings(s string) bool {
	var rest strings.Builder
	i, n := 0, len(s)
	sawBinding := false
	for i < n {
		if i+1 < n && s[i] == '{' && s[i+1] == '{' {
			j := i + 2
			for j+1 < n && (s[j] != '}' || s[j+1] != '}') {
				j++
			}
			if j+1 >= n {
				rest.WriteString(s[i:]) // unclosed: count remainder as text
				break
			}
			sawBinding = true
			i = j + 2
			continue
		}
		rest.WriteByte(s[i])
		i++
	}
	return sawBinding && isBlank(rest.String())
}

// defaultTranslatableAttrs is the set of HTML attributes whose values are
// extracted by default. Must stay in sync with wings.TranslatableAttrs.
// label/helper/error are the human-text attributes of the w-input widget.
var defaultTranslatableAttrs = []string{"title", "placeholder", "alt", "aria-label", "data-i18n", "expect", "label", "helper", "error"}

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

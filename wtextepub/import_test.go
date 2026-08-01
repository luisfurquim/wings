package wtextepub

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/luisfurquim/wings/wtext"
)

// sampleWithProps is what Content() persists for a document that
// remembers a webfont and carries book metadata: the properties ride the
// head as wt-cfg-* metas.
const sampleWithProps = `<!DOCTYPE html>
<html><head><meta charset="utf-8"/>` +
	`<meta name="wt-cfg-epub.title" content="Minha Biografia"/>` +
	`<meta name="wt-cfg-wtfont.lobster" content="https://fonts.googleapis.com/css2?family=Lobster"/>` +
	`<style>
.destaque { font-weight: bold }
</style></head><body><p>Olá <span class="destaque">mundo</span>!</p></body></html>`

// TestRoundTrip is the whole point of the pair: what the editor persists
// survives Build → Open → Document. The webfont in particular comes back
// as the store URL it always was — a reference, never bytes.
func TestRoundTrip(t *testing.T) {
	epub, err := Build(sampleWithProps, "pt-BR", "Capítulo 1", Config{Title: "Minha Biografia"})
	if err != nil {
		t.Fatal(err)
	}
	book, err := Open(epub)
	if err != nil {
		t.Fatal(err)
	}
	chapters := book.Chapters()
	if len(chapters) != 1 {
		t.Fatalf("chapters = %d (%v), want 1 — the nav document is not a chapter", len(chapters), chapters)
	}
	if chapters[0].Title != "Capítulo 1" {
		t.Errorf("chapter title = %q, want the TOC entry", chapters[0].Title)
	}
	doc, err := book.Document(chapters[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`name="wt-cfg-epub.title"`,
		`content="Minha Biografia"`,
		`name="wt-cfg-wtfont.lobster"`,
		"https://fonts.googleapis.com/css2?family=Lobster",
		".destaque { font-weight: bold }",
		`<span class="destaque">mundo</span>`,
		"Olá",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("round-tripped document is missing %q\n--- got ---\n%s", want, doc)
		}
	}
}

// TestDocumentRejectsForeignID: the chapter id crosses a dialog and comes
// back as user input; it may not address anything the book did not offer.
func TestDocumentRejectsForeignID(t *testing.T) {
	epub, err := Build(sample, "pt-BR", "Um", Config{Title: "Livro"})
	if err != nil {
		t.Fatal(err)
	}
	book, err := Open(epub)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"content.opf", "../../etc/passwd", "", "toc.xhtml"} {
		if _, err := book.Document(id); !errors.Is(err, ErrNoSuchChapter) {
			t.Errorf("Document(%q) error = %v, want ErrNoSuchChapter", id, err)
		}
	}
}

func TestOpenTooLarge(t *testing.T) {
	if _, err := Open(make([]byte, MaxImportBytes+1)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("error = %v, want ErrTooLarge", err)
	}
}

func TestOpenGarbage(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("not a zip"), bytes.Repeat([]byte{0}, 1024)} {
		if _, err := Open(data); err == nil {
			t.Errorf("Open(%d bytes of garbage) succeeded", len(data))
		}
	}
}

// --- the menu action -------------------------------------------------

func TestImportActionEmptyEditorLoadsStraight(t *testing.T) {
	epub, err := Build(sampleWithProps, "pt-BR", "Capítulo 1", Config{Title: "Livro"})
	if err != nil {
		t.Fatal(err)
	}
	core := &fakeCore{}
	if err := importAction(core, epub); err != nil {
		t.Fatalf("import = %v, want a straight load", err)
	}
	if !strings.Contains(core.content, "mundo") {
		t.Errorf("editor content = %q, want the imported chapter", core.content)
	}
}

// A document already in the editor turns the import into a question: the
// load clears the undo stack, so it is not the plugin's call to make.
func TestImportActionOccupiedEditorAsks(t *testing.T) {
	epub, err := Build(sampleWithProps, "pt-BR", "Capítulo 1", Config{Title: "Livro"})
	if err != nil {
		t.Fatal(err)
	}
	core := &fakeCore{docText: "trabalho em andamento"}
	err = importAction(core, epub)
	var pd *wtext.PendingDecision
	if !errors.As(err, &pd) {
		t.Fatalf("import = %v, want a PendingDecision", err)
	}
	if core.content != "" {
		t.Error("the document was replaced before the user answered")
	}
	if !pd.Valid() || len(pd.Options) != 1 {
		t.Fatalf("options = %v, want exactly the one chapter", pd.Options)
	}
	if pd.Message != "wtext-epub-import-replace" {
		t.Errorf("message = %q, want the replace warning", pd.Message)
	}
	if pd.Remember != "" {
		t.Error("an answer that discards the user's document must not be remembered")
	}
	if err := pd.Resume(core, pd.Options[0].Value); err != nil {
		t.Fatalf("resume = %v", err)
	}
	if !strings.Contains(core.content, "mundo") {
		t.Errorf("editor content = %q, want the imported chapter", core.content)
	}
}

// A multi-document book asks WHICH — with the book's own titles as user
// data, never as message ids.
func TestImportActionPicksChapter(t *testing.T) {
	core := &fakeCore{}
	err := importAction(core, twoChapterBook(t))
	var pd *wtext.PendingDecision
	if !errors.As(err, &pd) {
		t.Fatalf("import = %v, want a PendingDecision", err)
	}
	if len(pd.Options) != 2 {
		t.Fatalf("options = %v, want two chapters", pd.Options)
	}
	for _, want := range []string{"A ronda dos eucaliptos", "O regresso"} {
		found := false
		for _, o := range pd.Options {
			if o.Text == want {
				found = true
			}
			if o.Label != "" {
				t.Errorf("chapter option %q carries a message id (%q); titles are user data", o.Text, o.Label)
			}
		}
		if !found {
			t.Errorf("no option titled %q in %v", want, pd.Options)
		}
	}
	if err := pd.Resume(core, pd.Options[1].Value); err != nil {
		t.Fatalf("resume = %v", err)
	}
	if !strings.Contains(core.content, "segundo capítulo") {
		t.Errorf("editor content = %q, want the SECOND chapter", core.content)
	}
}

func TestResolveHref(t *testing.T) {
	cases := []struct {
		doc, href, want string
		ok              bool
	}{
		{"OEBPS/text/ch1.xhtml", "../styles/book.css", "OEBPS/styles/book.css", true},
		{"ch1.xhtml", "style.css", "style.css", true},
		{"text/ch1.xhtml", "/styles/book.css", "styles/book.css", true},
		{"text/ch1.xhtml", "https://example.com/evil.css", "", false},
		{"text/ch1.xhtml", "//example.com/evil.css", "", false},
		{"text/ch1.xhtml", "../../../../etc/passwd", "", false},
		{"text/ch1.xhtml", "#fragment", "", false},
		{"text/ch1.xhtml", "", "", false},
	}
	for _, c := range cases {
		got, ok := resolveHref(c.doc, c.href)
		if ok != c.ok || got != c.want {
			t.Errorf("resolveHref(%q, %q) = %q,%v want %q,%v", c.doc, c.href, got, ok, c.want, c.ok)
		}
	}
}

// FuzzOpen: a book is a file from the outside. Whatever it holds, opening
// it and reading every chapter it offers must not panic — a panic in a
// wasm frontend takes the whole app down.
func FuzzOpen(f *testing.F) {
	if epub, err := Build(sampleWithProps, "pt-BR", "Capítulo 1", Config{Title: "Livro"}); err == nil {
		f.Add(epub)
	}
	f.Add([]byte("PK\x03\x04 not really"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		book, err := Open(data)
		if err != nil {
			return
		}
		for _, c := range book.Chapters() {
			if _, err := book.Document(c.ID); err != nil {
				continue
			}
		}
	})
}

// twoChapterBook hand-builds the minimal EPUB that Build cannot produce:
// a book of two documents, which is what makes the import ask.
func twoChapterBook(t *testing.T) []byte {
	t.Helper()
	const container = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`
	const opf = `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Livro</dc:title><dc:identifier id="id">x</dc:identifier><dc:language>pt-BR</dc:language></metadata>
<manifest>
<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
<item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
<item id="c2" href="ch2.xhtml" media-type="application/xhtml+xml"/>
<item id="css" href="style.css" media-type="text/css"/>
</manifest>
<spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>`
	const nav = `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>TOC</title></head><body>
<nav epub:type="toc"><ol>
<li><a href="ch1.xhtml">A ronda dos eucaliptos</a></li>
<li><a href="ch2.xhtml">O regresso</a></li>
</ol></nav></body></html>`
	const ch1 = `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>1</title><link rel="stylesheet" type="text/css" href="style.css"/></head>
<body><p class="destaque">primeiro capítulo</p></body></html>`
	const ch2 = `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>2</title></head>
<body><p>segundo capítulo</p></body></html>`
	const css = `.destaque { font-style: italic }`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	add("mimetype", "application/epub+zip")
	add("META-INF/container.xml", container)
	add("OEBPS/content.opf", opf)
	add("OEBPS/nav.xhtml", nav)
	add("OEBPS/ch1.xhtml", ch1)
	add("OEBPS/ch2.xhtml", ch2)
	add("OEBPS/style.css", css)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestCheckArchive: the declaration is what gets refused, and it is
// refused before the reader library touches anything.
func TestCheckArchive(t *testing.T) {
	book := twoChapterBook(t)
	if err := checkArchive(book, MaxUnpackedBytes, MaxEntries); err != nil {
		t.Fatalf("a normal book was refused: %v", err)
	}
	if err := checkArchive(book, 8, MaxEntries); !errors.Is(err, ErrTooLarge) {
		t.Errorf("unpacked-size ceiling: err = %v, want ErrTooLarge", err)
	}
	if err := checkArchive(book, MaxUnpackedBytes, 2); !errors.Is(err, ErrTooLarge) {
		t.Errorf("entry-count ceiling: err = %v, want ErrTooLarge", err)
	}
}

// TestDocumentInlinesLinkedStylesheet: a chapter's formatting usually
// lives in a linked .css inside the book, not in its head — without
// following that link the text arrives with class attributes naming rules
// nobody ever defined.
func TestDocumentInlinesLinkedStylesheet(t *testing.T) {
	book, err := Open(twoChapterBook(t))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := book.Document(book.Chapters()[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc, ".destaque { font-style: italic }") {
		t.Errorf("linked stylesheet did not travel with the chapter:\n%s", doc)
	}
}

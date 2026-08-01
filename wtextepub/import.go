package wtextepub

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/luisfurquim/ugarit"
	"github.com/luisfurquim/wings/wtext"
	"golang.org/x/net/html"
)

// The import half: an EPUB file becomes a document the editor can load.
// It is the mirror of Build, and portable for the same reason — the
// parsing lives here, testable natively, while the js half only picks the
// file and hands the result to EditorCore.SetContent.
//
// Everything here treats the file as hostile input. It arrives from
// wherever the user got the book: the zip is bounded before it is read,
// each chapter and stylesheet is bounded as it is read, and NOTHING is
// trusted to be safe by the time it leaves — the string this package
// returns still goes through the editor's own policy walker, which is
// what actually decides what may exist in a document. This package's job
// is to be unable to hand over something ENORMOUS or malformed, not to
// decide what is safe.

// Import bounds. A book is a file from the outside: every dimension of it
// that a hostile author controls has a ceiling here.
const (
	// MaxImportBytes bounds the .epub file itself. Generous, because a
	// legitimate illustrated book with embedded fonts is genuinely large.
	MaxImportBytes = 32 << 20
	// MaxChapters bounds how many documents one book may offer.
	MaxChapters = 2000
	// MaxChapterBytes bounds one chapter's markup.
	MaxChapterBytes = 8 << 20
	// MaxStyleBytes bounds the CSS a chapter may drag in, summed over its
	// own <style> blocks and every stylesheet it links.
	MaxStyleBytes = 256 << 10
	// MaxUnpackedBytes bounds what the archive says it unpacks to, summed
	// over its entries. A 32 MiB zip can declare terabytes: the reads this
	// package performs are individually bounded, but the FIRST reads of an
	// epub are the container, the package document and the navigation, and
	// those happen inside the reader library, out of reach. Refusing the
	// file on its own declaration is the only bound that comes early
	// enough. A lying header still cannot get further than the bounded
	// reads below.
	MaxUnpackedBytes = 256 << 20
	// MaxEntries bounds how many files the archive may hold, so a book made
	// of a million empty entries costs a directory scan and nothing else.
	MaxEntries = 10000
)

var (
	// ErrTooLarge is returned for a file beyond MaxImportBytes — refused
	// whole, before any parsing: the cheapest place to say no.
	ErrTooLarge = errors.New("wtextepub: epub file too large")
	// ErrNoChapters is returned for a book with no readable document.
	ErrNoChapters = errors.New("wtextepub: no readable document in the book")
	// ErrNoSuchChapter is returned when a chapter id names nothing in the
	// book. The id crosses a dialog and comes back as user input, so it is
	// re-checked and never used to reach into the zip unvalidated.
	ErrNoSuchChapter = errors.New("wtextepub: no such chapter")
)

// Chapter is one document a book offers for import. ID is the opaque
// handle to ask for it (the book's own path for it, valid only within the
// Book that issued it); Title is USER DATA, the TOC entry or, failing
// that, the manifest id — never a message id, and never to be resolved
// through a catalog.
type Chapter struct {
	ID    string
	Title string
}

// Book is an opened EPUB. Open reads the whole file (a zip needs random
// access, and this one lives in browser memory anyway); the chapters are
// listed up front and their markup is read only when asked for.
type Book struct {
	rd       ugarit.BookReader
	chapters []Chapter
}

// Open reads an EPUB file and lists the documents it offers.
//
// The chapters are the book's TABLE OF CONTENTS, which is the reading
// order its author declared and the only list with titles a human wrote.
// A book whose TOC resolves to nothing (a bare package, a TOC that names
// no XHTML) falls back to every XHTML document in the manifest, ids for
// titles — degraded, but never empty when there is something to read.
func Open(data []byte) (*Book, error) {
	if len(data) > MaxImportBytes {
		return nil, fmt.Errorf("%w: %d bytes (max %d)", ErrTooLarge, len(data), MaxImportBytes)
	}
	if err := checkArchive(data, MaxUnpackedBytes, MaxEntries); err != nil {
		return nil, err
	}
	rd, err := ugarit.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("wtextepub: opening epub: %w", err)
	}
	b := &Book{rd: rd}
	for title, meta := range rd.Index() {
		if !isDocument(meta.MimeType) {
			continue
		}
		if b.add(title, meta.Path) {
			break
		}
	}
	if len(b.chapters) == 0 {
		for title, meta := range rd.Docs() {
			if !isDocument(meta.MimeType) {
				continue
			}
			if b.add(title, meta.Path) {
				break
			}
		}
	}
	if len(b.chapters) == 0 {
		return nil, ErrNoChapters
	}
	return b, nil
}

// checkArchive reads the zip's central directory — cheap, it is just the
// index — and refuses a file that declares more than it may. This runs
// BEFORE the epub reader opens anything: the point is to fail while the
// only thing anyone has done is read a table of contents.
func checkArchive(data []byte, maxUnpacked uint64, maxEntries int) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("wtextepub: opening epub: %w", err)
	}
	if len(zr.File) > maxEntries {
		return fmt.Errorf("%w: %d entries (max %d)", ErrTooLarge, len(zr.File), maxEntries)
	}
	var total uint64
	for _, f := range zr.File {
		total += f.UncompressedSize64
		if total > maxUnpacked {
			return fmt.Errorf("%w: archive unpacks to more than %d bytes", ErrTooLarge, maxUnpacked)
		}
	}
	return nil
}

// add appends a chapter, reporting whether the ceiling was reached and the
// listing should stop. A duplicate path (a TOC pointing twice into the
// same document, once per fragment) lists once.
func (b *Book) add(title, docPath string) (full bool) {
	if len(b.chapters) >= MaxChapters {
		return true
	}
	for _, c := range b.chapters {
		if c.ID == docPath {
			return false
		}
	}
	if title == "" {
		title = docPath
	}
	b.chapters = append(b.chapters, Chapter{ID: docPath, Title: title})
	return false
}

// Chapters returns what the book offers, in the order it declared.
func (b *Book) Chapters() []Chapter { return b.chapters }

// Document returns the chapter as a document of the shape the editor
// persists (and Content produces): the properties it carries as head
// metas, its style rules as one head <style>, its markup as the body. The
// editor's SetContent is what validates all three — class names, CSS
// declarations, property keys, every element and attribute of the body —
// so a chapter that arrives full of things a document may not hold simply
// loses them there.
func (b *Book) Document(id string) (string, error) {
	if !b.has(id) {
		return "", fmt.Errorf("%w: %q", ErrNoSuchChapter, id)
	}
	raw, err := b.read(id, MaxChapterBytes)
	if err != nil {
		return "", err
	}
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("wtextepub: parsing %s: %w", id, err)
	}

	var (
		sb    strings.Builder
		css   strings.Builder
		props []prop
		body  *html.Node
		links []string
	)
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "style":
				if n.FirstChild != nil {
					appendCSS(&css, n.FirstChild.Data)
				}
				return
			case "link":
				if href, ok := styleHref(n); ok {
					links = append(links, href)
				}
				return
			case "meta":
				if name, value, ok := docProp(n); ok {
					props = append(props, prop{name, value})
				}
				return
			case "body":
				body = n
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// Linked stylesheets are read from the book itself; a href pointing
	// anywhere else (an absolute URL, a path climbing out of the archive)
	// names something this file does not contain, and importing does not
	// go to the network.
	for _, href := range links {
		target, ok := resolveHref(id, href)
		if !ok {
			continue
		}
		sheet, err := b.read(target, MaxStyleBytes)
		if err != nil {
			continue // a missing or oversized stylesheet costs formatting, not the import
		}
		appendCSS(&css, string(sheet))
	}

	sb.WriteString("<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\"/>")
	for _, p := range props {
		sb.WriteString(`<meta name="`)
		sb.WriteString(escapeAttr(p.name))
		sb.WriteString(`" content="`)
		sb.WriteString(escapeAttr(p.value))
		sb.WriteString(`"/>`)
	}
	if css.Len() > 0 {
		sb.WriteString("<style>\n")
		sb.WriteString(css.String())
		sb.WriteString("</style>")
	}
	sb.WriteString("</head><body>")
	if body != nil {
		sb.WriteString(xhtmlBody(body))
	}
	sb.WriteString("</body></html>")
	return sb.String(), nil
}

// importAction is the menu action: open the book, and load a document
// from it into the editor.
//
// Two things can make this a question rather than an act, and they are
// asked as ONE (a decision raised from inside Resume does not chain):
// WHICH document, when the book offers several, and WHETHER to discard
// what the editor currently holds, since a content load clears the undo
// stack and the old document is not coming back. When the book has a
// single document and the editor is empty, there is nothing to ask.
//
// The answer is never remembered: "which chapter" means nothing in the
// next book, and remembering "yes, replace" would silently discard a
// document the user was working on.
func importAction(core wtext.EditorCore, data []byte) error {
	book, err := Open(data)
	if err != nil {
		return err
	}
	load := func(core wtext.EditorCore, id string) error {
		doc, err := book.Document(id)
		if err != nil {
			return err
		}
		return core.SetContent(doc)
	}
	chapters := book.Chapters()
	occupied := strings.TrimSpace(core.DocText()) != ""
	if len(chapters) == 1 && !occupied {
		return load(core, chapters[0].ID)
	}

	d := &wtext.PendingDecision{
		Title:  "wtext-epub-import",
		Resume: load,
	}
	if len(chapters) == 1 {
		d.Message = "wtext-epub-import-replace"
		d.Detail = []string{chapters[0].Title}
		d.Options = []wtext.DecisionOption{
			{Value: chapters[0].ID, Label: "wtext-epub-import-open"},
		}
		return d
	}
	d.Message = "wtext-epub-import-pick"
	if occupied {
		d.Message = "wtext-epub-import-pick-replace"
	}
	for _, c := range chapters {
		// Text, not Label: a chapter title is the book's words, not ours,
		// and must not be looked up in the app's catalog.
		d.Options = append(d.Options, wtext.DecisionOption{Value: c.ID, Text: c.Title})
	}
	return d
}

// has reports whether id names a chapter of THIS book. The id makes a
// round trip through the widget's dialog and comes back as user input:
// it is checked against the list this book issued before it reaches the
// archive, so nothing else in the zip is addressable through it.
func (b *Book) has(id string) bool {
	for _, c := range b.chapters {
		if c.ID == id {
			return true
		}
	}
	return false
}

// read pulls one file out of the book, refusing to hold more than max
// bytes of it — a zip entry declares its size, and a declaration is not a
// promise. The extra byte read past the limit is how the overflow is
// detected without trusting the header.
func (b *Book) read(docPath string, max int) ([]byte, error) {
	r, err := b.rd.DocReader(docPath)
	if err != nil {
		return nil, fmt.Errorf("wtextepub: reading %s: %w", docPath, err)
	}
	data, err := io.ReadAll(io.LimitReader(r, int64(max)+1))
	if err != nil {
		return nil, fmt.Errorf("wtextepub: reading %s: %w", docPath, err)
	}
	if len(data) > max {
		return nil, fmt.Errorf("%w: %s beyond %d bytes", ErrTooLarge, docPath, max)
	}
	return data, nil
}

// appendCSS accumulates stylesheet text up to MaxStyleBytes, truncating at
// the ceiling rather than dropping what came before: the editor validates
// each rule it can parse and ignores the rest, so a cut sheet degrades to
// fewer styles, never to a broken document.
func appendCSS(css *strings.Builder, text string) {
	room := MaxStyleBytes - css.Len()
	if room <= 0 {
		return
	}
	if len(text) > room {
		text = text[:room]
	}
	css.WriteString(text)
	css.WriteString("\n")
}

// isDocument reports whether a manifest media type is a content document.
// Anything else in the manifest — images, fonts, stylesheets, the audio of
// a read-along — is not something the editor can open.
func isDocument(mime string) bool {
	mime, _, _ = strings.Cut(mime, ";")
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "application/xhtml+xml", "text/html":
		return true
	}
	return false
}

// styleHref returns the href of a <link rel="stylesheet">.
func styleHref(n *html.Node) (string, bool) {
	var rel, href, typ string
	for _, a := range n.Attr {
		switch strings.ToLower(a.Key) {
		case "rel":
			rel = strings.ToLower(a.Val)
		case "href":
			href = a.Val
		case "type":
			typ = strings.ToLower(a.Val)
		}
	}
	if href == "" {
		return "", false
	}
	if !strings.Contains(rel, "stylesheet") && typ != "text/css" {
		return "", false
	}
	return href, true
}

// resolveHref turns a chapter-relative href into the book-relative path
// the reader takes. It refuses anything that is not a plain relative path
// inside the archive: an absolute URL (the network), a fragment or query
// (not a file), a path climbing above the root (the classic zip traversal
// — which the reader would not find anyway, and which has no business
// being asked for).
func resolveHref(docPath, href string) (string, bool) {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return "", false
	}
	u, err := url.Parse(href)
	if err != nil || u.IsAbs() || u.Host != "" || u.Opaque != "" {
		return "", false
	}
	p := u.Path
	if p == "" {
		return "", false
	}
	if !strings.HasPrefix(p, "/") {
		p = path.Join(path.Dir(docPath), p)
	}
	p = path.Clean(strings.TrimPrefix(p, "/"))
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	return p, true
}

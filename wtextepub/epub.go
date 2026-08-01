package wtextepub

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/luisfurquim/ugarit/epub30"
)

// Config is the book metadata a profile fixes at registration time. Only
// Title is required; the language is not here because it is a property
// of the moment of export (wings.Locale), not of the profile.
type Config struct {
	Title     string
	Author    string
	Publisher string
	// Fonts are webfont files to embed in the book — the store fonts the
	// document actually uses, collected by the export action. Embedding
	// is legitimate BECAUSE the stores are curated for libre catalogs
	// (OFL/Apache); do not feed fonts of restrictive licenses here.
	Fonts []EmbeddedFont
}

// EmbeddedFont is one font file packaged inside the book, with the
// metadata its @font-face rule declares. Range carries the subset's
// unicode-range: stores split each (style, weight) into per-script
// files, and every one must ship, each under its range, or readers
// render fallback glyphs exactly like the browser did.
type EmbeddedFont struct {
	Family, Style, Weight, Format string
	Range                         string
	Data                          []byte
}

// Build packages an EditorCore.Content() document as an EPUB 3 book with
// a single content page and a nav TOC. lang is the book's BCP 47 tag —
// the js menu passes wings.Locale at click time. docName names THIS
// document: it becomes the TOC entry and the content page's <title>,
// exactly as typed (the export prompt's raw string; its sanitized form
// is only the download filename), while cfg.Title stays the BOOK's
// dc:title. Empty docName falls back to cfg.Title. The returned bytes
// are the complete .epub file.
func Build(content, lang, docName string, cfg Config) ([]byte, error) {
	if cfg.Title == "" {
		return nil, fmt.Errorf("wtextepub: Config.Title is required")
	}
	if docName == "" {
		docName = cfg.Title
	}
	if lang == "" {
		lang = "en"
	}
	parts, err := splitContent(content)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	book, err := epub30.New(
		nopCloser{&buf},
		[]string{cfg.Title},
		[]string{lang},
		[]string{time.Now().UTC().Format("20060102150405")},
		authors(cfg.Author),
		publishers(cfg.Publisher),
		[]epub30.Date{{Data: time.Now().UTC().Format("2006-01-02")}},
		epub30.Signature{},
		nil,   // metatags
		"ltr", // page progression
		nil,   // no versioner: skip the ibooks version metatag
	)
	if err != nil {
		return nil, fmt.Errorf("wtextepub: creating book: %w", err)
	}

	// Embedded fonts: each file joins the OCF (and the manifest, via
	// AddFile) and its @font-face rule joins the content document's
	// style, so the reader renders the same faces the editor showed.
	// woff2 is an EPUB 3.3 core media type.
	for i, f := range cfg.Fonts {
		if len(f.Data) == 0 {
			continue
		}
		path := fontPath(f, i)
		_, _, err = book.AddFile(path, fontMime(f.Format), bytes.NewReader(f.Data), "", nil)
		if err != nil {
			return nil, fmt.Errorf("wtextepub: embedding font %s: %w", path, err)
		}
		parts.css += fontFaceRule(f, path)
	}

	page := contentDocument(docName, parts)
	// TOCItemTitle alone: a flat, single TOC entry. TOCTitle would open a
	// SECTION containing the entry — a nested duplicate for a one-page book.
	_, _, _, err = book.AddPage(
		"content.xhtml",
		"application/xhtml+xml",
		strings.NewReader(page),
		"",
		&epub30.EPubOptions{TOCItemTitle: docName},
	)
	if err != nil {
		return nil, fmt.Errorf("wtextepub: adding page: %w", err)
	}

	gen, err := epub30.NewIndexGenerator(cfg.Title)
	if err != nil {
		return nil, fmt.Errorf("wtextepub: creating TOC generator: %w", err)
	}
	if _, err = book.AddTOC(gen, ""); err != nil {
		return nil, fmt.Errorf("wtextepub: adding TOC: %w", err)
	}
	if err = book.Close(); err != nil {
		return nil, fmt.Errorf("wtextepub: closing book: %w", err)
	}
	return buf.Bytes(), nil
}

// contentDocument wraps the re-serialized body and its style rules as a
// well-formed EPUB 3 content document.
func contentDocument(title string, parts contentParts) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	sb.WriteString("<!DOCTYPE html>\n")
	sb.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>`)
	sb.WriteString(escapeText(title))
	sb.WriteString("</title>")
	// The document's own properties ride along, so that reading the book
	// back restores the document and not just its text: which webfonts it
	// remembers (as store URLs — never bytes), what a plugin configured in
	// it. A reader that does not know these metas ignores them, which is
	// what generic name/content metas are for.
	for _, p := range parts.props {
		sb.WriteString(`<meta name="`)
		sb.WriteString(escapeAttr(p.name))
		sb.WriteString(`" content="`)
		sb.WriteString(escapeAttr(p.value))
		sb.WriteString(`"/>`)
	}
	if parts.css != "" {
		sb.WriteString(`<style type="text/css">` + "\n")
		sb.WriteString(parts.css)
		sb.WriteString("</style>")
	}
	sb.WriteString("</head><body>")
	sb.WriteString(parts.body)
	sb.WriteString("</body></html>")
	return sb.String()
}

// fontPath names an embedded font file inside the book. The index keeps
// subset files of one (style, weight) apart.
func fontPath(f EmbeddedFont, i int) string {
	name := Filename(f.Family) // slug + ".epub"
	name = strings.TrimSuffix(name, ".epub")
	w := strings.ReplaceAll(f.Weight, " ", "-") // variable ranges: "100 900"
	return fmt.Sprintf("fonts/%s-%s-%s-%d.%s", name, w, f.Style, i, fontExt(f.Format))
}

func fontExt(format string) string {
	if format == "" {
		return "woff2"
	}
	return format
}

func fontMime(format string) string {
	switch fontExt(format) {
	case "woff":
		return "font/woff"
	case "ttf":
		return "font/ttf"
	case "otf":
		return "font/otf"
	default:
		return "font/woff2"
	}
}

// fontFaceRule renders the @font-face of one embedded file. Family,
// style and weight came through the wtext store parser (bounded,
// shape-checked); the escaping here is belt and suspenders.
func fontFaceRule(f EmbeddedFont, path string) string {
	fam := strings.ReplaceAll(f.Family, `"`, "")
	urange := ""
	if f.Range != "" {
		urange = " unicode-range: " + f.Range + ";"
	}
	return fmt.Sprintf(
		"@font-face { font-family: %q; font-style: %s; font-weight: %s;%s src: url(%s) format(%q); }\n",
		fam, f.Style, f.Weight, urange, path, fontExt(f.Format))
}

// Filename derives a safe download name from the book title: letters and
// digits kept (lowercased), everything else collapsed to single dashes.
func Filename(title string) string {
	var sb strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
			dash = false
			continue
		}
		if !dash && sb.Len() > 0 {
			sb.WriteRune('-')
			dash = true
		}
	}
	name := strings.TrimSuffix(sb.String(), "-")
	if name == "" {
		name = "document"
	}
	return name + ".epub"
}

func authors(a string) []epub30.Author {
	if a == "" {
		return nil
	}
	return []epub30.Author{{Data: a}}
}

func publishers(p string) []string {
	if p == "" {
		return nil
	}
	return []string{p}
}

// nopCloser adapts the in-memory buffer to the io.WriteCloser ugarit
// closes for us.
type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

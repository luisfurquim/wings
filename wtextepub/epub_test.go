package wtextepub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

// sample mirrors what EditorCore.Content() persists: browser HTML
// serialization (unclosed <br>, &nbsp; entity) plus used class rules in a
// head <style>.
const sample = `<!DOCTYPE html>
<html><head><meta charset="utf-8"/><style>
.destaque { font-weight: bold }
</style></head><body><h1>Título</h1><p>Olá&nbsp;<span class="destaque">mundo</span>!<br>Segunda linha &amp; "aspas" &lt;tag&gt;</p></body></html>`

func buildSample(t *testing.T) map[string][]byte {
	t.Helper()
	// docName ≠ cfg.Title on purpose: the tests below assert each lands in
	// its own place (TOC entry + page title vs the book's dc:title).
	b, err := Build(sample, "pt-BR", "Memórias & Causos",
		Config{Title: "Minha Biografia", Author: "Autora"})
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("output is not a zip: %v", err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = data
		if f.Name == "mimetype" && f.Method != zip.Store {
			t.Error("mimetype entry must be stored uncompressed")
		}
	}
	if zr.File[0].Name != "mimetype" {
		t.Errorf("first zip entry is %q, want mimetype", zr.File[0].Name)
	}
	return files
}

func TestBuildOCF(t *testing.T) {
	files := buildSample(t)
	if got := string(files["mimetype"]); got != "application/epub+zip" {
		t.Errorf("mimetype = %q", got)
	}
	for _, name := range []string{"META-INF/container.xml", "OEBPS/content.opf", "OEBPS/content.xhtml"} {
		if _, ok := files[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
}

// TestBuildWellFormedXML feeds every XML document of the book to
// encoding/xml — the proof that the browser-HTML → XHTML re-serialization
// actually produced well-formed XML.
func TestBuildWellFormedXML(t *testing.T) {
	files := buildSample(t)
	for name, data := range files {
		if name == "mimetype" || name == "META-INF/com.apple.ibooks.display-options.xml" {
			continue
		}
		if !strings.HasSuffix(name, ".xml") && !strings.HasSuffix(name, ".xhtml") && !strings.HasSuffix(name, ".opf") {
			continue
		}
		dec := xml.NewDecoder(bytes.NewReader(data))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("%s is not well-formed XML: %v", name, err)
				break
			}
		}
	}
}

func TestBuildContentDocument(t *testing.T) {
	files := buildSample(t)
	doc := string(files["OEBPS/content.xhtml"])
	for _, want := range []string{
		`xmlns="http://www.w3.org/1999/xhtml"`,
		"<title>Memórias &amp; Causos</title>", // docName, escaped
		".destaque { font-weight: bold }",
		"<h1>Título</h1>",
		`<span class="destaque">mundo</span>`,
		"<br/>",                            // void self-closed
		"Ol\u00e1\u00a0",                   // &nbsp; became a literal NBSP
		`Segunda linha &amp; "aspas" &lt;`, // escaping round-trips
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("content.xhtml missing %q\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "&nbsp;") {
		t.Error("content.xhtml still carries the XML-undefined &nbsp; entity")
	}
	opf := string(files["OEBPS/content.opf"])
	for _, want := range []string{"Minha Biografia", "Autora", "pt-BR"} {
		if !strings.Contains(opf, want) {
			t.Errorf("content.opf missing %q", want)
		}
	}
}

// TestBuildTOC pins the QA findings of 2026-07-12: the nav document must
// be the FIRST spine itemref (out of the spine its placement is
// reader-defined — one reader appended it after the content), the TOC
// must be flat (TOCTitle+TOCItemTitle nested a duplicate entry), and a
// coverless book must not carry a landmarks nav pointing at a cover that
// does not exist.
func TestBuildTOC(t *testing.T) {
	files := buildSample(t)
	opf := string(files["OEBPS/content.opf"])
	spine := opf[strings.Index(opf, "<spine"):]
	nav := strings.Index(spine, `idref="nav"`)
	page := strings.Index(spine, `idref="pg0"`)
	if nav == -1 || page == -1 || nav > page {
		t.Errorf("nav must be the first spine itemref:\n%s", spine)
	}
	toc := string(files["OEBPS/index.xhtml"])
	if n := strings.Count(toc, "Memórias &amp; Causos"); n != 1 { // the one flat entry, as typed
		t.Errorf("TOC mentions the doc name %d times, want 1 (flat, no nested duplicate):\n%s", n, toc)
	}
	if strings.Contains(toc, "cover.xhtml") || strings.Contains(toc, "landmarks") {
		t.Errorf("coverless book must not carry a cover landmark:\n%s", toc)
	}
}

func TestBuildValidation(t *testing.T) {
	if _, err := Build(sample, "pt-BR", "nome", Config{}); err == nil {
		t.Error("Build without a title must fail")
	}
}

func TestFilename(t *testing.T) {
	cases := map[string]string{
		"Minha Biografia":  "minha-biografia.epub",
		"Ação & Reação!":   "ação-reação.epub",
		"---":              "document.epub",
		"":                 "document.epub",
		"Relatório (2026)": "relatório-2026.epub",
	}
	for in, want := range cases {
		if got := Filename(in); got != want {
			t.Errorf("Filename(%q) = %q, want %q", in, got, want)
		}
	}
}

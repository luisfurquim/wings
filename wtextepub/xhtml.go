// Package wtextepub packages a w-text document as an EPUB 3 e-book. The
// split follows the house policy/mechanism line: this portable half turns
// EditorCore.Content() into a well-formed EPUB (testable natively, no
// browser), and the js half (toolbar.go) wires it to a toolbar button and
// hands the bytes to the browser as a download.
//
// ugarit (github.com/luisfurquim/ugarit) provides the container — OCF
// zip, OPF manifest, TOC — while wings' epubhtml policy already
// guarantees, by construction, that editor content only carries
// EPUB-safe markup. This package bridges the serialization gap between
// them: browsers serialize innerHTML as HTML, not XML — unclosed voids
// (<br>), named entities beyond XML's five (&nbsp;) — and an EPUB 3
// content document must be well-formed XHTML.
package wtextepub

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// xhtmlBody re-serializes the children of an HTML node as well-formed
// XHTML: void elements self-close, text and attribute values use only
// XML's predefined entities (U+00A0 and friends stay literal — the
// document declares UTF-8), and attribute order is stable (as parsed).
func xhtmlBody(n *html.Node) string {
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		writeXHTML(&sb, c)
	}
	return sb.String()
}

// voidElements are the HTML void elements that may appear in editor
// content or its exported wrapper; they serialize self-closed.
var voidElements = map[string]bool{
	"br": true, "hr": true, "img": true, "meta": true, "link": true,
}

func writeXHTML(sb *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.TextNode:
		sb.WriteString(escapeText(n.Data))
	case html.ElementNode:
		sb.WriteString("<")
		sb.WriteString(n.Data)
		for _, a := range n.Attr {
			sb.WriteString(" ")
			sb.WriteString(a.Key)
			sb.WriteString("=\"")
			sb.WriteString(escapeAttr(a.Val))
			sb.WriteString("\"")
		}
		if voidElements[n.Data] {
			sb.WriteString("/>")
			return
		}
		sb.WriteString(">")
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeXHTML(sb, c)
		}
		sb.WriteString("</")
		sb.WriteString(n.Data)
		sb.WriteString(">")
	}
	// Comments and doctypes do not survive the editor's policy; anything
	// else is silently dropped rather than risking malformed output.
}

// escapeText and escapeAttr escape the XML-significant characters of
// their context: quotes only matter inside a quoted attribute value.
// Everything else — including U+00A0, which browsers serialize as
// &nbsp;, undefined in XML — travels as a literal UTF-8 character.
var (
	textEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	attrEscaper = strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;",
	)
)

func escapeText(s string) string { return textEscaper.Replace(s) }
func escapeAttr(s string) string { return attrEscaper.Replace(s) }

// contentParts is what splitContent extracts from an
// EditorCore.Content() document.
type contentParts struct {
	css   string // the <style> rules of the head (may be empty)
	body  string // the body markup re-serialized as XHTML
	props []prop // the document's wt-cfg-* properties, in document order
}

// prop is one persisted document property (an editor's wt-cfg-<key> head
// meta): the webfonts the document remembers, the book metadata, whatever
// else a plugin stored in it. They ride the exported book so that reading
// it back restores the document, not merely its text — a font in
// particular comes back as the store REFERENCE it always was, never as
// bytes from the file.
type prop struct{ name, value string }

// docPropPrefix names the head metas that carry document properties. It
// mirrors the editor's own prefix (wtext.Content writes them); the two
// halves agree on the string, not on a shared constant, because this
// module is separate and its wings dependency is a published version.
const docPropPrefix = "wt-cfg-"

// splitContent parses the editor's persisted document and returns its
// style rules and its body as XHTML. html.Parse is lenient by design —
// whatever malformation it absorbs, the output side is built by
// writeXHTML and stays well-formed.
func splitContent(content string) (contentParts, error) {
	var p contentParts
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return p, fmt.Errorf("wtextepub: parsing editor content: %w", err)
	}
	var walk func(n *html.Node)
	var body *html.Node
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "style":
				if n.FirstChild != nil {
					p.css += n.FirstChild.Data
				}
				return
			case "meta":
				if name, value, ok := docProp(n); ok {
					p.props = append(p.props, prop{name, value})
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
	if body != nil {
		p.body = xhtmlBody(body)
	}
	return p, nil
}

// docProp reads a <meta name="wt-cfg-…" content="…"> node. A meta without
// both attributes, or one naming anything else (charset, viewport, a
// reader's own metadata), is not a document property and is ignored.
func docProp(n *html.Node) (name, value string, ok bool) {
	for _, a := range n.Attr {
		switch a.Key {
		case "name":
			name = a.Val
		case "content":
			value = a.Val
		}
	}
	if !strings.HasPrefix(name, docPropPrefix) || len(name) == len(docPropPrefix) {
		return "", "", false
	}
	return name, value, true
}

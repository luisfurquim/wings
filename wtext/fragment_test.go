package wtext

import (
	"strings"
	"testing"
)

func TestBuilderValid(t *testing.T) {
	f, err := NewFragment().
		Block("p", func(n Node) {
			n.Text("olá ").Mark("strong", func(m Node) {
				m.Text("mundo")
			}).Br().Text("fim")
		}).
		Block("h2", func(n Node) { n.Text("título") }).
		Done()
	if err != nil {
		t.Fatalf("valid build failed: %v", err)
	}
	if len(f.nodes) != 2 || f.nodes[0].tag != "p" || f.nodes[1].tag != "h2" {
		t.Fatalf("unexpected shape: %+v", f.nodes)
	}
	kids := f.nodes[0].kids
	if len(kids) != 4 || kids[1].tag != "strong" || kids[2].tag != "br" {
		t.Fatalf("unexpected p children: %+v", kids)
	}
}

func TestBuilderLinkAttr(t *testing.T) {
	f, err := NewFragment().
		Mark("a", func(n Node) {
			n.Attr("href", "https://bücher.example/x").Text("livros")
		}).
		Done()
	if err != nil {
		t.Fatalf("link build failed: %v", err)
	}
	if got := f.nodes[0].attrs["href"]; got != "https://xn--bcher-kva.example/x" {
		t.Errorf("href not canonicalized: %q", got)
	}
}

func TestBuilderRejects(t *testing.T) {
	cases := []struct {
		name  string
		build func() (Fragment, error)
		want  string
	}{
		{"script is not a block", func() (Fragment, error) {
			return NewFragment().Block("script", nil).Done()
		}, "not a block"},
		{"span is not a mark", func() (Fragment, error) {
			return NewFragment().Mark("span", nil).Done()
		}, "not a mark"},
		{"block inside mark", func() (Fragment, error) {
			return NewFragment().Mark("strong", func(n Node) {
				n.Block("p", nil)
			}).Done()
		}, "inside a mark"},
		{"onclick not expressible", func() (Fragment, error) {
			return NewFragment().Mark("a", func(n Node) {
				n.Attr("onclick", "alert(1)")
			}).Done()
		}, "not allowed"},
		{"javascript href", func() (Fragment, error) {
			return NewFragment().Mark("a", func(n Node) {
				n.Attr("href", "javascript:alert(1)")
			}).Done()
		}, "scheme"},
		{"style attr not expressible", func() (Fragment, error) {
			return NewFragment().Block("p", func(n Node) {
				n.Attr("style", "color:red")
			}).Done()
		}, "not allowed"},
	}
	for _, c := range cases {
		_, err := c.build()
		if err == nil {
			t.Errorf("%s: build accepted", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q does not mention %q", c.name, err, c.want)
		}
	}
}

func TestBuilderTextCleaned(t *testing.T) {
	f, err := NewFragment().Block("p", func(n Node) {
		n.Text("a‮b\x00c") // bidi override + NUL
	}).Done()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if got := f.nodes[0].kids[0].text; got != "abc" {
		t.Errorf("text not cleaned: %q", got)
	}
}

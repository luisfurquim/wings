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

func TestSplitOnBreaks(t *testing.T) {
	txt := func(s string) fnode { return fnode{text: s} }
	br := fnode{tag: "br"}
	cases := []struct {
		name string
		in   []fnode
		want [][]fnode
	}{
		{"no breaks", []fnode{txt("a")}, [][]fnode{{txt("a")}}},
		{"single br is a line break, not a split",
			[]fnode{txt("a"), br, txt("b")},
			[][]fnode{{txt("a"), br, txt("b")}}},
		{"double br splits into two groups",
			[]fnode{txt("a"), br, br, txt("b")},
			[][]fnode{{txt("a")}, {txt("b")}}},
		{"triple br still just one split, run dropped",
			[]fnode{txt("a"), br, br, br, txt("b")},
			[][]fnode{{txt("a")}, {txt("b")}}},
		{"leading double br yields an empty first group",
			[]fnode{br, br, txt("a")},
			[][]fnode{{}, {txt("a")}}},
		{"trailing double br yields an empty last group",
			[]fnode{txt("a"), br, br},
			[][]fnode{{txt("a")}, {}}},
		{"two paragraph breaks, three groups",
			[]fnode{txt("a"), br, br, txt("b"), br, br, txt("c")},
			[][]fnode{{txt("a")}, {txt("b")}, {txt("c")}}},
		{"empty input", nil, [][]fnode{{}}},
	}
	for _, c := range cases {
		got := splitOnBreaks(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %d groups, want %d (%+v)", c.name, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			if len(got[i]) != len(c.want[i]) {
				t.Errorf("%s: group %d = %+v, want %+v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

func TestSplitTopLevelBreaksSynthesizesParagraphs(t *testing.T) {
	// Bodiless paste: bare text/br at the top level, no wrapping block —
	// exactly what copyChildren(body, ...) returns when the clipboard
	// payload had no <p>/<div> around it.
	in := []fnode{{text: "First."}, {tag: "br"}, {tag: "br"}, {text: "Second."}}
	out := splitTopLevelBreaks(in)
	if len(out) != 2 || out[0].tag != "p" || out[1].tag != "p" {
		t.Fatalf("got %+v, want two synthesized <p>", out)
	}
	if len(out[0].kids) != 1 || out[0].kids[0].text != "First." {
		t.Errorf("first paragraph = %+v", out[0].kids)
	}
	if len(out[1].kids) != 1 || out[1].kids[0].text != "Second." {
		t.Errorf("second paragraph = %+v", out[1].kids)
	}
}

func TestSplitTopLevelBreaksLeavesBlockShapedAlone(t *testing.T) {
	// Already block-shaped (e.g. two real <p> from the source): nothing
	// to synthesize, even if a stray top-level <br><br> somehow sat
	// alongside them (fragmentHasBlocks short-circuits the whole thing).
	in := []fnode{{tag: "p", kids: []fnode{{text: "a"}}}, {tag: "p", kids: []fnode{{text: "b"}}}}
	out := splitTopLevelBreaks(in)
	if len(out) != 2 || out[0].tag != "p" || out[1].tag != "p" {
		t.Fatalf("got %+v, want the two <p> untouched", out)
	}
}

func TestFragmentHasBlocks(t *testing.T) {
	if fragmentHasBlocks(Fragment{nodes: []fnode{{text: "plain"}, {tag: "strong"}}}) {
		t.Error("text and marks are not blocks")
	}
	if !fragmentHasBlocks(Fragment{nodes: []fnode{{text: "a"}, {tag: "p"}}}) {
		t.Error("a <p> among the nodes should count as block-shaped")
	}
}

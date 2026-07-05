package epubhtml

import "testing"

func TestElementFor(t *testing.T) {
	cases := []struct {
		tag   string
		want  Disposition
		canon string
	}{
		// profile elements survive under their own name
		{"p", Keep, "p"},
		{"strong", Keep, "strong"},
		{"a", Keep, "a"},
		{"h6", Keep, "h6"},
		{"blockquote", Keep, "blockquote"},
		{"pre", Keep, "pre"},
		{"br", Keep, "br"},
		// legacy aliases are canonicalized (native Ctrl+B, iOS callout)
		{"b", Keep, "strong"},
		{"i", Keep, "em"},
		{"div", Keep, "p"},
		// DOM tagName arrives uppercase
		{"SCRIPT", Drop, ""},
		{"B", Keep, "strong"},
		// dangerous subtrees are dropped whole
		{"script", Drop, ""},
		{"style", Drop, ""},
		{"iframe", Drop, ""},
		{"svg", Drop, ""},
		{"math", Drop, ""},
		{"img", Drop, ""},
		{"base", Drop, ""},
		{"form", Drop, ""},
		{"input", Drop, ""},
		{"template", Drop, ""},
		// presentational and unknown markup fails toward text
		{"u", Unwrap, ""},
		{"span", Unwrap, ""},
		{"font", Unwrap, ""},
		{"marquee", Unwrap, ""},
		{"x-custom-thing", Unwrap, ""},
		// lists are out of v1
		{"ul", Unwrap, ""},
		{"ol", Unwrap, ""},
		{"li", Unwrap, ""},
	}
	for _, c := range cases {
		got := ElementFor(c.tag)
		if got.Disposition != c.want || (c.want == Keep && got.Canonical != c.canon) {
			t.Errorf("ElementFor(%q) = %+v, want disposition %v canonical %q",
				c.tag, got, c.want, c.canon)
		}
	}
}

func TestAttrFor(t *testing.T) {
	cases := []struct {
		tag, attr string
		want      AttrKind
	}{
		{"a", "href", AttrHref},
		{"A", "HREF", AttrHref},
		{"p", "class", AttrClass},
		{"strong", "class", AttrClass},
		// nothing else is expressible
		{"a", "onclick", AttrDrop},
		{"a", "style", AttrDrop},
		{"a", "target", AttrDrop},
		{"a", "id", AttrDrop},
		{"p", "href", AttrDrop},
		{"p", "onmouseover", AttrDrop},
		{"img", "src", AttrDrop},
	}
	for _, c := range cases {
		if got := AttrFor(c.tag, c.attr); got != c.want {
			t.Errorf("AttrFor(%q, %q) = %v, want %v", c.tag, c.attr, got, c.want)
		}
	}
}

func TestIsMarkIsBlock(t *testing.T) {
	if !IsMark("strong") || !IsMark("a") || IsMark("p") || IsMark("u") {
		t.Error("IsMark misclassifies")
	}
	if !IsBlock("p") || !IsBlock("h4") || IsBlock("strong") || IsBlock("div") {
		t.Error("IsBlock misclassifies")
	}
}

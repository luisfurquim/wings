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
		// class carrier: kept, but the walkers unwrap it when classless
		{"span", Keep, "span"},
		{"SPAN", Keep, "span"},
		// u joined the profile for Underline (DualMark, like b/strong)
		{"u", Keep, "u"},
		// presentational and unknown markup fails toward text
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

func TestInlineAndRequiresClass(t *testing.T) {
	for tag, wantInline := range map[string]bool{
		"strong": true, "em": true, "a": true, "code": true,
		"sup": true, "sub": true, "span": true, "SPAN": true,
		"p": false, "h1": false, "blockquote": false, "br": false,
	} {
		if got := IsInline(tag); got != wantInline {
			t.Errorf("IsInline(%q) = %v, want %v", tag, got, wantInline)
		}
	}
	for tag, want := range map[string]bool{
		"span": true, "SPAN": true,
		"strong": false, "em": false, "p": false,
	} {
		if got := RequiresClass(tag); got != want {
			t.Errorf("RequiresClass(%q) = %v, want %v", tag, got, want)
		}
	}
}

func TestBlockListMatchesProfile(t *testing.T) {
	list := BlockList()
	if len(list) != len(blocks) {
		t.Fatalf("BlockList has %d tags, profile has %d", len(list), len(blocks))
	}
	for _, tag := range list {
		if !IsBlock(tag) {
			t.Errorf("BlockList tag %q is not a profile block", tag)
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
		{"p", "style", AttrStyle},
		{"A", "STYLE", AttrStyle},
		// nothing else is expressible
		{"a", "onclick", AttrDrop},
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
	if !IsMark("strong") || !IsMark("a") || !IsMark("u") || IsMark("p") {
		t.Error("IsMark misclassifies")
	}
	if !IsBlock("p") || !IsBlock("h4") || IsBlock("strong") || IsBlock("div") {
		t.Error("IsBlock misclassifies")
	}
}

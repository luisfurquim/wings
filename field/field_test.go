package field

import (
	"regexp"
	"testing"
)

func TestEmail(t *testing.T) {
	e := NewEmail("bad")
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},           // empty is valid (required handles it)
		{"  a@b.co  ", ""}, // trimmed, valid
		{"nope", "bad"},    // no @
		{"a@b", "bad"},     // no TLD
		{"a@b.co", ""},     // valid
	}
	for _, c := range cases {
		e.FromString(c.in)
		if got := e.Validate(); got != c.want {
			t.Errorf("Email(%q): Validate=%q want %q", c.in, got, c.want)
		}
	}
	e.FromString("  x@y.z  ")
	if e.String() != "x@y.z" {
		t.Errorf("Email.String=%q want trimmed", e.String())
	}
}

func TestInt(t *testing.T) {
	i := NewInt(0, 120, "nan", "range")
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"42", ""},
		{"0", ""},
		{"120", ""},
		{"-1", "range"},
		{"121", "range"},
		{"abc", "nan"}, // not a number ≠ out of range
		{"1.5", "nan"}, // float is not an int
		{"1e2", "nan"}, // scientific notation rejected by Atoi
	}
	for _, c := range cases {
		i.FromString(c.in)
		if got := i.Validate(); got != c.want {
			t.Errorf("Int(%q): Validate=%q want %q", c.in, got, c.want)
		}
	}
	i.FromString("99")
	if n, ok := i.Int(); !ok || n != 99 {
		t.Errorf("Int.Int()=%d,%v want 99,true", n, ok)
	}
}

func TestPattern(t *testing.T) {
	p := NewPattern(regexp.MustCompile(`^\d{5}-\d{3}$`), "cep")
	p.FromString("12345-678")
	if got := p.Validate(); got != "" {
		t.Errorf("Pattern match: Validate=%q want empty", got)
	}
	p.FromString("nope")
	if got := p.Validate(); got != "cep" {
		t.Errorf("Pattern no-match: Validate=%q want %q", got, "cep")
	}
	p.FromString("")
	if got := p.Validate(); got != "" {
		t.Errorf("Pattern empty: Validate=%q want empty", got)
	}
}

func TestText(t *testing.T) {
	x := NewText()
	x.FromString("  hi  ")
	if x.String() != "hi" || x.Validate() != "" {
		t.Errorf("Text=%q Validate=%q", x.String(), x.Validate())
	}
}

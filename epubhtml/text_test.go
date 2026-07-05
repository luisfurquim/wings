package epubhtml

import "testing"

func TestCleanText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		mode TextMode
		want string
	}{
		{"plain passthrough", "hello, café", DocumentText, "hello, café"},
		{"document keeps newline and tab", "a\nb\tc", DocumentText, "a\nb\tc"},
		{"header flattens line breaks", "a\r\nb\rc\nd", HeaderValue, "a b c d"},
		{"header flattens tab", "a\tb", HeaderValue, "a b"},
		{"body normalizes breaks to CRLF", "a\nb\rc\r\nd", BodyValue, "a\r\nb\r\nc\r\nd"},
		{"body keeps tab", "campo:\tvalor", BodyValue, "campo:\tvalor"},
		{"C0 stripped", "a\x00b\x08c\x1bd", DocumentText, "abcd"},
		{"DEL and C1 stripped", "a\x7fbcd", DocumentText, "abcd"},
		{"bidi overrides stripped", "a‪b‫c‬d‭e‮f", DocumentText, "abcdef"},
		// ZWNJ is orthographically required in Persian; ZWJ builds emoji
		// sequences and Indic ligatures — both must survive every mode.
		{"ZWNJ kept (Persian)", "می‌خواهم", HeaderValue, "می‌خواهم"},
		{"ZWJ kept (emoji)", "👩‍💻", BodyValue, "👩‍💻"},
		{"bidi isolates kept", "a⁦b⁧c⁨d⁩e", DocumentText, "a⁦b⁧c⁨d⁩e"},
		{"invalid UTF-8 dropped", "a\xffb", DocumentText, "ab"},
		{"empty", "", DocumentText, ""},
	}
	for _, c := range cases {
		if got := CleanText(c.in, c.mode); got != c.want {
			t.Errorf("%s: CleanText(%q, %v) = %q, want %q", c.name, c.in, c.mode, got, c.want)
		}
	}
}

func TestCleanTextIdempotent(t *testing.T) {
	inputs := []string{
		"a\r\nb\tc‪d‌e", "plain", "👩‍💻\n\n", "\x00\x7f",
	}
	for _, in := range inputs {
		for _, mode := range []TextMode{DocumentText, HeaderValue, BodyValue} {
			once := CleanText(in, mode)
			if twice := CleanText(once, mode); twice != once {
				t.Errorf("not idempotent: mode %v in %q: %q != %q", mode, in, once, twice)
			}
		}
	}
}

func TestStructuralOK(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"plain/path-1_2", true},
		{"caminho/ação", true}, // non-ASCII text is fine; invisibles are not
		{"a‮b", false},         // bidi override
		{"a​b", false},         // zero-width space
		{"a­b", false},         // soft hyphen
		{"a\nb", false},        // control
		{"a\xffb", false},      // invalid UTF-8
		{"", true},
	}
	for _, c := range cases {
		if got := structuralOK(c.in); got != c.ok {
			t.Errorf("structuralOK(%q) = %v, want %v", c.in, got, c.ok)
		}
	}
}

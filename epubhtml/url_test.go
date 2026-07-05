package epubhtml

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestCanonicalizeHrefAccepts(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain https", "https://example.com/doc", "https://example.com/doc"},
		{"scheme lowered", "HTTPS://example.com/", "https://example.com/"},
		{"host lowered", "https://EXAMPLE.com/x", "https://example.com/x"},
		{"port kept", "http://example.com:8080/a", "http://example.com:8080/a"},
		{"query and fragment kept", "https://e.com/p?a=1&b=2#sec", "https://e.com/p?a=1&b=2#sec"},
		{"IDN to punycode", "https://bücher.example/", "https://xn--bcher-kva.example/"},
		{"cyrillic homograph exposed", "https://аpple.com/", "https://xn--pple-43d.com/"},
		{"IPv4 literal", "http://192.168.0.1/x", "http://192.168.0.1/x"},
		{"IPv6 literal", "http://[::1]:8080/x", "http://[::1]:8080/x"},
		// UTS-46 maps ignorable invisibles (ZWSP here) away during IDN
		// lookup, so the spoof collapses into the real, visible name —
		// while contextually legitimate joiners (Persian ZWNJ) still work.
		{"ignorable invisible in host mapped away", "https://exam​ple.com/",
			"https://example.com/"},
		{"fragment only", "#capitulo-1", "#capitulo-1"},
		{"mailto simple", "mailto:ana@example.com", "mailto:ana@example.com"},
		{"mailto multi addr", "mailto:a@e.com,b@e.com", "mailto:a@e.com,b@e.com"},
		{"mailto plus tag", "mailto:user+tag@example.com", "mailto:user+tag@example.com"},
		{"mailto IDN domain", "mailto:ana@bücher.example", "mailto:ana@xn--bcher-kva.example"},
		{"mailto subject kept", "mailto:a@e.com?subject=Pedido%20123",
			"mailto:a@e.com?subject=Pedido%20123"},
		{"mailto body multiline", "mailto:a@e.com?body=campo1:%0Acampo2:",
			"mailto:a@e.com?body=campo1%3A%0D%0Acampo2%3A"},
		{"mailto cc validated", "mailto:a@e.com?cc=b@e.com,c@e.com",
			"mailto:a@e.com?cc=b@e.com,c@e.com"},
		{"mailto unknown key dropped", "mailto:a@e.com?subject=hi&x-header=evil",
			"mailto:a@e.com?subject=hi"},
	}
	for _, c := range cases {
		got, err := CanonicalizeHref(c.in, nil)
		if err != nil {
			t.Errorf("%s: CanonicalizeHref(%q) error: %v", c.name, c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: CanonicalizeHref(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestCanonicalizeHrefRejects(t *testing.T) {
	cases := []struct {
		name, in string
		want     error // nil = any error accepted
	}{
		{"javascript", "javascript:alert(1)", ErrScheme},
		{"javascript mixed case", "JaVaScRiPt:alert(1)", ErrScheme},
		{"data", "data:text/html,<script>", ErrScheme},
		{"vbscript", "vbscript:x", ErrScheme},
		{"file", "file:///etc/passwd", ErrScheme},
		{"ftp", "ftp://example.com/x", ErrScheme},
		{"userinfo impersonation", "https://google.com@evil.example/", ErrUserInfo},
		{"empty host", "https:///path", ErrHost},
		{"bad port", "https://example.com:99999/", nil},
		{"relative path", "docs/cap1.html", ErrRelative},
		{"empty", "", ErrRelative},
		{"bidi override in path", "https://example.com/%E2%80%AEtxt.exe", ErrInvisibleURL},
		{"zero-width in query", "https://example.com/?q=%E2%80%8B", ErrInvisibleURL},
		{"newline in fragment", "https://example.com/#a%0Ab", nil},
		{"mailto no address", "mailto:?subject=hi", ErrMailAddress},
		{"mailto bad local", "mailto:a b@e.com", ErrMailAddress},
		{"mailto quoted exotic local", "mailto:\"a b\"@e.com", ErrMailAddress},
		{"mailto double dot", "mailto:a..b@e.com", ErrMailAddress},
		{"mailto bad cc", "mailto:a@e.com?cc=not-an-address", ErrMailAddress},
		{"mailto slashes", "mailto://a@e.com", ErrMailAddress},
		{"invalid UTF-8 host", "http://\x80", ErrHost},
		{"literal U+FFFD host", "http://a�b.com", ErrHost},
		{"oversized", "https://example.com/" + strings.Repeat("a", MaxHrefLen), ErrHrefTooLong},
	}
	for _, c := range cases {
		got, err := CanonicalizeHref(c.in, nil)
		if err == nil {
			t.Errorf("%s: CanonicalizeHref(%q) accepted as %q, want error", c.name, c.in, got)
			continue
		}
		if c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%s: CanonicalizeHref(%q) error %v, want %v", c.name, c.in, err, c.want)
		}
	}
}

// TestMailtoHeaderInjection is the attack this canonicalization exists
// for: an encoded line break inside subject must never survive into the
// header value, where a naive handler would start a new header line.
func TestMailtoHeaderInjection(t *testing.T) {
	got, err := CanonicalizeHref("mailto:a@e.com?subject=oi%0ABcc:victim@x.com", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	subj := got[strings.Index(got, "subject="):]
	if strings.Contains(subj, "%0A") || strings.Contains(subj, "%0D") {
		t.Errorf("line break survived into a header value: %q", got)
	}
}

func TestLinkPolicyHook(t *testing.T) {
	corporate := func(u *url.URL) error {
		if u.Scheme != "mailto" && u.Hostname() != "intranet.example" {
			return fmt.Errorf("host %q outside the intranet", u.Hostname())
		}
		return nil
	}
	if _, err := CanonicalizeHref("https://intranet.example/doc", corporate); err != nil {
		t.Errorf("policy rejected an allowed host: %v", err)
	}
	if _, err := CanonicalizeHref("https://evil.example/", corporate); err == nil {
		t.Error("policy failed to reject a foreign host")
	}
}

func TestCanonicalizeHrefIdempotent(t *testing.T) {
	inputs := []string{
		"https://bücher.example/path?q=1#frag",
		"mailto:a@e.com,b@e.com?subject=x%20y&body=l1%0Al2&cc=c@e.com",
		"#anchor",
		"http://[::1]:8080/x",
	}
	for _, in := range inputs {
		once, err := CanonicalizeHref(in, nil)
		if err != nil {
			t.Fatalf("first pass rejected %q: %v", in, err)
		}
		twice, err := CanonicalizeHref(once, nil)
		if err != nil {
			t.Fatalf("canonical form %q rejected on re-entry: %v", once, err)
		}
		if twice != once {
			t.Errorf("not idempotent: %q -> %q -> %q", in, once, twice)
		}
	}
}

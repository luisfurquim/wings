package epubhtml

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

// MaxHrefLen bounds both the input and the canonical output of
// CanonicalizeHref. No document needs multi-kilobyte links, and real-world
// URL handlers misbehave far below this.
const MaxHrefLen = 8192

// LinkPolicy is an optional application hook consulted after structural
// canonicalization succeeds. wings ships the mechanism; where a document
// may link to is application policy — a corporate comment field may pin
// hosts, a book may link anywhere. A nil policy allows every URL that
// passed canonicalization.
type LinkPolicy func(*url.URL) error

// Errors returned by CanonicalizeHref.
var (
	// ErrHrefTooLong reports a URL beyond MaxHrefLen.
	ErrHrefTooLong = errors.New("epubhtml: URL exceeds the size bound")
	// ErrScheme reports a scheme outside http, https, mailto or a
	// #fragment-only relative reference.
	ErrScheme = errors.New("epubhtml: URL scheme not allowed")
	// ErrUserInfo reports a user@host URL — the classic
	// "https://trusted@evil.example" impersonation, never legitimate in
	// document links.
	ErrUserInfo = errors.New("epubhtml: user@host URLs not allowed")
	// ErrHost reports a missing host or one that fails IDN validation.
	ErrHost = errors.New("epubhtml: missing or malformed host")
	// ErrPort reports a non-numeric or out-of-range port.
	ErrPort = errors.New("epubhtml: malformed port")
	// ErrInvisibleURL reports a control or invisible character in a
	// structural URL part (also when smuggled behind percent-encoding).
	ErrInvisibleURL = errors.New("epubhtml: control or invisible character in URL")
	// ErrMailAddress reports a malformed address in a mailto target or
	// its cc/bcc fields.
	ErrMailAddress = errors.New("epubhtml: malformed e-mail address")
	// ErrRelative reports a relative reference other than "#fragment".
	ErrRelative = errors.New("epubhtml: only #fragment relative URLs allowed")
)

// CanonicalizeHref validates raw and returns its canonical form, the only
// form ever stored in a document. The attacker's bytes are never passed
// through: the URL is parsed once, each part is validated, and the output
// is rebuilt from the validated parts.
//
// Canonicalization also makes spoofing visible: internationalized hosts
// come out in their punycode (xn--) form, so a Cyrillic а in "аpple.com"
// stops impersonating the Latin name (homograph attack), and mailto
// queries are rebuilt with header values that cannot smuggle extra headers.
func CanonicalizeHref(raw string, policy LinkPolicy) (string, error) {
	if len(raw) > MaxHrefLen {
		return "", ErrHrefTooLong
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("epubhtml: unparseable URL: %w", err)
	}

	var canon *url.URL
	switch strings.ToLower(u.Scheme) {
	case "":
		canon, err = canonFragment(u)
	case "http", "https":
		canon, err = canonWeb(u)
	case "mailto":
		canon, err = canonMailto(u)
	default:
		return "", fmt.Errorf("%w: %q", ErrScheme, u.Scheme)
	}
	if err != nil {
		return "", err
	}

	if policy != nil {
		if err = policy(canon); err != nil {
			return "", fmt.Errorf("epubhtml: link rejected by application policy: %w", err)
		}
	}

	s := canon.String()
	if len(s) > MaxHrefLen {
		return "", ErrHrefTooLong
	}
	return s, nil
}

// canonFragment accepts only pure "#anchor" references — the EPUB-internal
// link form. Any other relative reference is rejected.
func canonFragment(u *url.URL) (*url.URL, error) {
	if u.Opaque != "" || u.Host != "" || u.User != nil ||
		u.Path != "" || u.RawQuery != "" || u.Fragment == "" {
		return nil, ErrRelative
	}
	if !structuralOK(u.Fragment) {
		return nil, ErrInvisibleURL
	}
	return &url.URL{Fragment: u.Fragment}, nil
}

// canonWeb canonicalizes an http/https URL: no userinfo, host reduced to
// its ASCII (punycode) form, and every part — including the
// percent-decoded query — free of invisible characters.
func canonWeb(u *url.URL) (*url.URL, error) {
	if u.User != nil {
		return nil, ErrUserInfo
	}
	if u.Opaque != "" {
		return nil, ErrHost
	}
	host, err := canonHost(u.Hostname())
	if err != nil {
		return nil, err
	}
	if p := u.Port(); p != "" {
		n, aerr := strconv.Atoi(p)
		if aerr != nil || n < 1 || n > 65535 {
			return nil, ErrPort
		}
		host = net.JoinHostPort(host, p)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]" // bracket a portless IPv6 literal back up
	}
	if !structuralOK(u.Path) || !structuralOK(u.Fragment) {
		return nil, ErrInvisibleURL
	}
	// The query is kept verbatim (rebuilding would reorder it), but its
	// decoded form is scanned so percent-encoding cannot smuggle
	// invisibles past the check.
	if dq, uerr := url.QueryUnescape(u.RawQuery); uerr != nil || !structuralOK(dq) {
		return nil, ErrInvisibleURL
	}
	return &url.URL{
		Scheme:   strings.ToLower(u.Scheme),
		Host:     host,
		Path:     u.Path,
		RawQuery: u.RawQuery,
		Fragment: u.Fragment,
	}, nil
}

// canonHost returns the canonical ASCII form of a host name. IP literals
// pass through in their normalized text form; everything else goes through
// the UTS-46 IDN lookup profile, which maps internationalized names to
// punycode and rejects the malformed.
func canonHost(host string) (string, error) {
	if host == "" {
		return "", ErrHost
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}
	// idna's encoder happily punycodes invalid UTF-8 (as U+FFFD) into a
	// label its own decoder then rejects — the canonical form would not
	// survive re-entry. Found by FuzzCanonicalizeHref ("http://\x80").
	if strings.ContainsRune(host, utf8.RuneError) {
		return "", ErrHost
	}
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrHost, err)
	}
	// UTS-46 maps ignorable characters away, so a host spelled entirely
	// out of them (a bare soft hyphen, say) comes back EMPTY with no
	// error — and the Lookup profile does not enforce the RFC 1035
	// length limits on Unicode input either, only on the xn-- form.
	// Both found by FuzzCanonicalizeHref.
	if ascii == "" || len(ascii) > 253 {
		return "", ErrHost
	}
	for _, label := range strings.Split(ascii, ".") {
		if label == "" || len(label) > 63 {
			return "", ErrHost
		}
	}
	// The canonical form must survive its own validation, or idempotence
	// breaks: bidi label rules and joiner context are only fully checked
	// when the label arrives already punycoded.
	if _, err = idna.Lookup.ToASCII(ascii); err != nil {
		return "", fmt.Errorf("%w: %v", ErrHost, err)
	}
	return ascii, nil
}

// canonMailto canonicalizes a mailto URL per RFC 6068: a non-empty address
// list in the opaque part, plus an allowlisted set of query fields.
// Address fields (including cc/bcc) are validated as address lists;
// subject is flattened to a single line (header injection lives in line
// breaks); body keeps line breaks — form-style templates for automated
// processing at the recipient are a legitimate use — normalized to CRLF.
// Unknown fields (In-Reply-To, arbitrary headers) are simply not copied.
func canonMailto(u *url.URL) (*url.URL, error) {
	if u.Host != "" || u.User != nil || u.Path != "" {
		return nil, ErrMailAddress // "mailto://..." and friends
	}
	rawAddrs, err := url.PathUnescape(u.Opaque)
	if err != nil {
		return nil, ErrMailAddress
	}
	addrs, err := canonAddrList(rawAddrs)
	if err != nil {
		return nil, err
	}

	var qparts []string
	for _, pair := range strings.Split(u.RawQuery, "&") {
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		// PathUnescape, not QueryUnescape: in mailto, "+" is a literal
		// plus (common in local parts), not an encoded space.
		dv, uerr := url.PathUnescape(v)
		if uerr != nil {
			return nil, ErrInvisibleURL
		}
		switch strings.ToLower(k) {
		case "subject":
			qparts = append(qparts, "subject="+escapeMailValue(CleanText(dv, HeaderValue)))
		case "body":
			qparts = append(qparts, "body="+escapeMailValue(CleanText(dv, BodyValue)))
		case "cc", "bcc":
			list, aerr := canonAddrList(dv)
			if aerr != nil {
				return nil, aerr
			}
			qparts = append(qparts, strings.ToLower(k)+"="+list)
		}
	}

	return &url.URL{
		Scheme:   "mailto",
		Opaque:   addrs,
		RawQuery: strings.Join(qparts, "&"),
	}, nil
}

// canonAddrList validates a comma-separated address list and returns it in
// canonical form (punycode domains included).
func canonAddrList(list string) (string, error) {
	parts := strings.Split(list, ",")
	out := make([]string, 0, len(parts))
	for _, addr := range parts {
		c, err := canonAddr(addr)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	return strings.Join(out, ","), nil
}

// canonAddr validates one e-mail address. The local part accepts a
// conservative subset of RFC 5322 atext — letters, digits and "._+-",
// which covers real-world addresses including plus-tagging — because the
// exotic remainder (quoting, %, and other relics) is exactly where mailto
// handler exploits have historically hidden. Reject, don't repair.
func canonAddr(addr string) (string, error) {
	local, domain, found := strings.Cut(addr, "@")
	if !found || local == "" || len(local) > 64 || len(domain) > 255 {
		return "", fmt.Errorf("%w: %q", ErrMailAddress, addr)
	}
	if strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") ||
		strings.Contains(local, "..") {
		return "", fmt.Errorf("%w: %q", ErrMailAddress, addr)
	}
	for _, r := range local {
		ok := r == '.' || r == '_' || r == '+' || r == '-' ||
			(r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !ok {
			return "", fmt.Errorf("%w: %q", ErrMailAddress, addr)
		}
	}
	ascii, err := canonHost(domain)
	if err != nil {
		return "", fmt.Errorf("%w: %q", ErrMailAddress, addr)
	}
	return local + "@" + ascii, nil
}

// escapeMailValue percent-encodes a cleaned header/body value for
// reassembly. QueryEscape's "+" form for spaces is avoided ("%20" instead):
// legacy mailto handlers are known to mishandle "+".
func escapeMailValue(v string) string {
	return strings.ReplaceAll(url.QueryEscape(v), "+", "%20")
}

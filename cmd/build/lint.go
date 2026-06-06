package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

// lintTemplates scans every non-generated .html file under roots for binding
// attribute names that contain an uppercase letter. The browser lowercases all
// attribute names, so a camelCase binding name (?showExtra, *minhaLista,
// &dataFoo) can never match its model key — a silent render bug. Any violation
// fails the build.
func lintTemplates(roots ...string) error {
	var violations []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := strings.ToLower(d.Name())
			if d.IsDir() || !strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".i18n.html") {
				return nil
			}
			v, err := lintFile(path)
			if err != nil {
				return err
			}
			violations = append(violations, v...)
			return nil
		})
		if err != nil {
			return err
		}
	}
	if len(violations) == 0 {
		return nil
	}
	return fmt.Errorf("camelCase binding name(s) — the browser lowercases attribute names, so these never match the model. Use snake_case:\n  %s",
		strings.Join(violations, "\n  "))
}

// lintFile reports every binding attribute name with an uppercase letter in a
// single template, as "path:line: name" strings. It scans the raw bytes via the
// tokenizer because html.Parse lowercases attribute names, destroying the
// evidence; Raw() preserves the original casing.
func lintFile(path string) ([]string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var violations []string
	z := html.NewTokenizer(bytes.NewReader(src))
	offset := 0
	for {
		tt := z.Next()
		start := offset
		offset += len(z.Raw())
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		for _, attr := range attrNames(z.Raw()) {
			if id := bindingIdent(attr); id != "" && hasUpper(id) {
				line := 1 + bytes.Count(src[:start], []byte{'\n'})
				violations = append(violations, fmt.Sprintf("%s:%d: %s", path, line, attr))
			}
		}
	}
	return violations, nil
}

// attrNames extracts the attribute-name tokens from the raw bytes of a start
// tag, skipping quoted values so a sigil inside a value is not mistaken for a
// name.
func attrNames(raw []byte) []string {
	s := string(raw)
	n := len(s)
	i := 0
	// skip '<' (and a stray '/' just in case) then the tag name
	for i < n && (s[i] == '<' || s[i] == '/') {
		i++
	}
	for i < n && !isSpace(s[i]) && s[i] != '>' && s[i] != '/' {
		i++
	}
	var names []string
	for i < n {
		for i < n && isSpace(s[i]) {
			i++
		}
		if i >= n || s[i] == '>' || s[i] == '/' {
			break
		}
		nameStart := i
		for i < n && !isSpace(s[i]) && s[i] != '=' && s[i] != '>' && s[i] != '/' {
			i++
		}
		names = append(names, s[nameStart:i])
		i = skipValue(s, i)
	}
	return names
}

// skipValue advances past an optional ="value" (quoted or bare) starting at i,
// returning the index just after it (or i if there is no value).
func skipValue(s string, i int) int {
	n := len(s)
	j := i
	for j < n && isSpace(s[j]) {
		j++
	}
	if j >= n || s[j] != '=' {
		return i
	}
	j++
	for j < n && isSpace(s[j]) {
		j++
	}
	if j < n && (s[j] == '"' || s[j] == '\'') {
		q := s[j]
		j++
		for j < n && s[j] != q {
			j++
		}
		if j < n {
			j++
		}
		return j
	}
	for j < n && !isSpace(s[j]) && s[j] != '>' && s[j] != '/' {
		j++
	}
	return j
}

// bindingIdent returns the identifier carried by a binding-sigil attribute name
// (?cond, ?!cond, *arr, **arr, &attr), or "" if name is not such a binding. The
// identifier is the leading run of [A-Za-z0-9_] after the sigil, stopping at any
// operator (:, !, ^, $, *) the syntax may append.
func bindingIdent(name string) string {
	var rest string
	switch {
	case strings.HasPrefix(name, "**"):
		rest = name[2:]
	case strings.HasPrefix(name, "*"):
		rest = name[1:]
	case strings.HasPrefix(name, "&"):
		rest = name[1:]
	case strings.HasPrefix(name, "?"):
		rest = strings.TrimPrefix(name[1:], "!")
	default:
		return ""
	}
	end := 0
	for end < len(rest) && isIdentChar(rest[end]) {
		end++
	}
	return rest[:end]
}

func hasUpper(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			return true
		}
	}
	return false
}

func isIdentChar(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

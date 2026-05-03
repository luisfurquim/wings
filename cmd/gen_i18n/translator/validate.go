package translator

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// reTplRef matches {{...}} template references. These must be preserved
	// verbatim by the translator.
	reTplRef = regexp.MustCompile(`\{\{[^}]+\}\}`)

	// rePctTok matches %word numeric placeholder tokens (e.g. %n, %count).
	// These must be preserved verbatim by the translator.
	rePctTok = regexp.MustCompile(`%[A-Za-z_]\w*`)
)

// ValidateCell checks that all required placeholders from src appear verbatim
// in dst, and that dst introduces no new placeholders not present in src.
// For simple text entries (cellKey == SimpleTextKey) it verifies {{...}}
// template references; for flex cells it verifies %word tokens.
func ValidateCell(cellKey, src, dst string) error {
	if src == "" {
		return nil
	}
	var srcToks, dstToks []string
	if cellKey == SimpleTextKey {
		srcToks = extractTemplateRefs(src)
		dstToks = extractTemplateRefs(dst)
	} else {
		srcToks = extractPctTokens(src)
		dstToks = extractPctTokens(dst)
	}

	srcSet := make(map[string]bool, len(srcToks))
	for _, tok := range srcToks {
		srcSet[tok] = true
	}

	var missing []string
	for _, tok := range srcToks {
		if !strings.Contains(dst, tok) {
			missing = append(missing, tok)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing placeholder(s): %s", strings.Join(missing, ", "))
	}

	for _, tok := range dstToks {
		if !srcSet[tok] {
			return fmt.Errorf("destination introduces unknown token %q (not in source)", tok)
		}
	}
	return nil
}

// extractTemplateRefs returns all {{...}} substrings in s, deduplicated,
// preserving first-occurrence order.
func extractTemplateRefs(s string) []string {
	return dedup(reTplRef.FindAllString(s, -1))
}

// extractPctTokens returns all %word tokens in s, deduplicated, preserving
// first-occurrence order.
func extractPctTokens(s string) []string {
	return dedup(rePctTok.FindAllString(s, -1))
}

func dedup(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

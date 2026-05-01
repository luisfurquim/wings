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
// in dst. For simple text entries (cellKey == SimpleTextKey) it verifies
// {{...}} template references; for flex cells it verifies %word tokens.
// Returns nil when all required placeholders are present in dst.
func ValidateCell(cellKey, src, dst string) error {
	if src == "" {
		return nil
	}
	var required []string
	if cellKey == SimpleTextKey {
		required = extractTemplateRefs(src)
	} else {
		required = extractPctTokens(src)
	}
	var missing []string
	for _, tok := range required {
		if !strings.Contains(dst, tok) {
			missing = append(missing, tok)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("missing placeholder(s): %s", strings.Join(missing, ", "))
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

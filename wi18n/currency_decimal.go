package wi18n

import "strconv"

// formatDecimal is a locale-agnostic fallback: it places the decimal point at
// `decimals` positions from the right, with no grouping separator. Used when
// Intl is unavailable or when the Currency has no Code. decimals is expected to
// be >= 0 (it comes from currencyDecimals, range 0..4).
//
// The sign is stripped from the formatted digit string rather than by negating
// the int64, so the minimum int64 value (whose negation overflows) formats
// correctly instead of leaking a stray '-'.
func formatDecimal(amount int64, decimals int) string {
	if decimals <= 0 {
		return strconv.FormatInt(amount, 10)
	}
	neg := amount < 0
	digits := strconv.FormatInt(amount, 10)
	if neg {
		digits = digits[1:] // drop the leading '-'; re-added below
	}
	for len(digits) <= decimals {
		digits = "0" + digits
	}
	cut := len(digits) - decimals
	whole, frac := digits[:cut], digits[cut:]
	if neg {
		return "-" + whole + "." + frac
	}
	return whole + "." + frac
}

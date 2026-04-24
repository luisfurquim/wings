//go:build js && wasm

package wi18n

import "testing"

func TestCurrencyDecimals(t *testing.T) {
	cases := map[string]int{
		"USD": 2,
		"BRL": 2,
		"EUR": 2,
		"JPY": 0,
		"KRW": 0,
		"BHD": 3,
		"CLF": 4,
		"":    2, // empty → default
		"XYZ": 2, // unknown → default
	}
	for code, want := range cases {
		if got := currencyDecimals(code); got != want {
			t.Errorf("currencyDecimals(%q) = %d, want %d", code, got, want)
		}
	}
}

func TestFormatDecimal(t *testing.T) {
	cases := []struct {
		amount   int64
		decimals int
		want     string
	}{
		{12345, 2, "123.45"},
		{100, 2, "1.00"},
		{5, 2, "0.05"},
		{0, 2, "0.00"},
		{-12345, 2, "-123.45"},
		{1000, 0, "1000"},
		{-42, 0, "-42"},
		{123456, 3, "123.456"},
		{1, 4, "0.0001"},
	}
	for _, c := range cases {
		got := formatDecimal(c.amount, c.decimals)
		if got != c.want {
			t.Errorf("formatDecimal(%d, %d) = %q, want %q", c.amount, c.decimals, got, c.want)
		}
	}
}

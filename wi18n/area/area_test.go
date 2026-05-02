package area

import (
	"math"
	"testing"
)

func TestConvert(t *testing.T) {
	const eps = 1e-6
	cases := []struct {
		m2      float64
		unit    string
		wantSym string
		wantVal float64
	}{
		{1, "m2", "m²", 1},
		{1e6, "km2", "km²", 1},
		{1, "cm2", "cm²", 1e4},
		{1, "mm2", "mm²", 1e6},
		{1e4, "ha", "ha", 1},
		{1609.344 * 1609.344, "mi2", "mi²", 1},
		{0.3048 * 0.3048, "ft2", "ft²", 1},
		{0.9144 * 0.9144, "yd2", "yd²", 1},
		{0.0254 * 0.0254, "in2", "in²", 1},
		{4046.8564224, "ac", "ac", 1},
	}
	for _, c := range cases {
		val, sym, err := Area{c.m2}.Convert(c.unit)
		if err != nil {
			t.Errorf("Convert(%g m², %q): unexpected error: %v", c.m2, c.unit, err)
			continue
		}
		if sym != c.wantSym {
			t.Errorf("Convert(%g, %q): sym=%q, want %q", c.m2, c.unit, sym, c.wantSym)
		}
		if math.Abs(val-c.wantVal) > eps {
			t.Errorf("Convert(%g, %q): val=%g, want %g", c.m2, c.unit, val, c.wantVal)
		}
	}
}

func TestConvertUnknownUnit(t *testing.T) {
	_, _, err := Area{1}.Convert("barn")
	if err == nil {
		t.Error("expected error for unknown unit, got nil")
	}
}

func TestDefaultUnit(t *testing.T) {
	cases := []struct{ locale, want string }{
		{"en-US", "ft2"},
		{"pt-BR", "m2"},
		{"de-DE", "m2"},
		{"en-GB", "m2"},
	}
	for _, c := range cases {
		if got := DefaultUnit(c.locale); got != c.want {
			t.Errorf("DefaultUnit(%q) = %q, want %q", c.locale, got, c.want)
		}
	}
}

func TestDefaultDecimals(t *testing.T) {
	cases := []struct {
		unit string
		want int
	}{
		{"m2", 0},
		{"cm2", 0},
		{"mm2", 0},
		{"ft2", 0},
		{"yd2", 1},
		{"in2", 1},
		{"km2", 2},
		{"ha", 2},
		{"mi2", 2},
		{"ac", 2},
	}
	for _, c := range cases {
		if got := DefaultDecimals(c.unit); got != c.want {
			t.Errorf("DefaultDecimals(%q) = %d, want %d", c.unit, got, c.want)
		}
	}
}

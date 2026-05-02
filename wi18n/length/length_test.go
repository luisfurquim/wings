package length

import (
	"math"
	"testing"
)

func TestConvert(t *testing.T) {
	cases := []struct {
		meters   float64
		unit     string
		wantSym  string
		wantApprox float64
		eps      float64
	}{
		{1000, "km", "km", 1, 1e-9},
		{1609.344, "mi", "mi", 1, 1e-9},
		{0.3048, "ft", "ft", 1, 1e-9},
		{0.9144, "yd", "yd", 1, 1e-9},
		{0.0254, "in", "in", 1, 1e-9},
		{1852, "nmi", "nmi", 1, 1e-9},
		{4828.032, "league", "lea", 1, 1e-9},
		{1, "cm", "cm", 100, 1e-9},
		{1, "mm", "mm", 1000, 1e-9},
		{1, "m", "m", 1, 1e-9},
	}
	for _, c := range cases {
		val, sym, err := Length{c.meters}.Convert(c.unit)
		if err != nil {
			t.Errorf("Convert(%g, %q): unexpected error: %v", c.meters, c.unit, err)
			continue
		}
		if sym != c.wantSym {
			t.Errorf("Convert(%g, %q): sym=%q, want %q", c.meters, c.unit, sym, c.wantSym)
		}
		if math.Abs(val-c.wantApprox) > c.eps {
			t.Errorf("Convert(%g, %q): val=%g, want %g (eps %g)", c.meters, c.unit, val, c.wantApprox, c.eps)
		}
	}
}

func TestConvertUnknownUnit(t *testing.T) {
	_, _, err := Length{1}.Convert("parsec")
	if err == nil {
		t.Error("Convert with unknown unit: expected error, got nil")
	}
}

func TestDefaultUnit(t *testing.T) {
	cases := []struct{ locale, want string }{
		{"en-US", "mi"},
		{"en-GB", "mi"},
		{"en-LR", "mi"},
		{"my-MM", "mi"},
		{"pt-BR", "m"},
		{"de-DE", "m"},
		{"fr-FR", "m"},
		{"en-AU", "m"},
		{"zh-CN", "m"},
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
		{"m", 0},
		{"cm", 0},
		{"mm", 0},
		{"km", 1},
		{"ft", 1},
		{"yd", 1},
		{"in", 1},
		{"mi", 2},
		{"nmi", 2},
		{"league", 2},
	}
	for _, c := range cases {
		if got := DefaultDecimals(c.unit); got != c.want {
			t.Errorf("DefaultDecimals(%q) = %d, want %d", c.unit, got, c.want)
		}
	}
}

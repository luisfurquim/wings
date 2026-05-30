package weight

import (
	"math"
	"testing"
)

func TestConvert(t *testing.T) {
	const eps = 1e-9
	cases := []struct {
		kg      float64
		unit    string
		wantSym string
		wantVal float64
	}{
		{1, "kg", "kg", 1},
		{0.001, "g", "g", 1},
		{0.000001, "mg", "mg", 1},
		{1000, "t", "t", 1},
		{lbKg, "lb", "lb", 1},
		{lbKg / 16, "oz", "oz", 1},
		{lbKg * 14, "st", "st", 1},
	}
	for _, c := range cases {
		val, sym, err := Weight{c.kg}.Convert(c.unit)
		if err != nil {
			t.Errorf("Convert(%g kg, %q): unexpected error: %v", c.kg, c.unit, err)
			continue
		}
		if sym != c.wantSym {
			t.Errorf("Convert(%g, %q): sym=%q, want %q", c.kg, c.unit, sym, c.wantSym)
		}
		if math.Abs(val-c.wantVal) > eps {
			t.Errorf("Convert(%g, %q): val=%g, want %g", c.kg, c.unit, val, c.wantVal)
		}
	}
}

func TestConvertUnknownUnit(t *testing.T) {
	_, _, err := Weight{1}.Convert("slug")
	if err == nil {
		t.Error("expected error for unknown unit, got nil")
	}
}

func TestDefaultUnit(t *testing.T) {
	cases := []struct{ locale, want string }{
		{"en-US", "lb"},
		{"pt-BR", "kg"},
		{"de-DE", "kg"},
		{"en-GB", "kg"},
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
		{"g", 0},
		{"mg", 0},
		{"t", 3},
		{"oz", 1},
		{"kg", 2},
		{"lb", 2},
		{"st", 2},
	}
	for _, c := range cases {
		if got := DefaultDecimals(c.unit); got != c.want {
			t.Errorf("DefaultDecimals(%q) = %d, want %d", c.unit, got, c.want)
		}
	}
}

func TestNew(t *testing.T) {
	cases := []struct {
		val  float64
		unit string
	}{{5, "kg"}, {500, "g"}, {1000, "mg"}, {2, "t"}, {10, "lb"}, {32, "oz"}, {1, "st"}}
	for _, c := range cases {
		v, err := New(c.val, c.unit)
		if err != nil {
			t.Errorf("New(%g, %q): unexpected error: %v", c.val, c.unit, err)
			continue
		}
		got, _, err := v.Convert(c.unit)
		if err != nil {
			t.Errorf("New(%g, %q).Convert: unexpected error: %v", c.val, c.unit, err)
			continue
		}
		if math.Abs(got-c.val) > 1e-9 {
			t.Errorf("New(%g, %q) round-trip = %g, want %g", c.val, c.unit, got, c.val)
		}
	}
}

func TestNewUnknownUnit(t *testing.T) {
	if _, err := New(1, "firkin"); err == nil {
		t.Error("New with unknown unit: expected error, got nil")
	}
}

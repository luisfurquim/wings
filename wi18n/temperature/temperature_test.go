package temperature

import (
	"math"
	"testing"
)

func TestConvert(t *testing.T) {
	const eps = 1e-9
	cases := []struct {
		kelvin  float64
		unit    string
		wantSym string
		wantVal float64
	}{
		{273.15, "c", "°C", 0},
		{273.15, "celsius", "°C", 0},
		{373.15, "c", "°C", 100},
		{273.15, "f", "°F", 32},
		{373.15, "f", "°F", 212},
		{0, "k", "K", 0},
		{300, "k", "K", 300},
		{300, "kelvin", "K", 300},
		{273.15, "r", "°R", 273.15 * 9 / 5},
		{0, "r", "°R", 0},
	}
	for _, c := range cases {
		val, sym, err := Temperature{c.kelvin}.Convert(c.unit)
		if err != nil {
			t.Errorf("Convert(%g K, %q): unexpected error: %v", c.kelvin, c.unit, err)
			continue
		}
		if sym != c.wantSym {
			t.Errorf("Convert(%g K, %q): sym=%q, want %q", c.kelvin, c.unit, sym, c.wantSym)
		}
		if math.Abs(val-c.wantVal) > eps {
			t.Errorf("Convert(%g K, %q): val=%g, want %g", c.kelvin, c.unit, val, c.wantVal)
		}
	}
}

func TestConvertUnknownUnit(t *testing.T) {
	_, _, err := Temperature{300}.Convert("planck")
	if err == nil {
		t.Error("Convert with unknown unit: expected error, got nil")
	}
}

func TestDefaultUnit(t *testing.T) {
	cases := []struct{ locale, want string }{
		{"en-US", "f"},
		{"en-BS", "f"},
		{"en-BZ", "f"},
		{"en-KY", "f"},
		{"pt-BR", "c"},
		{"de-DE", "c"},
		{"fr-FR", "c"},
		{"en-GB", "c"},
		{"en-AU", "c"},
		{"zh-CN", "c"},
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
		{"k", 2},
		{"kelvin", 2},
		{"c", 1},
		{"celsius", 1},
		{"f", 1},
		{"fahrenheit", 1},
		{"r", 1},
		{"rankine", 1},
	}
	for _, c := range cases {
		if got := DefaultDecimals(c.unit); got != c.want {
			t.Errorf("DefaultDecimals(%q) = %d, want %d", c.unit, got, c.want)
		}
	}
}

func TestNew(t *testing.T) {
	// Round-trip: New(v, unit) → Convert(unit) ≈ v
	cases := []struct {
		val  float64
		unit string
	}{
		{300, "k"}, {300, "kelvin"},
		{26.85, "c"}, {26.85, "celsius"},
		{80.33, "f"}, {80.33, "fahrenheit"},
		{540, "r"}, {540, "rankine"},
	}
	for _, c := range cases {
		temp, err := New(c.val, c.unit)
		if err != nil {
			t.Errorf("New(%g, %q): unexpected error: %v", c.val, c.unit, err)
			continue
		}
		got, _, err := temp.Convert(c.unit)
		if err != nil {
			t.Errorf("New(%g, %q).Convert: unexpected error: %v", c.val, c.unit, err)
			continue
		}
		if math.Abs(got-c.val) > 1e-9 {
			t.Errorf("New(%g, %q) round-trip = %g, want %g", c.val, c.unit, got, c.val)
		}
	}
	// Known value: 98.6 °F = 310.15 K
	temp, _ := New(98.6, "f")
	if math.Abs(temp.Kelvin-310.15) > 1e-9 {
		t.Errorf("New(98.6, f) = %g K, want 310.15", temp.Kelvin)
	}
}

func TestNewUnknownUnit(t *testing.T) {
	if _, err := New(0, "newton"); err == nil {
		t.Error("New with unknown unit: expected error, got nil")
	}
}

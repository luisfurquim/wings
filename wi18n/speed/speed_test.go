package speed

import (
	"math"
	"testing"
)

func TestConvert(t *testing.T) {
	const eps = 1e-6
	cases := []struct {
		ms      float64
		unit    string
		wantSym string
		wantVal float64
	}{
		{1, "ms", "m/s", 1},
		{1, "kmh", "km/h", 3.6},
		{0.44704, "mph", "mph", 1},        // 1 mph = 0.44704 m/s exactly
		{0.514444, "kn", "kn", 1},         // ≈1 kn
		{0.3048, "fps", "ft/s", 1},        // 1 ft/s = 0.3048 m/s
		{100.0 / 3.6, "kmh", "km/h", 100}, // 100 km/h
	}
	for _, c := range cases {
		val, sym, err := Speed{c.ms}.Convert(c.unit)
		if err != nil {
			t.Errorf("Convert(%g m/s, %q): unexpected error: %v", c.ms, c.unit, err)
			continue
		}
		if sym != c.wantSym {
			t.Errorf("Convert(%g, %q): sym=%q, want %q", c.ms, c.unit, sym, c.wantSym)
		}
		if math.Abs(val-c.wantVal) > eps {
			t.Errorf("Convert(%g, %q): val=%g, want %g", c.ms, c.unit, val, c.wantVal)
		}
	}
}

func TestConvertUnknownUnit(t *testing.T) {
	_, _, err := Speed{1}.Convert("warp")
	if err == nil {
		t.Error("expected error for unknown unit, got nil")
	}
}

func TestDefaultUnit(t *testing.T) {
	cases := []struct{ locale, want string }{
		{"en-US", "mph"},
		{"en-GB", "mph"},
		{"en-LR", "mph"},
		{"my-MM", "mph"},
		{"pt-BR", "kmh"},
		{"de-DE", "kmh"},
		{"fr-FR", "kmh"},
		{"en-AU", "kmh"},
	}
	for _, c := range cases {
		if got := DefaultUnit(c.locale); got != c.want {
			t.Errorf("DefaultUnit(%q) = %q, want %q", c.locale, got, c.want)
		}
	}
}

func TestNew(t *testing.T) {
	cases := []struct {
		val  float64
		unit string
	}{
		{30, "ms"}, {108, "kmh"}, {60, "mph"}, {10, "kn"}, {88, "fps"},
	}
	for _, c := range cases {
		s, err := New(c.val, c.unit)
		if err != nil {
			t.Errorf("New(%g, %q): unexpected error: %v", c.val, c.unit, err)
			continue
		}
		got, _, err := s.Convert(c.unit)
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
	if _, err := New(0, "warp"); err == nil {
		t.Error("New with unknown unit: expected error, got nil")
	}
}

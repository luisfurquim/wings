package codec

import "testing"

func TestEncode(t *testing.T) {
	var c Codec
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"empty string", "", ""},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"int", int(-42), "-42"},
		{"int8", int8(-8), "-8"},
		{"int64", int64(9000000000), "9000000000"},
		{"uint", uint(42), "42"},
		{"uint8", uint8(255), "255"},
		{"float32", float32(1.5), "1.5"},
		{"float64", float64(3.14159), "3.14159"},
		{"bytes", []byte("raw"), "raw"},
		{"nil bytes", []byte(nil), ""},
	}
	for _, tc := range cases {
		if got := c.Encode(tc.in); got != tc.want {
			t.Errorf("%s: Encode(%#v) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// Encode falls back to fmt.Sprintf("%v") for types outside the fast-path switch.
func TestEncodeFallback(t *testing.T) {
	var c Codec
	if got := c.Encode([]int{1, 2, 3}); got != "[1 2 3]" {
		t.Errorf("Encode([]int) = %q, want %q", got, "[1 2 3]")
	}
}

func TestDecodeScalars(t *testing.T) {
	var c Codec

	var s string
	if err := c.Decode("hello", &s); err != nil || s != "hello" {
		t.Errorf("Decode string: got (%q, %v)", s, err)
	}

	var b bool
	if err := c.Decode("true", &b); err != nil || !b {
		t.Errorf("Decode bool: got (%v, %v)", b, err)
	}

	var i int
	if err := c.Decode("-42", &i); err != nil || i != -42 {
		t.Errorf("Decode int: got (%d, %v)", i, err)
	}

	var u uint16
	if err := c.Decode("65535", &u); err != nil || u != 65535 {
		t.Errorf("Decode uint16: got (%d, %v)", u, err)
	}

	var f float64
	if err := c.Decode("3.5", &f); err != nil || f != 3.5 {
		t.Errorf("Decode float64: got (%v, %v)", f, err)
	}

	var raw []byte
	if err := c.Decode("bytes", &raw); err != nil || string(raw) != "bytes" {
		t.Errorf("Decode []byte: got (%q, %v)", raw, err)
	}
}

// Round-trip: Encode then Decode must recover the original value.
func TestRoundTrip(t *testing.T) {
	var c Codec
	orig := int64(-1234567)
	var back int64
	if err := c.Decode(c.Encode(orig), &back); err != nil {
		t.Fatalf("round-trip decode: %v", err)
	}
	if back != orig {
		t.Errorf("round-trip: got %d, want %d", back, orig)
	}
}

func TestDecodeErrors(t *testing.T) {
	var c Codec

	// Non-pointer target.
	var i int
	if err := c.Decode("1", i); err == nil {
		t.Error("Decode into non-pointer: expected error, got nil")
	}

	// Nil pointer target.
	var p *int
	if err := c.Decode("1", p); err == nil {
		t.Error("Decode into nil pointer: expected error, got nil")
	}

	// Unparseable values for each numeric kind.
	var b bool
	if err := c.Decode("notabool", &b); err == nil {
		t.Error("Decode bad bool: expected error")
	}
	if err := c.Decode("notanint", &i); err == nil {
		t.Error("Decode bad int: expected error")
	}
	var u uint
	if err := c.Decode("-1", &u); err == nil {
		t.Error("Decode negative into uint: expected error")
	}
	var f float64
	if err := c.Decode("notafloat", &f); err == nil {
		t.Error("Decode bad float: expected error")
	}

	// Overflow: 300 does not fit in int8.
	var i8 int8
	if err := c.Decode("300", &i8); err == nil {
		t.Error("Decode overflow int8: expected error")
	}

	// Unsupported pointer element type (slice of non-bytes).
	var xs []int
	if err := c.Decode("x", &xs); err == nil {
		t.Error("Decode []int: expected unsupported-type error")
	}

	// Unsupported element kind (struct).
	var st struct{ X int }
	if err := c.Decode("x", &st); err == nil {
		t.Error("Decode struct: expected unsupported-type error")
	}
}

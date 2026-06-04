//go:build js && wasm

package wings

import "testing"

type addr struct {
	City string
	zip  string // unexported: must be invisible to getField
}

func TestGetField(t *testing.T) {
	m := map[string]any{"name": "Ana", "age": 30}
	if got := getField(m, "name"); got != "Ana" {
		t.Errorf("map[string]any: got %v", got)
	}
	if got := getField(m, "missing"); got != nil {
		t.Errorf("missing key: got %v, want nil", got)
	}

	// Typed map with string keys via reflection.
	tm := map[string]int{"a": 1}
	if got := getField(tm, "a"); got != 1 {
		t.Errorf("typed map: got %v", got)
	}

	// Struct (value and pointer); only exported fields are reachable.
	a := addr{City: "Rio", zip: "20000"}
	if got := getField(a, "City"); got != "Rio" {
		t.Errorf("struct field: got %v", got)
	}
	if got := getField(&a, "City"); got != "Rio" {
		t.Errorf("struct ptr field: got %v", got)
	}
	if got := getField(a, "zip"); got != nil {
		t.Errorf("unexported field should be nil, got %v", got)
	}

	// nil object and nil pointer.
	if got := getField(nil, "x"); got != nil {
		t.Errorf("nil obj: got %v", got)
	}
	var pa *addr
	if got := getField(pa, "City"); got != nil {
		t.Errorf("nil ptr: got %v", got)
	}
}

func TestSetField(t *testing.T) {
	m := map[string]any{}
	if !setField(m, "k", 42) || m["k"] != 42 {
		t.Errorf("setField on map[string]any failed: %v", m)
	}
	// Non map[string]any targets are not assignable.
	if setField(map[string]int{}, "k", 1) {
		t.Error("setField on typed map should return false")
	}
	if setField("nope", "k", 1) {
		t.Error("setField on non-map should return false")
	}
}

func TestGetAt(t *testing.T) {
	xs := []any{"a", "b", "c"}
	if got := getAt(xs, 1); got != "b" {
		t.Errorf("[]any index 1: got %v", got)
	}
	if got := getAt(xs, "2"); got != "c" { // numeric string index
		t.Errorf("[]any string index: got %v", got)
	}
	if got := getAt(xs, 9); got != nil {
		t.Errorf("out of range: got %v, want nil", got)
	}
	if got := getAt(xs, -1); got != nil {
		t.Errorf("negative index: got %v, want nil", got)
	}

	// Typed slice + array via reflection.
	if got := getAt([]int{10, 20}, 0); got != 10 {
		t.Errorf("typed slice: got %v", got)
	}
	if got := getAt([3]string{"x", "y", "z"}, 2); got != "z" {
		t.Errorf("array: got %v", got)
	}
	if got := getAt(nil, 0); got != nil {
		t.Errorf("nil: got %v", got)
	}
}

func TestToInt(t *testing.T) {
	cases := []struct {
		in   any
		want int
	}{
		{5, 5},
		{int64(7), 7},
		{float64(3.9), 3}, // truncates
		{"42", 42},
		{"notanumber", 0},
		{nil, 0},
		{true, 0}, // unsupported type
	}
	for _, c := range cases {
		if got := toInt(c.in); got != c.want {
			t.Errorf("toInt(%#v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestToStr(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"hi", "hi"},
		{true, "true"},
		{false, "false"},
		{42, "42"},
		{int64(99), "99"},
		{3.5, "3.5"},
		{3.0, "3"}, // -1 precision trims trailing zeros
		{[]any{"a", 1, true}, "a,1,true"},
	}
	for _, c := range cases {
		if got := toStr(c.in); got != c.want {
			t.Errorf("toStr(%#v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCoerceToType(t *testing.T) {
	if got := coerceToType("x", nil); got != "x" {
		t.Errorf("nil existing returns string: got %v", got)
	}
	if got := coerceToType("true", false); got != true {
		t.Errorf("bool coerce: got %v", got)
	}
	if got := coerceToType("123", 0); got != 123 {
		t.Errorf("int coerce: got %v", got)
	}
	if got := coerceToType("12345678901", int64(0)); got != int64(12345678901) {
		t.Errorf("int64 coerce: got %v", got)
	}
	if got := coerceToType("2.5", 0.0); got != 2.5 {
		t.Errorf("float coerce: got %v", got)
	}
	// Unparseable numeric falls back to the raw string.
	if got := coerceToType("notanint", 0); got != "notanint" {
		t.Errorf("bad int falls back to string: got %v", got)
	}
}

func TestSolve(t *testing.T) {
	// nil tree returns the context unchanged.
	if got := Solve(nil, "ctx", nil); got != "ctx" {
		t.Errorf("nil tree: got %v", got)
	}

	data := map[string]any{
		"user": map[string]any{"name": "Bia"},
		"list": []any{"zero", "one", "two"},
		"i":    1,
	}
	ctx := Ctx{data}

	// user.name
	tree := []RefNode{
		{Type: TokIdent, StrVal: "user"},
		{Type: TokIdent, StrVal: "name"},
	}
	if got := Solve(tree, data, ctx); got != "Bia" {
		t.Errorf("user.name: got %v, want Bia", got)
	}

	// list[1]: a numeric index is a TokExpr whose sub-expression is a TokNum
	// literal. (A bare TokNum node just substitutes the integer for sym; only
	// TokExpr performs the getAt index step.)
	tree = []RefNode{
		{Type: TokIdent, StrVal: "list"},
		{Type: TokExpr, Sub: []RefNode{{Type: TokNum, IntVal: 1}}},
	}
	if got := Solve(tree, data, ctx); got != "one" {
		t.Errorf("list[1]: got %v, want one", got)
	}

	// list[i] via TokExpr resolving the index from the context stack.
	tree = []RefNode{
		{Type: TokIdent, StrVal: "list"},
		{Type: TokExpr, Sub: []RefNode{{Type: TokIdent, StrVal: "i"}}},
	}
	if got := Solve(tree, data, ctx); got != "one" {
		t.Errorf("list[i]: got %v, want one", got)
	}

	// {{#}} resolves to the hash fragment (empty under the test DOM shim).
	tree = []RefNode{{Type: TokIdent, StrVal: "#"}}
	if got := Solve(tree, data, ctx); got != hashFragment {
		t.Errorf("{{#}}: got %v, want %q", got, hashFragment)
	}
}

func TestSolveAll(t *testing.T) {
	segs := []TextSegment{
		{IsRef: false, Lit: "Hello, "},
		{IsRef: true, Ref: []RefNode{{Type: TokIdent, StrVal: "name"}}},
		{IsRef: false, Lit: "!"},
	}
	ctx := Ctx{map[string]any{"name": "World"}}
	if got := SolveAll(segs, ctx); got != "Hello, World!" {
		t.Errorf("SolveAll = %q, want %q", got, "Hello, World!")
	}
}

func TestRefOf(t *testing.T) {
	inner := map[string]any{"name": "old"}
	ctx := Ctx{map[string]any{"user": inner}}

	tree := []RefNode{
		{Type: TokIdent, StrVal: "user"},
		{Type: TokIdent, StrVal: "name"},
	}
	container, key := refOf(tree, ctx)
	if key != "name" {
		t.Fatalf("refOf key = %q, want name", key)
	}
	// The returned container must be the inner map so assignment writes through.
	if m, ok := container.(map[string]any); !ok || m["name"] != "old" {
		t.Fatalf("refOf container = %#v, want inner map", container)
	}
	container.(map[string]any)[key] = "new"
	if inner["name"] != "new" {
		t.Errorf("write-through failed: inner=%v", inner)
	}

	// Empty tree / empty ctx return the nil sentinel.
	if c, k := refOf(nil, ctx); c != nil || k != "" {
		t.Errorf("empty tree: got (%v,%q)", c, k)
	}
	if c, k := refOf(tree, nil); c != nil || k != "" {
		t.Errorf("empty ctx: got (%v,%q)", c, k)
	}
}

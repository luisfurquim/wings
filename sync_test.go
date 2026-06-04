//go:build js && wasm

package wings

import "testing"

func TestBuildCtx(t *testing.T) {
	ndx := map[string]any{"i": 0}
	item := []any{map[string]any{"row": 1}, nil, map[string]any{"row": 2}}
	main := Ctx{map[string]any{"root": true}}

	stk := buildCtx(ndx, item, main)
	// ndxMap + 2 non-nil item layers + 1 main layer = 4 (the nil item is dropped).
	if len(stk) != 4 {
		t.Fatalf("buildCtx len = %d, want 4 (stk=%v)", len(stk), stk)
	}
	// Order: ndxMap first, then item layers, then main.
	if m, ok := stk[0].(map[string]any); !ok || m["i"] != 0 {
		t.Errorf("layer 0 should be ndxMap, got %v", stk[0])
	}
	if m, ok := stk[3].(map[string]any); !ok || m["root"] != true {
		t.Errorf("last layer should be main ctx, got %v", stk[3])
	}

	// A nil ndxMap is omitted entirely.
	stk = buildCtx(nil, nil, main)
	if len(stk) != 1 {
		t.Errorf("buildCtx(nil,nil,main) len = %d, want 1", len(stk))
	}
}

func TestEvalCondOp(t *testing.T) {
	cases := []struct {
		name string
		op   string
		val  string
		res  any
		want bool
	}{
		{"truthy default true", "", "", "x", true},
		{"truthy default false", "", "", "", false},
		{"eq match", "eq", "5", 5, true},
		{"eq no match", "eq", "5", 6, false},
		{"neq match", "neq", "5", 6, true},
		{"neq no match", "neq", "5", 5, false},
		{"prefix hit", "prefix", "ab", "abcdef", true},
		{"prefix miss", "prefix", "xy", "abcdef", false},
		{"suffix hit", "suffix", "ef", "abcdef", true},
		{"suffix miss", "suffix", "xy", "abcdef", false},
		{"contains hit", "contains", "cd", "abcdef", true},
		{"contains miss", "contains", "zz", "abcdef", false},
		{"negation of falsy", "!", "", "", true},
		{"negation of truthy", "!", "", "nonempty", false},
	}
	for _, c := range cases {
		ref := &DOMRefNode{CondOp: c.op, CondVal: c.val}
		if got := evalCondOp(c.res, ref); got != c.want {
			t.Errorf("%s: evalCondOp(%#v, op=%q val=%q) = %v, want %v",
				c.name, c.res, c.op, c.val, got, c.want)
		}
	}
}

func TestIsTruthy(t *testing.T) {
	truthy := []any{true, 1, int64(1), 1.5, "x", []any{0}, map[string]any{"k": 0}, struct{}{}}
	falsy := []any{nil, false, 0, int64(0), 0.0, "", []any{}, map[string]any{}}
	for _, v := range truthy {
		if !isTruthy(v) {
			t.Errorf("isTruthy(%#v) = false, want true", v)
		}
	}
	for _, v := range falsy {
		if isTruthy(v) {
			t.Errorf("isTruthy(%#v) = true, want false", v)
		}
	}
}

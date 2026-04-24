package expr

import "testing"

func TestClassifyBlocks(t *testing.T) {
	type want struct{ fmt, flex bool }
	cases := []struct {
		expr string
		want want
	}{
		{"preco", want{false, false}},
		{"user.name", want{false, false}},
		{"%preco", want{true, false}},
		{"%cart[i].total", want{true, false}},
		{"@genero %qt ~aluno", want{false, true}},
		{"@genero %qt #42", want{false, true}},
		{"~aluno ~aprovado", want{false, true}},
		{"#42", want{false, true}},
		{"@genero", want{false, true}},
		{"%qt ~aluno", want{false, true}},
		{"%qt aluno", want{false, true}},
	}
	for _, c := range cases {
		toks := Tokenize(c.expr)
		gotFmt := IsFmtBlock(toks)
		gotFlex := IsFlexBlock(toks)
		if gotFmt != c.want.fmt || gotFlex != c.want.flex {
			t.Errorf("%q: got fmt=%v flex=%v, want fmt=%v flex=%v",
				c.expr, gotFmt, gotFlex, c.want.fmt, c.want.flex)
		}
	}
}

func TestParseFmtBlock(t *testing.T) {
	cases := []struct {
		expr    string
		wantVar string
		wantLen int // len(Path): 0=bare, >0=path with root+tail
	}{
		{"%preco", "preco", 0},
		{"%user.salary", "user", 2},
		{"%cart[0].total", "cart", 3},
	}
	for _, c := range cases {
		toks := Tokenize(c.expr)
		if !IsFmtBlock(toks) {
			t.Errorf("%q: IsFmtBlock=false, expected true", c.expr)
			continue
		}
		fb, err := ParseFmtBlock(&toks)
		if err != nil {
			t.Errorf("%q: ParseFmtBlock: %v", c.expr, err)
			continue
		}
		if fb.Var != c.wantVar {
			t.Errorf("%q: Var=%q, want %q", c.expr, fb.Var, c.wantVar)
		}
		if len(fb.Path) != c.wantLen {
			t.Errorf("%q: len(Path)=%d, want %d", c.expr, len(fb.Path), c.wantLen)
		}
	}
}

func TestParseFmtBlockRejectsFlex(t *testing.T) {
	toks := Tokenize("%qt ~aluno")
	if _, err := ParseFmtBlock(&toks); err == nil {
		t.Error("ParseFmtBlock on FlexBlock tokens: expected error, got nil")
	}
}

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
		{"%preco:compact", want{true, false}},
		{"%dist:km", want{true, false}},
		{"%user.salary:usd", want{true, false}},
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
		expr       string
		wantVar    string
		wantLen    int // len(Path): 0=bare, >0=path with root+tail
		wantFormat string
	}{
		{"%preco", "preco", 0, ""},
		{"%user.salary", "user", 2, ""},
		{"%cart[0].total", "cart", 3, ""},
		{"%preco:compact", "preco", 0, "compact"},
		{"%dist:km", "dist", 0, "km"},
		{"%user.salary:usd", "user", 2, "usd"},
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
		if fb.FormatName != c.wantFormat {
			t.Errorf("%q: FormatName=%q, want %q", c.expr, fb.FormatName, c.wantFormat)
		}
	}
}

func TestParseFmtBlockRejectsFlex(t *testing.T) {
	toks := Tokenize("%qt ~aluno")
	if _, err := ParseFmtBlock(&toks); err == nil {
		t.Error("ParseFmtBlock on FlexBlock tokens: expected error, got nil")
	}
}

func TestTokenizeCustomSigils(t *testing.T) {
	cases := []struct {
		expr     string
		wantType TokenType
		wantVal  string
	}{
		{"*foo", TokStarVar, "foo"},
		{"$foo", TokDollarVar, "foo"},
		{"~$foo", TokFlexBind, "foo"},
		{"**foo", TokStr, "*foo"}, // escape → literal
		{"$$foo", TokStr, "$foo"}, // escape → literal
	}
	for _, c := range cases {
		toks := Tokenize(c.expr)
		if len(toks) != 1 || toks[0].Type != c.wantType || toks[0].StrVal != c.wantVal {
			t.Errorf("%q: got %+v, want type=%d val=%q", c.expr, toks, c.wantType, c.wantVal)
		}
	}
}

func TestClassifyCustomSigils(t *testing.T) {
	for _, e := range []string{
		"*motor ~aluno",
		"$item",
		"*item",
		"~$item",
		"@g %count *motor ~$dyn",
		"~$itens[i].type",
	} {
		toks := Tokenize(e)
		if !IsFlexBlock(toks) {
			t.Errorf("%q: IsFlexBlock=false, want true", e)
		}
		if IsFmtBlock(toks) {
			t.Errorf("%q: IsFmtBlock=true, want false", e)
		}
	}
}

func TestParseFlexBlockCustomSigils(t *testing.T) {
	toks := Tokenize("@gender %count ~o ~aluno *motor[i].kind $item ~$adj.form")
	fb, err := ParseFlexBlock(&toks)
	if err != nil {
		t.Fatalf("ParseFlexBlock: %v", err)
	}
	if fb.GenderVar != "gender" || fb.CountVar != "count" {
		t.Errorf("gender/count = %q/%q", fb.GenderVar, fb.CountVar)
	}
	if len(fb.TildeWords) != 2 {
		t.Errorf("TildeWords = %v, want 2", fb.TildeWords)
	}
	if len(fb.StarVars) != 1 || fb.StarVars[0].Var != "motor" || len(fb.StarVars[0].Path) == 0 {
		t.Errorf("StarVars = %+v (want one 'motor' with path)", fb.StarVars)
	}
	if len(fb.DollarVars) != 1 || fb.DollarVars[0].Var != "item" || fb.DollarVars[0].Path != nil {
		t.Errorf("DollarVars = %+v (want one bare 'item')", fb.DollarVars)
	}
	if len(fb.FlexBinds) != 1 || fb.FlexBinds[0].Var != "adj" || len(fb.FlexBinds[0].Path) == 0 {
		t.Errorf("FlexBinds = %+v (want one 'adj' with path)", fb.FlexBinds)
	}
}

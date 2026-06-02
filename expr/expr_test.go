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

func TestTokenizeFlexContent(t *testing.T) {
	type tok struct {
		typ TokenType
		val string
	}
	cases := []struct {
		name string
		in   string
		want []tok
	}{
		{
			name: "pt with collapsing whitespace and tab",
			in:   "O usuário $nome comprou\t  $produto",
			want: []tok{
				{TokTxt, "O"}, {TokSpace, " "}, {TokTxt, "usuário"}, {TokSpace, " "},
				{TokDollarVar, "nome"}, {TokSpace, " "}, {TokTxt, "comprou"}, {TokSpace, " "},
				{TokDollarVar, "produto"},
			},
		},
		{
			name: "japanese, no spaces",
			in:   "$produtoを$nomeが購入しました",
			want: []tok{
				{TokDollarVar, "produto"}, {TokTxt, "を"},
				{TokDollarVar, "nome"}, {TokTxt, "が購入しました"},
			},
		},
		{
			name: "trailing period stays literal (not a path)",
			in:   "comprou $produto.",
			want: []tok{
				{TokTxt, "comprou"}, {TokSpace, " "}, {TokDollarVar, "produto"}, {TokTxt, "."},
			},
		},
		{
			name: "price: lone $ before digit is literal",
			in:   "US$ 5",
			want: []tok{{TokTxt, "US$"}, {TokSpace, " "}, {TokTxt, "5"}},
		},
	}
	for _, c := range cases {
		toks := TokenizeFlexContent(c.in)
		if len(toks) != len(c.want) {
			t.Errorf("%s: got %d tokens %+v, want %d", c.name, len(toks), toks, len(c.want))
			continue
		}
		for i, w := range c.want {
			if toks[i].Type != w.typ || toks[i].StrVal != w.val {
				t.Errorf("%s: tok[%d]={type:%d val:%q}, want {type:%d val:%q}",
					c.name, i, toks[i].Type, toks[i].StrVal, w.typ, w.val)
			}
		}
	}
}

func TestTokenizeFlexContentPath(t *testing.T) {
	toks := TokenizeFlexContent(" vendeu $user.name e $cart[i].qty itens")
	// find the two dollar vars and check their paths
	var dollars []RefNode
	for _, tk := range toks {
		if tk.Type == TokDollarVar {
			dollars = append(dollars, tk)
		}
	}
	if len(dollars) != 2 {
		t.Fatalf("want 2 dollar vars, got %d (%+v)", len(dollars), toks)
	}
	if dollars[0].StrVal != "user" || len(dollars[0].Sub) != 2 {
		t.Errorf("dollars[0]=%+v, want user with 2-node path", dollars[0])
	}
	if dollars[1].StrVal != "cart" || len(dollars[1].Sub) != 3 {
		t.Errorf("dollars[1]=%+v, want cart with 3-node path", dollars[1])
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

// Literal punctuation in a programmable block (e.g. a colon) must not abort the
// parse: ParseFlexBlock only extracts control sigils, so punctuation is kept as
// literal content. Regression for the `:` that used to error (TokColon) and
// silently leave the whole block un-rewritten.
func TestParseFlexBlockLiteralPunctuation(t *testing.T) {
	for _, src := range []string{
		"*help Copy on $platform: ~$action",
		"%qt ~aluno aprovado: nota.final",
	} {
		toks := Tokenize(src)
		fb, err := ParseFlexBlock(&toks)
		if err != nil {
			t.Fatalf("ParseFlexBlock(%q): unexpected error %v", src, err)
		}
		// Control sigils are still extracted correctly past the punctuation.
		if src[0] == '*' && (len(fb.StarVars) != 1 || fb.StarVars[0].Var != "help") {
			t.Errorf("%q: StarVars = %+v (want one 'help')", src, fb.StarVars)
		}
	}
}

// Reuse sigils: `=name` (define), `#name` (use). `#N` stays a numeric index;
// `==` escapes to a literal `=`.
func TestTokenizeReuseSigils(t *testing.T) {
	cases := []struct {
		expr     string
		wantType TokenType
		wantVal  string
		wantInt  int
	}{
		{"=greeting", TokDefName, "greeting", 0},
		{"#greeting", TokFlexName, "greeting", 0},
		{"#42", TokFlexIdx, "", 42},
		{"==eq", TokStr, "=", 0}, // escape → literal '='
	}
	for _, c := range cases {
		toks := Tokenize(c.expr)
		if c.wantType == TokStr {
			// `==eq` yields the literal '=' then the identifier `eq`.
			if len(toks) < 1 || toks[0].Type != TokStr || toks[0].StrVal != "=" {
				t.Errorf("%q: got %+v, want leading TokStr %q", c.expr, toks, "=")
			}
			continue
		}
		if len(toks) != 1 || toks[0].Type != c.wantType || toks[0].StrVal != c.wantVal || toks[0].IntVal != c.wantInt {
			t.Errorf("%q: got %+v, want type=%d val=%q int=%d", c.expr, toks, c.wantType, c.wantVal, c.wantInt)
		}
	}
	for _, e := range []string{"=greeting ~$nome", "#greeting %qt"} {
		if !IsFlexBlock(Tokenize(e)) {
			t.Errorf("%q: IsFlexBlock=false, want true", e)
		}
	}
}

// In flex content, `=name`/`#name` are recognized as control tokens (so the
// build can strip them), a lone `=` stays literal, and `==` escapes.
func TestTokenizeFlexContentReuse(t *testing.T) {
	type tok struct {
		typ TokenType
		val string
	}
	cases := []struct {
		name string
		in   string
		want []tok
	}{
		{
			name: "define name then content",
			in:   "=greeting Olá ~$nome",
			want: []tok{
				{TokDefName, "greeting"}, {TokSpace, " "}, {TokTxt, "Olá"},
				{TokSpace, " "}, {TokFlexBind, "nome"},
			},
		},
		{
			name: "lone equals stays literal",
			in:   "a = b",
			want: []tok{
				{TokTxt, "a"}, {TokSpace, " "}, {TokTxt, "="}, {TokSpace, " "}, {TokTxt, "b"},
			},
		},
		{
			name: "doubled equals escapes",
			in:   "x ==y",
			want: []tok{
				{TokTxt, "x"}, {TokSpace, " "}, {TokTxt, "=y"},
			},
		},
	}
	for _, c := range cases {
		toks := TokenizeFlexContent(c.in)
		if len(toks) != len(c.want) {
			t.Errorf("%s: got %d tokens %+v, want %d", c.name, len(toks), toks, len(c.want))
			continue
		}
		for i, w := range c.want {
			if toks[i].Type != w.typ || toks[i].StrVal != w.val {
				t.Errorf("%s: tok[%d]={type:%d val:%q}, want {type:%d val:%q}",
					c.name, i, toks[i].Type, toks[i].StrVal, w.typ, w.val)
			}
		}
	}
}

// ParseFlexBlock records =name/#name and enforces one of each per block.
func TestParseFlexBlockReuse(t *testing.T) {
	toks := Tokenize("=greeting *motor @g ~$nome")
	fb, err := ParseFlexBlock(&toks)
	if err != nil {
		t.Fatalf("ParseFlexBlock define: %v", err)
	}
	if fb.DefName != "greeting" {
		t.Errorf("DefName = %q, want greeting", fb.DefName)
	}

	toks = Tokenize("#greeting %qt")
	fb, err = ParseFlexBlock(&toks)
	if err != nil {
		t.Fatalf("ParseFlexBlock use: %v", err)
	}
	if fb.RefName != "greeting" || fb.CountVar != "qt" {
		t.Errorf("RefName/CountVar = %q/%q, want greeting/qt", fb.RefName, fb.CountVar)
	}

	for _, bad := range []string{"=a =b", "#a #b"} {
		toks = Tokenize(bad)
		if _, err := ParseFlexBlock(&toks); err == nil {
			t.Errorf("%q: ParseFlexBlock should reject a second reuse name", bad)
		}
	}
}

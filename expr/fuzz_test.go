package expr

import "testing"

// FuzzTokenizeFlexContent feeds arbitrary strings to the flex-content tokenizer.
// Property: it never panics — any webdev-authored sentence, however malformed,
// must at worst produce odd tokens, never crash the build/runtime.
func FuzzTokenizeFlexContent(f *testing.F) {
	for _, s := range []string{
		"O usuário $nome comprou $produto",
		"$produtoを$nomeが購入しました",
		"US$ 5",
		"vendeu $user.name e $cart[i].qty itens",
		"",
		"$",
		"$$$$....[[[[",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_ = TokenizeFlexContent(s)
	})
}

// FuzzTokenizeAndParseFlexBlock exercises the full control-block pipeline
// (Tokenize -> ParseFlexBlock). Property: never panics; an accepted block is
// returned without error, a malformed one returns an error, neither crashes.
func FuzzTokenizeAndParseFlexBlock(f *testing.F) {
	for _, s := range []string{
		"@gender %count ~o ~aluno",
		"%qt *flexer ~$produto",
		"=greeting *motor @g ~$nome",
		"@gender %qt #0",
		"*help Copy on $platform: ~$action",
		"",
		"@@@@ %%%%",
		"{{{{",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		toks := Tokenize(s)
		_, _ = ParseFlexBlock(&toks)
	})
}

// FuzzTokenizeAndParseReference exercises the reference parser (variable paths,
// indexes). Property: never panics.
func FuzzTokenizeAndParseReference(f *testing.F) {
	for _, s := range []string{
		"%qt ~aluno",
		"user.name",
		"cart[i].qty",
		"a.b.c.d.e",
		"",
		"...[].[",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		toks := Tokenize(s)
		_, _ = ParseReference(&toks)
	})
}

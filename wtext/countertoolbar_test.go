package wtext

import "testing"

func TestCount(t *testing.T) {
	cases := []struct {
		name                  string
		text                  string
		chars, letters, words int
	}{
		{"empty", "", 0, 0, 0},
		{"plain", "Olá, mundo!", 11, 8, 2},
		{"blocks do not count as chars", "um\ndois", 6, 6, 2},
		{"digits and punctuation are not letters", "abc 123", 7, 3, 2},
		{"emoji is a char, not a letter", "a😀 b", 4, 2, 2},
		{"spaces count as chars", "  ", 2, 0, 0},
		{"standalone punctuation is not a word", "olá , mundo", 11, 8, 2},
		{"lone dash is not a word", "um — dois", 9, 6, 2},
		{"currency sign with digits is a word", "R$ 100", 6, 1, 2},
		{"only punctuation", "... — !?", 8, 0, 0},
	}
	for _, c := range cases {
		chars, letters, words := Count(c.text)
		if chars != c.chars || letters != c.letters || words != c.words {
			t.Errorf("%s: Count(%q) = (%d, %d, %d), want (%d, %d, %d)",
				c.name, c.text, chars, letters, words, c.chars, c.letters, c.words)
		}
	}
}

func TestCounterToolbar(t *testing.T) {
	items := CounterToolbar{}.Items()
	if len(items) != 1 {
		t.Fatalf("CounterToolbar declares %d items, want 1", len(items))
	}
	st, ok := items[0].(StatusItem)
	if !ok {
		t.Fatalf("item is %T, want StatusItem", items[0])
	}
	if st.ID == "" || st.Label == "" || st.Format == "" || st.Help == "" || st.Args == nil {
		t.Error("StatusItem fields incomplete")
	}
	core := &fakeCore{docText: "duas palavras\nem dois blocos"}
	args := st.Args(core)
	if len(args) != 3 {
		t.Fatalf("Args returned %d values, want 3", len(args))
	}
	if args[0] != 27 || args[1] != 24 || args[2] != 5 {
		t.Errorf("Args = %v, want [27 24 5]", args)
	}
}

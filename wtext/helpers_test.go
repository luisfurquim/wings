package wtext

import "testing"

// fakeCore is a minimal EditorCore for exercising the portable helpers
// and BasicToolbar natively — exactly what the portable/mechanism split
// promises plugin authors.
type fakeCore struct {
	sel    Selection
	hasSel bool
	marked map[string]bool // tag → selection fully inside the mark
	block  string
	calls  []string
}

func (f *fakeCore) Sel() (Selection, bool) { return f.sel, f.hasSel }
func (f *fakeCore) Text(Selection) (string, error) {
	return "", nil
}
func (f *fakeCore) InMark(_ Selection, tag string) (bool, error) {
	return f.marked[tag], nil
}
func (f *fakeCore) BlockType(Selection) (string, error) { return f.block, nil }
func (f *fakeCore) HasClass(Selection, string) (bool, error) {
	return false, nil
}
func (f *fakeCore) Wrap(_ Selection, m Mark) error {
	f.calls = append(f.calls, "wrap:"+m.Tag())
	return nil
}
func (f *fakeCore) Unwrap(_ Selection, tag string) error {
	f.calls = append(f.calls, "unwrap:"+tag)
	return nil
}
func (f *fakeCore) SetBlock(_ Selection, tag string) error {
	f.calls = append(f.calls, "setblock:"+tag)
	return nil
}
func (f *fakeCore) ApplyClass(Selection, string) error  { return nil }
func (f *fakeCore) RemoveClass(Selection, string) error { return nil }
func (f *fakeCore) DefineClass(string, string) error    { return nil }
func (f *fakeCore) Replace(Selection, Fragment) error   { return nil }
func (f *fakeCore) Delete(Selection) error              { return nil }
func (f *fakeCore) Txn(fn func(EditorCore) error) error { return fn(f) }

func TestToggleMarkWordBehaviour(t *testing.T) {
	// Not (fully) marked: toggling marks — the partially-bold selection
	// becomes fully bold, never the reverse.
	core := &fakeCore{hasSel: true, marked: map[string]bool{}}
	if err := ToggleMark(core, Strong()); err != nil {
		t.Fatal(err)
	}
	// Fully marked: toggling unmarks.
	core.marked["strong"] = true
	if err := ToggleMark(core, Strong()); err != nil {
		t.Fatal(err)
	}
	if len(core.calls) != 2 || core.calls[0] != "wrap:strong" || core.calls[1] != "unwrap:strong" {
		t.Errorf("calls = %v, want [wrap:strong unwrap:strong]", core.calls)
	}
}

func TestToggleMarkNoSelection(t *testing.T) {
	core := &fakeCore{hasSel: false}
	if err := ToggleMark(core, Em()); err != nil {
		t.Fatal(err)
	}
	if len(core.calls) != 0 {
		t.Errorf("acted without a selection: %v", core.calls)
	}
}

func TestMarkActive(t *testing.T) {
	core := &fakeCore{hasSel: true, marked: map[string]bool{"em": true}}
	if !MarkActive("em")(core) {
		t.Error("active mark reported inactive")
	}
	if MarkActive("strong")(core) {
		t.Error("inactive mark reported active")
	}
	core.hasSel = false
	if MarkActive("em")(core) {
		t.Error("active without selection")
	}
}

func TestBlockHelpers(t *testing.T) {
	core := &fakeCore{hasSel: true, block: "h2"}
	if got := BlockCurrent()(core); got != "h2" {
		t.Errorf("BlockCurrent = %q, want h2", got)
	}
	if err := BlockPick()(core, "blockquote"); err != nil {
		t.Fatal(err)
	}
	if len(core.calls) != 1 || core.calls[0] != "setblock:blockquote" {
		t.Errorf("calls = %v", core.calls)
	}
}

func TestBasicToolbarShape(t *testing.T) {
	items := BasicToolbar{}.Items()
	if len(items) != 5 {
		t.Fatalf("items = %d, want 5", len(items))
	}
	toggles := 0
	selects := 0
	for _, it := range items {
		switch item := it.(type) {
		case ToggleItem:
			toggles++
			if item.Do == nil || item.Active == nil || item.Label == "" {
				t.Errorf("toggle %q incomplete", item.ID)
			}
		case SelectItem:
			selects++
			opts := item.Options(nil)
			if len(opts) != 9 { // p, h1..h6, blockquote, pre
				t.Errorf("block picker has %d options, want 9", len(opts))
			}
		}
	}
	if toggles != 3 || selects != 1 {
		t.Errorf("toggles=%d selects=%d, want 3 and 1", toggles, selects)
	}
}

func TestProfileRegistry(t *testing.T) {
	RegisterProfile("test-prof", Profile{Toolbar: []ToolbarPlugin{BasicToolbar{}}})
	p, ok := ProfileFor("test-prof")
	if !ok || len(p.Toolbar) != 1 {
		t.Errorf("registered profile not found")
	}
	if _, ok = ProfileFor("no-such"); ok {
		t.Error("unknown profile found")
	}
}

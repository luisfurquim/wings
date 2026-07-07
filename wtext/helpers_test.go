package wtext

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/luisfurquim/wings/epubhtml"
)

// fakeCore is a minimal EditorCore for exercising the portable helpers
// and the stock toolbars natively — exactly what the portable/mechanism
// split promises plugin authors.
type fakeCore struct {
	sel     Selection
	hasSel  bool
	marked  map[string]bool // tag → selection fully inside the mark
	block   string
	classes map[string]string // DefineClass registry
	at      []string          // classes in effect at the selection
	calls   []string
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
func (f *fakeCore) ApplyClass(_ Selection, name string) error {
	f.calls = append(f.calls, "apply:"+name)
	return nil
}
func (f *fakeCore) RemoveClass(_ Selection, name string) error {
	f.calls = append(f.calls, "remove:"+name)
	return nil
}
func (f *fakeCore) DefineClass(name, css string) error {
	if err := epubhtml.ValidClassName(name); err != nil {
		return err
	}
	clean, err := epubhtml.SanitizeCSS(css)
	if err != nil {
		return err
	}
	if f.classes == nil {
		f.classes = map[string]string{}
	}
	f.classes[name] = clean
	f.calls = append(f.calls, "define:"+name)
	return nil
}
func (f *fakeCore) Classes() []string {
	names := make([]string, 0, len(f.classes))
	for name := range f.classes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func (f *fakeCore) ClassCSS(name string) (string, bool) {
	css, ok := f.classes[name]
	return css, ok
}
func (f *fakeCore) ClassesAt(Selection) ([]string, error) { return f.at, nil }
func (f *fakeCore) Replace(Selection, Fragment) error     { return nil }
func (f *fakeCore) Delete(Selection) error                { return nil }
func (f *fakeCore) Txn(fn func(EditorCore) error) error   { return fn(f) }

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

func TestSwapClassExclusive(t *testing.T) {
	core := &fakeCore{hasSel: true}
	for _, def := range []string{"wt-ff-serif", "wt-ff-sans", "wt-fs-150"} {
		if err := core.DefineClass(def, "color: red"); err != nil {
			t.Fatal(err)
		}
	}
	core.calls = nil
	if err := SwapClass(core, "wt-ff-", "wt-ff-sans"); err != nil {
		t.Fatal(err)
	}
	// serif (same family) leaves, the size family is untouched, sans lands.
	want := []string{"remove:wt-ff-serif", "apply:wt-ff-sans"}
	if strings.Join(core.calls, ",") != strings.Join(want, ",") {
		t.Errorf("calls = %v, want %v", core.calls, want)
	}
	// The empty name only clears the family.
	core.calls = nil
	if err := SwapClass(core, "wt-ff-", ""); err != nil {
		t.Fatal(err)
	}
	want = []string{"remove:wt-ff-sans", "remove:wt-ff-serif"}
	if strings.Join(core.calls, ",") != strings.Join(want, ",") {
		t.Errorf("clear calls = %v, want %v", core.calls, want)
	}
}

func TestClassToggleAndActive(t *testing.T) {
	core := &fakeCore{hasSel: true, at: []string{"wt-al-center"}}
	if err := core.DefineClass("wt-al-center", "text-align: center"); err != nil {
		t.Fatal(err)
	}
	if !ClassActive("wt-al-center")(core) {
		t.Error("active class reported inactive")
	}
	if ClassActive("wt-al-right")(core) {
		t.Error("inactive class reported active")
	}
	// Toggling the active alignment clears the family.
	core.calls = nil
	if err := ClassToggle("wt-al-", "wt-al-center")(core); err != nil {
		t.Fatal(err)
	}
	if strings.Join(core.calls, ",") != "remove:wt-al-center" {
		t.Errorf("toggle-off calls = %v", core.calls)
	}
	// Without a selection nothing acts or lights.
	core.hasSel = false
	if ClassActive("wt-al-center")(core) {
		t.Error("active without selection")
	}
}

func TestClassCurrent(t *testing.T) {
	core := &fakeCore{hasSel: true, at: []string{"titulo", "wt-ff-serif", "wt-fs-150"}}
	if got := ClassCurrent("wt-ff-")(core); got != "serif" {
		t.Errorf("ClassCurrent(wt-ff-) = %q, want serif", got)
	}
	if got := ClassCurrent("wt-al-")(core); got != "" {
		t.Errorf("ClassCurrent(wt-al-) = %q, want empty", got)
	}
}

func TestFontToolbarInit(t *testing.T) {
	core := &fakeCore{}
	if err := (FontToolbar{}).Init(core); err != nil {
		t.Fatal(err)
	}
	// Every default face, size and alignment landed in the registry with
	// its axis property.
	for name, wantProp := range map[string]string{
		"wt-ff-serif":   "font-family",
		"wt-fs-150":     "font-size",
		"wt-al-justify": "text-align",
	} {
		css, ok := core.ClassCSS(name)
		if !ok || !strings.HasPrefix(css, wantProp+":") {
			t.Errorf("class %q = %q, %v; want %s declaration", name, css, ok, wantProp)
		}
	}
	// A bad face fails loudly at Init, not silently at pick time.
	bad := FontToolbar{Faces: []FontFace{{ID: "x", Label: "x", Family: "serif} body{"}}}
	if err := bad.Init(&fakeCore{}); err == nil {
		t.Error("hostile font-family accepted")
	}
}

func TestFontToolbarShape(t *testing.T) {
	items := FontToolbar{}.Items()
	if len(items) != 7 { // 2 selects, separator, 4 alignment toggles
		t.Fatalf("items = %d, want 7", len(items))
	}
	sel, okSel := items[0].(SelectItem)
	if !okSel || len(sel.Options(nil)) != len(DefaultFontFaces())+1 {
		t.Errorf("font picker misses the default option")
	}
	left, okLeft := items[3].(ToggleItem)
	if !okLeft || left.ID != "align-left" {
		t.Fatalf("items[3] = %#v, want the left toggle", items[3])
	}
	// Default alignment lights the left toggle; an explicit center moves it.
	core := &fakeCore{hasSel: true}
	if !left.Active(core) {
		t.Error("left not active on unaligned block")
	}
	core.at = []string{"wt-al-center"}
	if left.Active(core) {
		t.Error("left active under explicit center")
	}
	center := items[4].(ToggleItem)
	if !center.Active(core) {
		t.Error("center not active")
	}
}

func TestCreateStyle(t *testing.T) {
	core := &fakeCore{hasSel: true, at: []string{"wt-ff-serif", "wt-fs-150"}}
	if err := core.DefineClass("wt-ff-serif", "font-family: serif"); err != nil {
		t.Fatal(err)
	}
	if err := core.DefineClass("wt-fs-150", "font-size: 1.5em"); err != nil {
		t.Fatal(err)
	}
	core.calls = nil
	if err := CreateStyle(core, " titulo "); err != nil {
		t.Fatal(err)
	}
	css, ok := core.ClassCSS("titulo")
	if !ok || css != "font-family: serif; font-size: 1.5em" {
		t.Errorf("titulo = %q, %v", css, ok)
	}
	// The source selection swapped utilities for the style.
	want := "define:titulo,remove:wt-ff-serif,remove:wt-fs-150,apply:titulo"
	if strings.Join(core.calls, ",") != want {
		t.Errorf("calls = %v, want %s", core.calls, want)
	}
}

func TestCreateStyleRejections(t *testing.T) {
	core := &fakeCore{hasSel: true, at: []string{"wt-ff-serif"}}
	if err := core.DefineClass("wt-ff-serif", "font-family: serif"); err != nil {
		t.Fatal(err)
	}
	if err := CreateStyle(core, "wt-meu"); !errors.Is(err, ErrReservedClass) {
		t.Errorf("reserved prefix: err = %v", err)
	}
	if err := CreateStyle(core, "1ruim"); err == nil {
		t.Error("invalid name accepted")
	}
	if err := CreateStyle(&fakeCore{hasSel: true}, "vazio"); !errors.Is(err, ErrNoFormatting) {
		t.Errorf("no formatting: err = %v", err)
	}
	if err := CreateStyle(&fakeCore{}, "semsel"); !errors.Is(err, ErrNoSelection) {
		t.Errorf("no selection: err = %v", err)
	}
}

func TestStylePicker(t *testing.T) {
	core := &fakeCore{hasSel: true, at: []string{"wt-ff-serif", "nota"}}
	for name, css := range map[string]string{
		"wt-ff-serif": "font-family: serif",
		"titulo":      "font-size: 2em",
		"nota":        "color: gray",
	} {
		if err := core.DefineClass(name, css); err != nil {
			t.Fatal(err)
		}
	}
	// Options: none + the two named styles, never the utility.
	opts := StyleOptions(core)
	if len(opts) != 3 || opts[0].Value != "" || opts[1].Value != "nota" || opts[2].Value != "titulo" {
		t.Errorf("options = %v", opts)
	}
	if got := StyleCurrent(core); got != "nota" {
		t.Errorf("current = %q, want nota", got)
	}
	// Picking titulo removes nota, keeps direct formatting.
	core.calls = nil
	if err := StylePick(core, "titulo"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(core.calls, ",") != "remove:nota,apply:titulo" {
		t.Errorf("pick calls = %v", core.calls)
	}
	// The none option only clears.
	core.calls = nil
	if err := StylePick(core, ""); err != nil {
		t.Fatal(err)
	}
	if strings.Join(core.calls, ",") != "remove:nota,remove:titulo" {
		t.Errorf("clear calls = %v", core.calls)
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

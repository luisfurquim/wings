package wtext

import (
	"errors"
	"strings"
	"testing"
)

// libFixture is an editor holding two named styles over the stock
// utilities — the shape a user's registry has after a session of
// creating styles.
func libFixture(t *testing.T) *fakeCore {
	t.Helper()
	core := &fakeCore{hasSel: true}
	if err := (FontToolbar{}).Init(core); err != nil {
		t.Fatal(err)
	}
	for name, css := range map[string]string{
		"titulo": "font-family: serif; font-size: 2em; text-align: center",
		"nota":   "color: gray; font-style: italic",
	} {
		if err := core.DefineClass(name, css); err != nil {
			t.Fatal(err)
		}
	}
	return core
}

func TestCollectStyleLibSkipsUtilities(t *testing.T) {
	lib := CollectStyleLib(libFixture(t))
	if len(lib.Styles) != 2 {
		t.Fatalf("styles = %d (%v), want the 2 named ones", len(lib.Styles), lib.Styles)
	}
	for _, s := range lib.Styles {
		if strings.HasPrefix(s.Name, "wt-") {
			t.Errorf("utility class %q exported: the wt- namespace is not the user's", s.Name)
		}
	}
	if lib.Version != StyleLibVersion {
		t.Errorf("version = %d, want %d", lib.Version, StyleLibVersion)
	}
}

// TestStyleLibRoundTrip is the contract of the whole feature: what one
// editor saves, another loads — same names, same CSS.
func TestStyleLibRoundTrip(t *testing.T) {
	data, err := CollectStyleLib(libFixture(t)).JSON()
	if err != nil {
		t.Fatal(err)
	}
	lib, rejected, err := ParseStyleLib(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rejected) != 0 {
		t.Errorf("own output rejected entries: %v", rejected)
	}

	dst := &fakeCore{hasSel: true}
	if err := (StyleLibrary{}).load(dst, data); err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, s := range lib.Styles {
		css, ok := dst.ClassCSS(s.Name)
		if !ok || css != s.CSS {
			t.Errorf("%q = %q, %v; want %q", s.Name, css, ok, s.CSS)
		}
	}
	// Import DEFINES; it never APPLIES: nothing was written to the text.
	for _, call := range dst.calls {
		if strings.HasPrefix(call, "apply:") {
			t.Errorf("import applied a style to the text: %v", dst.calls)
		}
	}
}

func TestStyleLibRejectsHostileEntries(t *testing.T) {
	file := []byte(`{"wtstyles":1,"styles":[
	  {"name":"bom","css":"color: red"},
	  {"name":"wt-b","css":"color: red"},
	  {"name":"1ruim","css":"color: red"},
	  {"name":"exfil","css":"background-color: url(https://evil.example/x)"},
	  {"name":"fuga","css":"color: red } body { display: none"},
	  {"name":"posicao","css":"position: fixed"}
	]}`)
	lib, rejected, err := ParseStyleLib(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Styles) != 1 || lib.Styles[0].Name != "bom" {
		t.Fatalf("styles = %v, want only [bom]", lib.Styles)
	}
	if len(rejected) != 5 {
		t.Errorf("rejected = %v, want the 5 bad entries named", rejected)
	}
}

func TestStyleLibStructuralFailures(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"not json", []byte("nope"), ErrStyleLib},
		{"too big", make([]byte, MaxStyleLibLen+1), ErrStyleLib},
		{"no version", []byte(`{"styles":[{"name":"a","css":"color: red"}]}`), ErrStyleLibVersion},
		{"future version", []byte(`{"wtstyles":99,"styles":[]}`), ErrStyleLibVersion},
		{"nothing usable", []byte(`{"wtstyles":1,"styles":[{"name":"wt-x","css":"color: red"}]}`), ErrStyleLibEmpty},
	}
	for _, c := range cases {
		if _, _, err := ParseStyleLib(c.data); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

// TestStyleLibIgnoresUnknownFields keeps the format forward-compatible: a
// file written by a later build still loads what this one understands.
func TestStyleLibIgnoresUnknownFields(t *testing.T) {
	file := []byte(`{"wtstyles":1,"whatsnext":{"a":1},
	  "styles":[{"name":"bom","css":"color: red","tint":"warm"}]}`)
	lib, _, err := ParseStyleLib(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Styles) != 1 || lib.Styles[0].CSS != "color: red" {
		t.Errorf("styles = %v", lib.Styles)
	}
}

// TestStyleLibBoundsStyles guards the file bound: a runaway file loads its
// first MaxLibStyles entries and no more.
func TestStyleLibBoundsStyles(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"wtstyles":1,"styles":[`)
	for i := 0; i < MaxLibStyles+10; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"name":"s`)
		sb.WriteString(strings.Repeat("x", 1))
		sb.WriteString(itoa(i))
		sb.WriteString(`","css":"color: red"}`)
	}
	sb.WriteString(`]}`)
	lib, rejected, err := ParseStyleLib([]byte(sb.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Styles) != MaxLibStyles {
		t.Errorf("styles = %d, want the %d bound", len(lib.Styles), MaxLibStyles)
	}
	if len(rejected) == 0 {
		t.Error("the dropped overflow was not reported")
	}
}

// TestStyleLibFontsNeedAnAllowedStore is the file's other trust boundary:
// a font reference is followed only if the hard-coded store allowlist
// recognizes its URL — a file cannot introduce an origin.
func TestStyleLibFontsNeedAnAllowedStore(t *testing.T) {
	file := []byte(`{"wtstyles":1,
	  "styles":[{"name":"bom","css":"color: red"}],
	  "fonts":[
	    {"family":"Lobster","url":"https://fonts.googleapis.com/css2?family=Lobster"},
	    {"family":"Evil","url":"https://evil.example/css2?family=Evil"},
	    {"family":"Insecure","url":"http://fonts.googleapis.com/css2?family=X"}
	  ]}`)
	lib, rejected, err := ParseStyleLib(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Fonts) != 1 || lib.Fonts[0].Family != "Lobster" {
		t.Fatalf("fonts = %v, want only the store one", lib.Fonts)
	}
	if len(rejected) != 2 {
		t.Errorf("rejected = %v, want the 2 off-store references named", rejected)
	}
}

// TestStyleLibCollisionAsksTheUser: a name already taken is not
// overwritten behind the user's back — the plugin hands the choice back
// as a PendingDecision, and only Resume("overwrite") replaces the style.
func TestStyleLibCollisionAsksTheUser(t *testing.T) {
	file := []byte(`{"wtstyles":1,"styles":[
	  {"name":"titulo","css":"color: red"},
	  {"name":"novo","css":"color: blue"}
	]}`)
	core := &fakeCore{hasSel: true}
	if err := core.DefineClass("titulo", "font-size: 2em"); err != nil {
		t.Fatal(err)
	}
	err := (StyleLibrary{}).load(core, file)

	var pd *PendingDecision
	if !errors.As(err, &pd) {
		t.Fatalf("err = %v, want a PendingDecision", err)
	}
	if !pd.Valid() || pd.Remember != StyleLibCollisionKey {
		t.Errorf("decision = %#v, want a valid one keyed for remembering", pd)
	}
	if len(pd.Detail) != 1 || pd.Detail[0] != "titulo" {
		t.Errorf("detail = %v, want the colliding name", pd.Detail)
	}
	// A remembered answer is input, not state: only declared options pass.
	if pd.Allows("rm -rf") || !pd.Allows("overwrite") || !pd.Allows("skip") {
		t.Error("Allows does not gate the answers to the declared options")
	}
	// The free name landed already; the taken one still holds its old CSS.
	if css, _ := core.ClassCSS("novo"); css != "color: blue" {
		t.Errorf("novo = %q, want it defined right away", css)
	}
	if css, _ := core.ClassCSS("titulo"); css != "font-size: 2em" {
		t.Errorf("titulo = %q, want it untouched until the user answers", css)
	}

	// Skip: the existing style stands.
	if err := pd.Resume(core, "skip"); err != nil {
		t.Fatal(err)
	}
	if css, _ := core.ClassCSS("titulo"); css != "font-size: 2em" {
		t.Errorf("titulo = %q after skip, want it unchanged", css)
	}
	// Overwrite: the file's version wins.
	if err := pd.Resume(core, "overwrite"); err != nil {
		t.Fatal(err)
	}
	if css, _ := core.ClassCSS("titulo"); css != "color: red" {
		t.Errorf("titulo = %q after overwrite, want the file's CSS", css)
	}
}

func TestStyleLibNothingToExport(t *testing.T) {
	core := &fakeCore{}
	if err := (FontToolbar{}).Init(core); err != nil { // utilities only
		t.Fatal(err)
	}
	if err := (StyleLibrary{}).save(core, "styles"); !errors.Is(err, ErrNoStyles) {
		t.Errorf("err = %v, want ErrNoStyles", err)
	}
}

func TestLibFilename(t *testing.T) {
	for in, want := range map[string]string{
		"meus estilos":  "meus-estilos.json",
		"  Livro 2  ":   "livro-2.json",
		"a/b\\c":        "a-b-c.json",
		"":              "styles.json",
		"...":           "styles.json",
		"Ação & Coisas": "a-o-coisas.json",
	} {
		if got := LibFilename(in); got != want {
			t.Errorf("LibFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestStyleLibraryMenuShape checks the plugin declares both directions
// with help, under the standard groups.
func TestStyleLibraryMenuShape(t *testing.T) {
	items := StyleLibrary{}.MenuItems()
	if len(items) != 2 {
		t.Fatalf("items = %d, want save and load", len(items))
	}
	save, ok := items[0].(MenuInput)
	if !ok || save.Group != "wtext-export" || save.Help == "" || save.Value(nil) != "styles" {
		t.Errorf("save item = %#v", items[0])
	}
	load, ok := items[1].(MenuUpload)
	if !ok || load.Group != "wtext-import" || load.Help == "" || load.MaxLen != MaxStyleLibLen {
		t.Errorf("load item = %#v", items[1])
	}
	if named := (StyleLibrary{DefaultName: "livro"}).MenuItems()[0].(MenuInput); named.Value(nil) != "livro" {
		t.Errorf("DefaultName ignored: %q", named.Value(nil))
	}
}

// itoa avoids pulling strconv into the test for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

package wtextepub

import "github.com/luisfurquim/wings/wtext"

// fakeCore is an EditorCore with no DOM behind it: enough of a document
// for the import to ask its question and load its answer, so the whole
// menu action is exercised natively. Everything the import does not touch
// answers with the zero value.
type fakeCore struct {
	docText string // what the editor already holds (empty = a clean sheet)
	content string // what SetContent stored
}

func (f *fakeCore) DocText() string { return f.docText }
func (f *fakeCore) Content() string { return f.content }

func (f *fakeCore) SetContent(html string) error {
	f.content = html
	return nil
}

func (f *fakeCore) Sel() (wtext.Selection, bool)                 { return wtext.Selection{}, false }
func (f *fakeCore) Text(wtext.Selection) (string, error)         { return "", nil }
func (f *fakeCore) InMark(wtext.Selection, string) (bool, error) { return false, nil }
func (f *fakeCore) BlockType(wtext.Selection) (string, error)    { return "p", nil }
func (f *fakeCore) HasClass(wtext.Selection, string) (bool, error) {
	return false, nil
}
func (f *fakeCore) ClassSpanned(wtext.Selection, string) (bool, error) {
	return false, nil
}
func (f *fakeCore) Classes() []string                             { return nil }
func (f *fakeCore) ClassCSS(string) (string, bool)                { return "", false }
func (f *fakeCore) ClassesAt(wtext.Selection) ([]string, error)   { return nil, nil }
func (f *fakeCore) Wrap(wtext.Selection, wtext.Mark) error        { return nil }
func (f *fakeCore) Unwrap(wtext.Selection, string) error          { return nil }
func (f *fakeCore) SetBlock(wtext.Selection, string) error        { return nil }
func (f *fakeCore) ApplyClass(wtext.Selection, string) error      { return nil }
func (f *fakeCore) RemoveClass(wtext.Selection, string) error     { return nil }
func (f *fakeCore) DefineClass(string, string) error              { return nil }
func (f *fakeCore) Config(string) string                          { return "" }
func (f *fakeCore) SetConfig(string, string) error                { return nil }
func (f *fakeCore) Replace(wtext.Selection, wtext.Fragment) error { return nil }
func (f *fakeCore) Delete(wtext.Selection) error                  { return nil }
func (f *fakeCore) Txn(fn func(wtext.EditorCore) error) error     { return fn(f) }

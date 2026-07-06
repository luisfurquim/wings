//go:build js && wasm

package wtext

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings/epubhtml"
)

// G is the logger for this module.
var G goose.Alert

// Undo bounds: a step cap and a byte budget over retained strings/nodes,
// whichever trips first (see undostack.go).
const (
	undoMaxSteps = 100
	undoMaxBytes = 2 << 20 // ~2MB
)

// Editor drives one contenteditable root. It implements EditorCore; it is
// created by the w-text widget (or a test) around an element already in
// the document.
type Editor struct {
	root    js.Value // the contenteditable element
	doc     js.Value
	profile Profile

	// handles maps opaque Point handles to DOM nodes. Cleared at the end
	// of every pipeline turn: handles are per-turn capabilities and the
	// js.Value entries must not pin nodes across events.
	handles    map[int]js.Value
	nextHandle int

	// classes holds DefineClass registrations (name → sanitized CSS);
	// styleEl is the <style> the rules are rendered into.
	classes map[string]string
	styleEl js.Value

	undo *undoStack[jsOp]

	// observer is built by hand (not dom.Observe) because the capture
	// pipeline needs takeRecords() for synchronous drains, which the dom
	// package does not expose. Released in Detach.
	observer   js.Value
	observerFn js.Func

	listenerIDs []int64 // dom.AddEvent ids on the root
	selListener int64   // document-level selectionchange (outside root!)

	composing    bool // inside IME composition
	applying     bool // applying undo/redo: drained records are discarded
	inTxn        bool
	txnOps       []jsOp
	inOnChanged  bool
	cascades     int  // native-change cascades during OnChanged dispatch
	changedQueue bool // an OnChanged dispatch is scheduled

	// onSelChange lets the widget refresh toolbar state; nil until set.
	onSelChange func()
}

// maxCascades bounds reactive loops: a plugin whose OnChanged writes can
// trigger more changes; past this many rounds in one flush the editor
// logs and stops dispatching (fail-operational: degrade, don't spin).
const maxCascades = 3

// New attaches an Editor to root, a contenteditable element living in the
// document. The initial content of root is discarded and replaced by an
// empty paragraph; load content with SetContent.
func New(root js.Value, p Profile) (*Editor, error) {
	if !root.Truthy() || root.Get("nodeType").IsUndefined() {
		return nil, fmt.Errorf("wtext: root is not a DOM element")
	}
	e := &Editor{
		root:    root,
		doc:     root.Get("ownerDocument"),
		profile: p,
		handles: map[int]js.Value{},
		classes: map[string]string{},
		undo: newUndoStack[jsOp](undoMaxSteps, undoMaxBytes,
			func(op jsOp) int { return op.bytes() }),
	}
	e.styleEl = e.doc.Call("createElement", "style")
	root.Get("parentNode").Call("insertBefore", e.styleEl, root)
	root.Call("setAttribute", "contenteditable", "true")
	e.clearContent()
	e.wire()
	return e, nil
}

// Detach releases everything the editor wired: listeners, the observer,
// handles and the undo stack. The root stays in the document.
func (e *Editor) Detach() {
	e.unwire()
	if e.observer.Truthy() {
		e.observer.Call("disconnect")
		e.observerFn.Release()
		e.observer = js.Undefined()
	}
	if e.styleEl.Truthy() {
		e.styleEl.Call("remove")
	}
	e.handles = map[int]js.Value{}
	e.undo.Clear()
}

// OnSelectionChange registers the widget's toolbar-refresh callback.
func (e *Editor) OnSelectionChange(fn func()) { e.onSelChange = fn }

// ── Turn and handle management ──────────────────────────────────────────

// handleFor mints (or reuses within the turn) an opaque handle for node.
func (e *Editor) handleFor(node js.Value) int {
	for h, n := range e.handles {
		if n.Equal(node) {
			return h
		}
	}
	e.nextHandle++
	e.handles[e.nextHandle] = node
	return e.nextHandle
}

// endTurn expires every outstanding handle. Called after each event
// dispatch: a Point is a per-turn capability, and the js.Value entries
// must not keep detached nodes (and their subtrees) alive.
func (e *Editor) endTurn() {
	if len(e.handles) > 0 {
		e.handles = map[int]js.Value{}
	}
}

// resolve maps a Point back to its node, verifying it is still alive,
// still ours, and that the offset still fits.
func (e *Editor) resolve(p Point) (js.Value, error) {
	node, ok := e.handles[p.Node]
	if !ok {
		return js.Undefined(), fmt.Errorf("%w: expired handle", ErrStaleSelection)
	}
	if !node.Get("isConnected").Bool() || !e.contains(node) {
		return js.Undefined(), fmt.Errorf("%w: node left the editor", ErrStaleSelection)
	}
	if p.Offset < 0 || p.Offset > nodeLength(node) {
		return js.Undefined(), fmt.Errorf("%w: offset out of range", ErrStaleSelection)
	}
	return node, nil
}

// contains reports whether node is the root or sits under it.
func (e *Editor) contains(node js.Value) bool {
	return e.root.Call("contains", node).Bool()
}

// nodeLength is a Point offset's upper bound: UTF-16 units for text
// nodes, child count for elements — the DOM's own boundary-point rule.
func nodeLength(node js.Value) int {
	if node.Get("nodeType").Int() == 3 {
		// CharacterData.length: UTF-16 code units. Read it off the node,
		// not off .data — .data is a JS string and Value.Get on a string
		// panics (a panic is total loss here).
		return node.Get("length").Int()
	}
	return node.Get("childNodes").Get("length").Int()
}

// Sel returns the current document selection as per-turn Points, ok=false
// when it does not live inside the editor.
func (e *Editor) Sel() (Selection, bool) {
	docSel := e.doc.Call("getSelection")
	if !docSel.Truthy() || docSel.Get("rangeCount").Int() == 0 {
		return Selection{}, false
	}
	rng := docSel.Call("getRangeAt", 0)
	start, startOff := rng.Get("startContainer"), rng.Get("startOffset").Int()
	end, endOff := rng.Get("endContainer"), rng.Get("endOffset").Int()
	if !e.contains(start) || !e.contains(end) {
		return Selection{}, false
	}
	return Selection{
		From: Point{Node: e.handleFor(start), Offset: startOff},
		To:   Point{Node: e.handleFor(end), Offset: endOff},
	}, true
}

// rangeFor materializes a Selection into a DOM Range, normalizing the
// direction so start <= end.
func (e *Editor) rangeFor(s Selection) (js.Value, error) {
	from, err := e.resolve(s.From)
	if err != nil {
		return js.Undefined(), err
	}
	to, err := e.resolve(s.To)
	if err != nil {
		return js.Undefined(), err
	}
	rng := e.doc.Call("createRange")
	rng.Call("setStart", from, s.From.Offset)
	rng.Call("setEnd", to, s.To.Offset)
	if rng.Get("collapsed").Bool() && !s.Collapsed() {
		// setEnd before setStart collapses a backwards range; rebuild in
		// document order.
		rng.Call("setStart", to, s.To.Offset)
		rng.Call("setEnd", from, s.From.Offset)
	}
	return rng, nil
}

// ── Content I/O ─────────────────────────────────────────────────────────

// Content serializes the document as epub-html. By construction the tree
// only holds what the policy let in, so innerHTML here is already the
// canonical serialization.
func (e *Editor) Content() string {
	return e.root.Get("innerHTML").String()
}

// SetContent replaces the document with html — hostile input like any
// other (a value loaded from a database can carry stored XSS): it goes
// through the same DOMParser + policy walker as a paste. The undo stack
// is cleared: history cannot reach across content loads.
func (e *Editor) SetContent(html string) error {
	f, err := e.sanitizeHTML(html)
	if err != nil {
		return err
	}
	e.beginWrite()
	e.clearContent()
	if !f.Empty() {
		e.root.Call("replaceChildren", e.materialize(f))
		e.ensureBlockShape()
	}
	e.discardRecords()
	e.undo.Clear()
	return nil
}

// IsEmpty reports whether the document holds no user text — the pristine
// single empty paragraph, whatever exact filler markup it carries.
func (e *Editor) IsEmpty() bool {
	return strings.TrimSpace(e.root.Get("textContent").String()) == ""
}

// clearContent resets the tree to the empty document: one empty paragraph
// with the <br> filler contenteditable needs to park the caret.
func (e *Editor) clearContent() {
	p := e.doc.Call("createElement", "p")
	p.Call("appendChild", e.doc.Call("createElement", "br"))
	e.root.Call("replaceChildren", p)
}

// ── DefineClass ─────────────────────────────────────────────────────────

// DefineClass registers a named style. Name and CSS go through the
// epubhtml validators; the rule lands in the editor's own <style> under
// a selector scoped to the editor root.
func (e *Editor) DefineClass(name, css string) error {
	if err := epubhtml.ValidClassName(name); err != nil {
		return err
	}
	clean, err := epubhtml.SanitizeCSS(css)
	if err != nil {
		return err
	}
	e.classes[name] = clean
	e.renderClasses()
	return nil
}

// classDefined reports whether a class name was registered.
func (e *Editor) classDefined(name string) bool {
	_, ok := e.classes[name]
	return ok
}

// renderClasses rebuilds the editor's <style> from the registry. Set via
// textContent — never innerHTML — although every part was sanitized.
func (e *Editor) renderClasses() {
	var sb strings.Builder
	for name, css := range e.classes {
		sb.WriteString("[contenteditable] .")
		sb.WriteString(name)
		sb.WriteString(" { ")
		sb.WriteString(css)
		sb.WriteString(" }\n")
	}
	e.styleEl.Set("textContent", sb.String())
}

// Compile-time check: the js Editor implements the portable plugin API.
var _ EditorCore = (*Editor)(nil)

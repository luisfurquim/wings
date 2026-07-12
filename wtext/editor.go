//go:build js && wasm

package wtext

import (
	"fmt"
	"sort"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings/epubhtml"
)

// G is the logger for this module. Errors are visible by default: the
// guard recovers report through Logf(1), and a swallowed panic with no
// trace is undebuggable.
var G goose.Alert

func init() { G.Set(1) }

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

	// config is the document-property store (see EditorCore.Config): it
	// persists inside Content()'s head and reloads in SetContent.
	// configDefaults carries the ConfigPlugins' declared defaults, the
	// read fallback for keys the document does not store.
	config         map[string]string
	configDefaults map[string]string

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

	// savedStart/savedEnd remember the last selection seen inside the
	// editor as raw endpoints (not per-turn handles): a toolbar control
	// that takes focus — a combobox, a link dialog — moves the document
	// selection out of the editor, but the user's selection is still the
	// operand of the action. Cleared on Detach/SetContent (the js.Values
	// pin DOM nodes).
	savedStart, savedEnd       js.Value
	savedStartOff, savedEndOff int

	// pending holds marks armed at a collapsed caret (Word behaviour:
	// toggle bold with nothing selected, type, the typing comes out bold).
	// Keyed by tag; consumed by the next insertText at the anchor, cleared
	// when the caret moves elsewhere inside the editor (see pending.go).
	pending     map[string]pendingMark
	pendingNode js.Value
	pendingOff  int

	// bufOps holds ops converted from observer-DELIVERED records that
	// could not be pushed yet (IME composition in flight). Delivery empties
	// the observer queue, so these are the only copy; takeOps folds them
	// in front of any synchronously drained records.
	bufOps []jsOp
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
		config:  map[string]string{},
		undo: newUndoStack[jsOp](undoMaxSteps, undoMaxBytes,
			func(op jsOp) int { return op.bytes() }),
	}
	e.configDefaults = configDefaults(p)
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
	e.clearSavedSel()
	e.clearPending()
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

// Sel returns the current selection as per-turn Points, ok=false when
// none exists. A live document selection inside the editor wins and is
// remembered; when focus sits on a toolbar control the remembered one
// still stands in, so the control acts on what the user selected.
func (e *Editor) Sel() (Selection, bool) {
	rng := e.selectionRange()
	if rng.Truthy() {
		start, startOff := rng.Get("startContainer"), rng.Get("startOffset").Int()
		end, endOff := rng.Get("endContainer"), rng.Get("endOffset").Int()
		if e.contains(start) && e.contains(end) {
			e.savedStart, e.savedStartOff = start, startOff
			e.savedEnd, e.savedEndOff = end, endOff
			return Selection{
				From: Point{Node: e.handleFor(start), Offset: startOff},
				To:   Point{Node: e.handleFor(end), Offset: endOff},
			}, true
		}
	}
	return e.savedSel()
}

// savedSel mints per-turn Points over the remembered endpoints, ok=false
// when nothing valid is remembered.
func (e *Editor) savedSel() (Selection, bool) {
	if !e.savedValid() {
		return Selection{}, false
	}
	return Selection{
		From: Point{Node: e.handleFor(e.savedStart), Offset: e.savedStartOff},
		To:   Point{Node: e.handleFor(e.savedEnd), Offset: e.savedEndOff},
	}, true
}

// savedValid reports whether the remembered endpoints still point into
// the editor's live tree with in-range offsets.
func (e *Editor) savedValid() bool {
	for _, n := range []js.Value{e.savedStart, e.savedEnd} {
		if !n.Truthy() || !n.Get("isConnected").Bool() || !e.contains(n) {
			return false
		}
	}
	return e.savedStartOff <= nodeLength(e.savedStart) &&
		e.savedEndOff <= nodeLength(e.savedEnd)
}

// clearSavedSel drops the remembered endpoints and their node pins.
func (e *Editor) clearSavedSel() {
	e.savedStart, e.savedEnd = js.Undefined(), js.Undefined()
	e.savedStartOff, e.savedEndOff = 0, 0
}

// RestoreSel gives focus back to the editor and re-applies the remembered
// selection — the return leg of a focus-taking toolbar control, so the
// user lands back on their selection after the pick. setBaseAndExtent is
// used because Chromium accepts it into shadow trees; a refusal degrades
// to the bare focus.
func (e *Editor) RestoreSel() {
	e.root.Call("focus")
	if !e.savedValid() {
		return
	}
	e.guard("restoresel", func() {
		e.selectionObj().Call("setBaseAndExtent",
			e.savedStart, e.savedStartOff, e.savedEnd, e.savedEndOff)
	})
}

// selectionRange returns the live selection as a Range-shaped js.Value
// (startContainer/startOffset/endContainer/endOffset), undefined when
// there is none. When the editor lives inside a shadow root the document
// selection cannot be used directly: Chromium rescopes it to the host, so
// every boundary point lands outside the editor. The escalation is
// ShadowRoot.getSelection() (Chromium's legacy API), then the standard
// Selection.getComposedRanges(), then the plain document selection (Gecko
// pierces shadow trees natively; also the light-DOM case).
func (e *Editor) selectionRange() js.Value {
	docSel := e.doc.Call("getSelection")
	if !docSel.Truthy() {
		return js.Undefined()
	}
	if root := e.root.Call("getRootNode"); isShadowRoot(root) {
		if root.Get("getSelection").Type() == js.TypeFunction {
			if s := root.Call("getSelection"); s.Truthy() && s.Get("rangeCount").Int() > 0 {
				return s.Call("getRangeAt", 0)
			}
		} else if r := composedRange(docSel, root); r.Truthy() {
			return r
		}
	}
	if docSel.Get("rangeCount").Int() == 0 {
		return js.Undefined()
	}
	return docSel.Call("getRangeAt", 0)
}

// isShadowRoot reports whether v is a ShadowRoot instance.
func isShadowRoot(v js.Value) bool {
	sr := js.Global().Get("ShadowRoot")
	return sr.Type() == js.TypeFunction && v.InstanceOf(sr)
}

// selectionObj returns the Selection to WRITE through: inside a shadow
// root the document selection silently refuses boundary points in the
// shadow tree (Chromium), so writes must go through the shadow root's own
// selection object when it exists.
func (e *Editor) selectionObj() js.Value {
	if root := e.root.Call("getRootNode"); isShadowRoot(root) &&
		root.Get("getSelection").Type() == js.TypeFunction {
		return root.Call("getSelection")
	}
	return e.doc.Call("getSelection")
}

// composedRange asks Selection.getComposedRanges for the selection scoped
// into root, returning the first StaticRange (undefined when absent). The
// call runs under recover: early WebKit shipped a variadic signature that
// rejects the options object, and a JS exception through syscall/js is a
// panic — total loss without the guard.
func composedRange(docSel, root js.Value) (rng js.Value) {
	rng = js.Undefined()
	if docSel.Get("getComposedRanges").Type() != js.TypeFunction {
		return
	}
	defer func() {
		if recover() != nil {
			rng = js.Undefined()
		}
	}()
	ranges := docSel.Call("getComposedRanges", map[string]any{"shadowRoots": []any{root}})
	if ranges.Truthy() && ranges.Get("length").Int() > 0 {
		rng = ranges.Index(0)
	}
	return
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

// maxDocClasses bounds how many class rules a stored document may define
// on load — bounded everything.
const maxDocClasses = 256

// Content serializes the document as a complete EPUB-style content
// document: the body is the editor tree (which by construction only
// holds what the policy let in), and the registered classes the tree
// actually uses travel as ".name { css }" rules in a head <style> — so a
// stored value round-trips with its styles.
func (e *Editor) Content() string {
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\"/>")
	if rules := e.usedClassRules(); rules != "" {
		sb.WriteString("<style>\n")
		sb.WriteString(rules)
		sb.WriteString("</style>")
	}
	sb.WriteString(e.configMetas())
	sb.WriteString("</head><body>")
	sb.WriteString(e.root.Get("innerHTML").String())
	sb.WriteString("</body></html>")
	return sb.String()
}

// Config reads a document property: the value this document stores,
// falling back to the profile's declared default, then "".
func (e *Editor) Config(key string) string {
	if v, ok := e.config[key]; ok {
		return v
	}
	return e.configDefaults[key]
}

// SetConfig stores a document property (bounded); an empty value deletes
// the entry, reverting reads to the declared default.
func (e *Editor) SetConfig(key, value string) error {
	if err := validateConfigKey(key); err != nil {
		return err
	}
	if len(value) > MaxConfigValueLen {
		return fmt.Errorf("%w: %d bytes", ErrConfigValue, len(value))
	}
	if value == "" {
		delete(e.config, key)
		return nil
	}
	if _, ok := e.config[key]; !ok && len(e.config) >= MaxConfigKeys {
		return ErrConfigFull
	}
	e.config[key] = value
	return nil
}

// configMetas renders the store as the persisted head metas, sorted for
// deterministic output. Keys are pre-validated (no quotes/whitespace);
// values are attribute-escaped.
func (e *Editor) configMetas() string {
	if len(e.config) == 0 {
		return ""
	}
	keys := make([]string, 0, len(e.config))
	for k := range e.config {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(`<meta name="wt-cfg-`)
		sb.WriteString(k)
		sb.WriteString(`" content="`)
		sb.WriteString(metaAttrEscaper.Replace(e.config[k]))
		sb.WriteString(`"/>`)
	}
	return sb.String()
}

var metaAttrEscaper = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;",
)

// adoptDocConfig loads the wt-cfg- head metas of a stored document into
// the store. Hostile input: SetConfig's own validation bounds each entry,
// and a failing one is skipped (fail toward losing a property, never the
// document).
func (e *Editor) adoptDocConfig(parsed js.Value) {
	head := parsed.Get("head")
	if !head.Truthy() {
		return
	}
	metas := head.Call("querySelectorAll", `meta[name]`)
	for i := 0; i < metas.Get("length").Int(); i++ {
		m := metas.Index(i)
		name := m.Call("getAttribute", "name").String()
		if !strings.HasPrefix(name, "wt-cfg-") {
			continue
		}
		val := ""
		if c := m.Call("getAttribute", "content"); c.Truthy() {
			val = c.String()
		}
		if err := e.SetConfig(strings.TrimPrefix(name, "wt-cfg-"), val); err != nil {
			G.Logf(1, "wtext: dropping stored config %q: %v\n", name, err)
		}
	}
}

// DocText returns the document's plain text with a newline at every
// block boundary and <br> — a bare textContent read would fuse the last
// word of one paragraph with the first of the next.
func (e *Editor) DocText() string {
	var sb strings.Builder
	var walk func(n js.Value)
	walk = func(n js.Value) {
		switch n.Get("nodeType").Int() {
		case 3: // text
			sb.WriteString(n.Get("data").String())
		case 1: // element
			tag := strings.ToLower(n.Get("tagName").String())
			if tag == "br" {
				sb.WriteString("\n")
				return
			}
			kids := n.Get("childNodes")
			for i := 0; i < kids.Get("length").Int(); i++ {
				walk(kids.Index(i))
			}
			if epubhtml.IsBlock(tag) {
				sb.WriteString("\n")
			}
		}
	}
	walk(e.root)
	return sb.String()
}

// usedClassRules renders the registered classes the tree references, in
// the persistable logical form (one unsplit rule per class), sorted.
func (e *Editor) usedClassRules() string {
	els := e.root.Call("querySelectorAll", "[class]")
	used := map[string]bool{}
	for i := 0; i < els.Get("length").Int(); i++ {
		for _, cls := range strings.Fields(els.Index(i).Call("getAttribute", "class").String()) {
			if e.classDefined(cls) {
				used[cls] = true
			}
		}
	}
	names := make([]string, 0, len(used))
	for n := range used {
		names = append(names, n)
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, n := range names {
		sb.WriteString(".")
		sb.WriteString(n)
		sb.WriteString(" { ")
		sb.WriteString(e.classes[n])
		sb.WriteString(" }\n")
	}
	return sb.String()
}

// SetContent replaces the document with html — hostile input like any
// other (a value loaded from a database can carry stored XSS): it goes
// through the same DOMParser + policy walker as a paste. Class rules the
// document carries in its head are adopted first (each re-validated), so
// the body's class attributes survive the registry filter. The undo
// stack is cleared: history cannot reach across content loads.
func (e *Editor) SetContent(html string) error {
	parsed := js.Global().Get("DOMParser").New().
		Call("parseFromString", html, "text/html")
	e.config = map[string]string{} // a content load replaces the document's properties too
	e.adoptDocConfig(parsed)
	e.adoptDocClasses(parsed)
	var f Fragment
	if body := parsed.Get("body"); body.Truthy() {
		f.nodes = e.copyChildren(body, false)
	}
	e.beginWrite()
	e.clearContent()
	if !f.Empty() {
		e.root.Call("replaceChildren", e.materialize(f))
		e.ensureBlockShape()
	}
	e.discardRecords()
	e.clearSavedSel()
	e.clearPending()
	e.undo.Clear()
	return nil
}

// adoptDocClasses registers the class rules a stored document carries in
// its head <style> elements. The rules are hostile input: each must be
// exactly ".name { declarations }", name and CSS pass the same validators
// as DefineClass, and a rule that fails is skipped (fail toward text),
// never trusted. Utility classes (wt-*) already defined by the attached
// plugins are theirs — a document cannot redefine what the toolbar will
// apply. The DOMParser tree is inert, so nothing here ever executes.
func (e *Editor) adoptDocClasses(parsed js.Value) {
	head := parsed.Get("head")
	if !head.Truthy() {
		return
	}
	styles := head.Call("querySelectorAll", "style")
	adopted := 0
	for i := 0; i < styles.Get("length").Int(); i++ {
		for _, rule := range strings.Split(styles.Index(i).Get("textContent").String(), "}") {
			rule = strings.TrimSpace(rule)
			if rule == "" {
				continue
			}
			sel, decls, found := strings.Cut(rule, "{")
			if !found {
				continue
			}
			sel = strings.TrimSpace(sel)
			if !strings.HasPrefix(sel, ".") {
				continue
			}
			name := sel[1:]
			if strings.HasPrefix(name, "wt-") && e.classDefined(name) {
				continue
			}
			if adopted >= maxDocClasses {
				G.Logf(1, "wtext: document sheet beyond %d classes; rest ignored\n", maxDocClasses)
				return
			}
			if err := e.DefineClass(name, strings.TrimSpace(decls)); err != nil {
				G.Logf(1, "wtext: document style %q rejected: %v\n", name, err)
				continue
			}
			adopted++
		}
	}
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

// Classes returns every registered class name, sorted.
func (e *Editor) Classes() []string {
	names := make([]string, 0, len(e.classes))
	for name := range e.classes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ClassCSS returns the sanitized CSS registered for name.
func (e *Editor) ClassCSS(name string) (string, bool) {
	css, ok := e.classes[name]
	return css, ok
}

// renderClasses rebuilds the editor's <style> from the registry. Each
// class renders as its Word split: the character half on span.name, the
// paragraph half on the profile blocks — one name, two scoped rules, so
// a block never inherits character declarations it should not carry.
// Named styles render first and wt-* utilities last: later rules win
// specificity ties, so direct formatting overrides a named style, as it
// does in Word. Set via textContent — never innerHTML — although every
// part was sanitized.
func (e *Editor) renderClasses() {
	names := e.Classes()
	ordered := make([]string, 0, len(names))
	for _, n := range names {
		if !strings.HasPrefix(n, "wt-") {
			ordered = append(ordered, n)
		}
	}
	for _, n := range names {
		if strings.HasPrefix(n, "wt-") {
			ordered = append(ordered, n)
		}
	}
	blockSel := ":is(" + strings.Join(epubhtml.BlockList(), ",") + ")"
	var sb strings.Builder
	for _, name := range ordered {
		char, block := epubhtml.SplitCSS(e.classes[name])
		if char != "" {
			sb.WriteString("[contenteditable] span.")
			sb.WriteString(name)
			sb.WriteString(" { ")
			sb.WriteString(char)
			sb.WriteString(" }\n")
		}
		if block != "" {
			sb.WriteString("[contenteditable] ")
			sb.WriteString(blockSel)
			sb.WriteString(".")
			sb.WriteString(name)
			sb.WriteString(" { ")
			sb.WriteString(block)
			sb.WriteString(" }\n")
		}
	}
	e.styleEl.Set("textContent", sb.String())
}

// Compile-time check: the js Editor implements the portable plugin API.
var _ EditorCore = (*Editor)(nil)

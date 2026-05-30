//go:build js && wasm

package wings

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"context"
	"fmt"
	"strconv"
	"sync"
	"syscall/js"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings/expr"
)

// itoa is a package-local shortcut used by SynPrinter fallbacks.
func itoa(n int) string { return strconv.Itoa(n) }

// G is the global logger for the package. Recommended levels: 1=errors only, 2=general, 3=detail,
// 4=light debug, 5=verbose debug, 6=sensitive debug.
var G goose.Alert = goose.Alert(2)

// InitWG lets side-effect packages (e.g. wi18n) delay Main() until their own
// asynchronous initialization has finished. Any package that needs to run work
// before DefineAll() should call InitWG.Add(1) in its init() and InitWG.Done()
// when its work is complete. Main() calls InitWG.Wait() before defining the
// custom elements. If nothing registers, Wait() returns immediately.
var InitWG sync.WaitGroup

// printer is the active TextNode transform function (private; set via SetPrinter).
var printer func(string) string = ByPass

// Printer calls the active printer function. Installed packages (e.g. wi18n)
// replace the default ByPass via SetPrinter; all other callers use this wrapper.
func Printer(s string) string { return printer(s) }

// ByPass is the default Printer: it returns its input unchanged.
func ByPass(in string) string { return in }

// ── Printer authorization ────────────────────────────────────────────────────

var (
	printerMu        sync.Mutex
	printerSet       bool
	printerTokenHash [sha256.Size]byte // sha256 of the one-time authorization token
	printerTokenVal  []byte            // raw token; nil after TakePrinterToken is called
	printerTokenOut  bool              // true after TakePrinterToken was called
)

// TakePrinterToken returns the one-time authorization token required by
// SetPrinter. It can only be called once; subsequent calls return nil.
// Intended for use by github.com/luisfurquim/wings/wi18n only.
func TakePrinterToken() []byte {
	printerMu.Lock()
	defer printerMu.Unlock()
	if printerTokenOut {
		return nil
	}
	printerTokenOut = true
	tok := make([]byte, len(printerTokenVal))
	copy(tok, printerTokenVal)
	return tok
}

// SetPrinter installs fn as the active Printer. token must be the value
// previously returned by TakePrinterToken(). Panics if the token is invalid.
// Idempotent: subsequent calls from the same authorized holder are no-ops once
// the printer is installed (so SetLang() re-calls do not panic).
func SetPrinter(fn func(string) string, token []byte) {
	h := sha256.Sum256(token)
	printerMu.Lock()
	defer printerMu.Unlock()
	if !bytes.Equal(h[:], printerTokenHash[:]) {
		panic("wprana: SetPrinter: invalid authorization token")
	}
	if printerSet {
		return
	}
	printer = fn
	printerSet = true
}

// SynPrinter resolves a flexion reference block (e.g. `{{@genero %qt #42}}`)
// at sync time. It receives the raw flex token slice (the inner part of the
// `{{...}}` block) and the current data context stack, and returns the
// rendered string for the current locale.
//
// The default is NoFlexSynPrinter, which emits a best-effort literal
// rendering so pages without wi18n still show something sensible. When
// wi18n is imported, its init installs a catalog-backed implementation
// that performs CLDR plural resolution against the loaded inflections
// catalog.
var SynPrinter func(toks []RefNode, ctx Ctx) string = NoFlexSynPrinter

// NoFlexSynPrinter is the default SynPrinter: since no inflections catalog
// is loaded, it returns the rule index in `#N` form as a visible marker —
// missing translations stay obvious instead of rendering blank.
func NoFlexSynPrinter(toks []RefNode, _ Ctx) string {
	for _, t := range toks {
		if t.Type == expr.TokFlexIdx {
			return "#" + itoa(t.IntVal)
		}
	}
	return ""
}

// Locale is the BCP 47 language tag currently in effect for locale-aware
// rendering (FmtPrinter, SynPrinter). Empty until a catalog-backed package
// (e.g. wi18n) detects the browser language and assigns it. Reading an
// empty Locale is safe — FmtPrinter implementations fall back to a
// locale-agnostic rendering in that case.
var Locale string

// NodeAnnotator is called by translateTextNodes and applyStashSweep after each
// node is translated. rawIndex is the original TextNode content (the decimal
// integer string written by gen_i18n). node is the DOM node that was
// translated: a Text node (nodeType==3) for text content, or an Element node
// (nodeType==1) for attribute translations. The default nil means no annotation.
// wi18n installs a function here when TranslateCheckHighlight is active.
var NodeAnnotator func(rawIndex string, node js.Value)

// FmtPrinter formats a single value into its locale-appropriate string
// representation. It is called by the solver for lone-`%var` bindings
// (FmtBlock), invoked with the resolved value, the current Locale, and the
// format name extracted from the `%var:formatName` template syntax (empty
// string when the template uses bare `%var`).
//
// The default is NoFmtFmtPrinter, a locale-agnostic `fmt.Sprint` passthrough.
// When wi18n is imported, its init installs a type-switching implementation
// that routes native ints/floats through Intl.NumberFormat, time.Time
// through Intl.DateTimeFormat, and any value implementing wi18n.Numerical
// through its Format(locale, formatName) method. A non-nil error from
// Numerical.Format stops rendering: the binding emits "" and the error is
// logged with locale/formatName context.
var FmtPrinter func(val any, locale, formatName string) string = NoFmtFmtPrinter

// NoFmtFmtPrinter is the default FmtPrinter. With no locale-aware backend
// loaded, it renders values via fmt.Sprint so pages still show something
// reasonable — just not locale-correct.
func NoFmtFmtPrinter(val any, _ string, _ string) string {
	if val == nil {
		return ""
	}
	return fmt.Sprint(val)
}

// ── Type aliases for expr package ───────────────────────────────────────────
// These aliases allow the rest of the wprana package to use the parser
// types without qualifying every reference with "expr.".

type TokenType = expr.TokenType
type RefNode = expr.RefNode
type TextSegment = expr.TextSegment
type FlexBlock = expr.FlexBlock

const (
	TokTxt       = expr.TokTxt
	TokRef       = expr.TokRef
	TokStr       = expr.TokStr
	TokDot       = expr.TokDot
	TokOpen      = expr.TokOpen
	TokClose     = expr.TokClose
	TokNum       = expr.TokNum
	TokIdent     = expr.TokIdent
	TokWSep      = expr.TokWSep
	TokExpr      = expr.TokExpr
	TokAttr      = expr.TokAttr
	TokPctVar    = expr.TokPctVar
	TokAtVar     = expr.TokAtVar
	TokTildeWord = expr.TokTildeWord
	TokFlexIdx   = expr.TokFlexIdx
	TokColon     = expr.TokColon
)

// AttrBinding stores the bindings of a single attribute.
type AttrBinding struct {
	Segs      []TextSegment
	ForceSync bool      // attribute with & prefix
	PureRef   []RefNode // non-nil if the binding is a pure reference (two-way binding)
}

// DOMRefNode describes the template bindings for a DOM node.
type DOMRefNode struct {
	Type     TokenType               // TokTxt = text node; TokAttr = element
	TextSegs []TextSegment           // text node segments
	Attrs    map[string]*AttrBinding // attribute bindings
	Children map[int]*DOMRefNode     // child bindings (by index)
	ArrayVar string                  // array iteration control variable (empty if none)
	ArrayIdx string                  // array iteration index variable
	NoSpan   bool                    // ** prefix: model is the parent itself
	ModelRef *DOMRefNode             // child template ref for ** (noSpan)
	Cond     string                  // conditional expression (empty if none)
	CondTree []RefNode               // parsed tree of the condition
	CondOp   string                  // conditional operator: "" = truthy, "eq" = equality, "neq" = inequality
	CondVal  string                  // literal value for comparison (used with CondOp "eq" or "neq")
}

// ── Reactive state ──────────────────────────────────────────────────────────

// Change describes a data mutation for optimized sync.
type Change struct {
	Delete *DeleteInfo
}

// DeleteInfo describes the removal of an array element.
type DeleteInfo struct {
	Target []any // target slice (reference, not copy)
	Index  int
}

// Ctx is a data context stack used in reference resolution.
type Ctx []any

// PranaState holds the reactive state of a bound component.
type PranaState struct {
	Data      *ReactiveData
	Refs      *DOMRefNode
	ForceSync bool
	MaySync   bool
	dom       js.Value // SPAN container in the shadow root
	model     js.Value // root of the HTML template content
	lastEpoch uint64   // epoch of the last sync (for cycle prevention)
}

// ReactiveData encapsulates the data map with change notification.
// Mutations via Set/Delete/Append/DeleteAt trigger automatic sync.
type ReactiveData struct {
	M     map[string]any
	state *PranaState
}

// TwoWayBinding holds the state of a bidirectional binding (input/select/textarea).
type TwoWayBinding struct {
	Ref     []RefNode
	CtxPtr  *Ctx    // updated on each sync; handler closure points here
	Handler js.Func // JS handler; must be Released when the element is removed
}

// NodeState stores the Go-side state for DOM nodes managed by prana.
// It is indexed by the _pranaId field of the JS node.
type NodeState struct {
	// For array iteration plug nodes
	Model  js.Value
	ACtrl  string
	AIndex string
	Tree   []RefNode

	// For conditional nodes (when replaced by a comment)
	CondModel js.Value // the original element (stored while there is a comment)
	CondDaddy js.Value // parent for conditional restoration

	// For component roots
	State     *PranaState
	PRoot     js.Value
	EHandlers map[string]string

	// For bidirectional bindings (indexed by attribute name)
	TwoWay map[string]*TwoWayBinding

	// Shadow root reference — populated for all components; required when the
	// component uses mode:"closed" (element.shadowRoot returns null in that case).
	ShadowRoot js.Value

	// Render lifecycle — cancel stops the waitAndRender goroutine; RenderDone
	// is closed when that goroutine exits, enabling safe async cleanup.
	CancelRender context.CancelFunc
	RenderDone   chan struct{}
}

// ── Key-value storage interface ──────────────────────────────────────────────

// KeyStorage defines a key-value storage backend that accepts arbitrary
// Go values. Implementations are responsible for serializing values
// (typically via an Encoder/Decoder pair).
type KeyStorage interface {
	Set(key string, val any) error
	Get(key string, outval any) error
	Del(key string) error
	Exists(key string) (bool, int64, error)
}

// ── Module public interface ─────────────────────────────────────────────────

// PranaObj is passed to the module's Render method.
type PranaObj struct {
	This    *ReactiveData
	Dom     js.Value // SPAN in the shadow root
	Element js.Value // the custom element itself
	Trigger func(eventName string, args ...any)
}

// PranaMod is the interface that every Go web component must implement.
//   - InitData() returns the initial data map (equivalent to the "return {...}" in JS).
//   - Render(obj) is called after connection to the DOM (equivalent to the ready.then(...) in JS).
type PranaMod interface {
	InitData() map[string]any
	Render(obj *PranaObj)
}

// TriggerHandler is the function type used as a handler for @ events.
// Use the nil literal (TriggerHandler(nil)) as a placeholder in InitData;
// define the actual body in Render, where obj is available.
type TriggerHandler func(...any)

// CSSPart represents a named CSS section of a component.
// The order of CSSParts matters: Vars must come before Design,
// because Design may use variables defined in Vars.
type CSSPart struct {
	Name    string
	Content string
}

// Customizable is an optional interface that modules can implement
// to allow consuming applications to change parts of the CSS.
// Modules that satisfy only PranaMod have fixed CSS; modules that
// satisfy Customizable allow replacement of CSS sections
// (e.g.: swap only the color variables, keeping the layout).
type Customizable interface {
	PranaMod
	ListCSS() []CSSPart
	ReplaceCSS(key string, content string)
}

// ModFactory creates a new instance of PranaMod.
type ModFactory func() PranaMod

// ComponentOpts configures optional behavior for a custom element.
type ComponentOpts struct {
	// Closed makes the shadow root use mode:"closed", preventing external scripts
	// from accessing the component's internals via element.shadowRoot.
	// See README.md §Security — CVE-2019-11730, GHSA-wh77-3x4m-4q9g.
	Closed bool
}

// modDef is the internal definition of a registered module.
type modDef struct {
	factory  ModFactory
	html     string
	css      string
	observed []string // attributes observed by attributeChangedCallback
	closed   bool     // true → shadow root mode:"closed"
}

// ── Global registries ───────────────────────────────────────────────────────

var (
	// moduleRegistry stores the modules registered via Register().
	moduleRegistry = map[string]*modDef{}

	// nodeRegistry stores the Go-side state of DOM nodes, indexed by _pranaId.
	nodeRegistry = map[int64]*NodeState{}

	// instanceRegistry tracks the live instances of each custom element
	// by tagName, allowing Update() to update the CSS of all of them.
	instanceRegistry = map[string][]js.Value{}

	// nextNodeID is the next ID to be assigned.
	nextNodeID int64 = 1

	// jsSVGNS is the SVG namespace.
	jsSVGNS = "http://www.w3.org/2000/svg"

	// syncEpoch is the global propagation epoch counter.
	// Each propagation chain (Set, Delete, etc.) increments the epoch.
	// Components already synced in the current epoch are skipped,
	// breaking circular propagation cycles.
	// Starts at 1 so that lastEpoch=0 (PranaState default) is always < syncEpoch.
	syncEpoch uint64 = 1

	// syncDepth counts the nesting level of sync (0 = no sync in progress).
	// Used to distinguish elementAttrChanged triggered internally (by
	// setAttribute during sync) from external changes (user JavaScript).
	syncDepth int
)

// jsVars are initialized in init() to avoid repeated calls.
var (
	jsGlobal js.Value
	jsDoc    js.Value
)

func init() {
	jsGlobal = js.Global()
	jsDoc = jsGlobal.Get("document")
	initHash()
	initPrinterToken()
}

func initPrinterToken() {
	tok := make([]byte, 32)
	if _, err := cryptorand.Read(tok); err != nil {
		panic("wprana: crypto/rand unavailable: " + err.Error())
	}
	printerTokenVal = tok
	printerTokenHash = sha256.Sum256(tok)
}

// assignNodeID assigns a unique _pranaId to the node and returns the ID.
func assignNodeID(node js.Value) int64 {
	id := nextNodeID
	nextNodeID++
	node.Set("_pranaId", id)
	return id
}

// getNodeID reads the _pranaId from a JS node. Returns (0, false) if it doesn't have one.
func getNodeID(node js.Value) (int64, bool) {
	v := node.Get("_pranaId")
	if v.IsUndefined() || v.IsNull() {
		return 0, false
	}
	return int64(v.Int()), true
}

// getOrCreateState returns (or creates) the NodeState associated with a DOM node.
func getOrCreateState(node js.Value) (int64, *NodeState) {
	id, ok := getNodeID(node)
	if ok {
		if st, found := nodeRegistry[id]; found {
			return id, st
		}
	}
	id = assignNodeID(node)
	st := &NodeState{}
	nodeRegistry[id] = st
	return id, st
}

// getState returns the NodeState of a DOM node, or nil if it doesn't exist.
func getState(node js.Value) *NodeState {
	id, ok := getNodeID(node)
	if !ok {
		return nil
	}
	return nodeRegistry[id]
}

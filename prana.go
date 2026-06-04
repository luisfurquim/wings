//go:build js && wasm

package wings

import (
	"context"
	"strings"
	"syscall/js"
	"time"

	"github.com/luisfurquim/wings/expr"
)

// ── Module registration ─────────────────────────────────────────────────────

// Register registers a Go web component to be defined as a custom element.
// Must be called from within func init() in the module's package.
//
//	tagName     - name of the custom element (e.g. "my-widget")
//	htmlContent - HTML content of the template (usually via //go:embed)
//	cssContent  - CSS content of the component (usually via //go:embed)
//	factory     - function that creates a new instance of PranaMod
//	observed    - names of attributes to be observed (attributeChangedCallback)
func Register(tagName, htmlContent, cssContent string, factory ModFactory, observed ...string) {
	if _, exists := moduleRegistry[tagName]; exists {
		G.Logf(1, "Register: module %q already registered\n", tagName)
		return
	}
	moduleRegistry[tagName] = &modDef{
		factory:  factory,
		html:     htmlContent,
		css:      cssContent,
		observed: observed,
	}
	G.Logf(2, "Register: module %q registered\n", tagName)
}

// RegisterWithOpts is like Register but accepts ComponentOpts for additional
// configuration (e.g. Closed shadow DOM mode).
func RegisterWithOpts(tagName, htmlContent, cssContent string, opts ComponentOpts, factory ModFactory, observed ...string) {
	if _, exists := moduleRegistry[tagName]; exists {
		G.Logf(1, "RegisterWithOpts: module %q already registered\n", tagName)
		return
	}
	moduleRegistry[tagName] = &modDef{
		factory:  factory,
		html:     htmlContent,
		css:      cssContent,
		observed: observed,
		closed:   opts.Closed,
	}
	G.Logf(2, "RegisterWithOpts: module %q registered (closed=%v)\n", tagName, opts.Closed)
}

// DefineAll defines all custom elements registered via Register().
// Must be called once in main() after all modules have been imported.
func DefineAll() {
	for tagName, def := range moduleRegistry {
		defineCustomElement(tagName, def)
	}
}

// ── Custom element definition ───────────────────────────────────────────────

// defineCustomElement uses the JS helper _pranaDef to register the custom element.
// All lifecycle logic is implemented in Go; the JS only forwards
// the constructor/connectedCallback/attributeChangedCallback calls.
func defineCustomElement(tagName string, def *modDef) {
	pranaDef := jsGlobal.Get("_pranaDef")
	if pranaDef.IsUndefined() || pranaDef.IsNull() {
		G.Logf(1, "defineCustomElement: _pranaDef not found in global scope. "+
			"Include the prana_helper.js helper before the WASM.\n")
		return
	}

	// Converts observed to JS array
	jsObserved := jsGlobal.Get("Array").New()
	for i, attr := range def.observed {
		jsObserved.SetIndex(i, attr)
	}

	constructorFn := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		self := args[0]
		elementConstructor(self, tagName, def)
		return nil
	})

	connectedFn := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		self := args[0]
		elementConnected(self)
		return nil
	})

	attrChangedFn := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) < 4 {
			return nil
		}
		self := args[0]
		name := args[1].String()
		oldVal := args[2].String()
		newVal := args[3].String()
		elementAttrChanged(self, name, oldVal, newVal)
		return nil
	})

	disconnectedFn := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		self := args[0]
		elementDisconnected(self)
		return nil
	})

	pranaDef.Invoke(tagName, constructorFn, connectedFn, attrChangedFn, disconnectedFn, jsObserved)
	G.Logf(2, "defineCustomElement: %q defined\n", tagName)
}

// TranslatableAttrs lists the element attributes whose values should be
// passed through Printer at construction time. Must mirror the effective
// attribute set gen_i18n was run with; the default matches gen_i18n's
// default. Apps can assign a new slice directly to override, or call
// AddTranslatableAttrs / RemoveTranslatableAttrs to tweak the list.
var TranslatableAttrs = []string{"title", "placeholder", "alt", "aria-label", "data-i18n", "expect"}

// AddTranslatableAttrs appends attrs to TranslatableAttrs, skipping those
// already present. Attribute names are compared case-insensitively.
func AddTranslatableAttrs(attrs ...string) {
	for _, a := range attrs {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		found := false
		for _, existing := range TranslatableAttrs {
			if strings.EqualFold(existing, a) {
				found = true
				break
			}
		}
		if !found {
			TranslatableAttrs = append(TranslatableAttrs, a)
		}
	}
}

// RemoveTranslatableAttrs deletes attrs from TranslatableAttrs. Attribute
// names are compared case-insensitively; missing entries are ignored.
func RemoveTranslatableAttrs(attrs ...string) {
	for _, a := range attrs {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		kept := TranslatableAttrs[:0]
		for _, existing := range TranslatableAttrs {
			if !strings.EqualFold(existing, a) {
				kept = append(kept, existing)
			}
		}
		TranslatableAttrs = kept
	}
}

// translateTextNodes walks the cloned template DOM and rewrites every
// TextNode's content via the current Printer, and every value of the
// attributes listed in TranslatableAttrs. Text nodes and elements whose tag
// is <style> or <script> are skipped. Called from elementConstructor right
// after the template is cloned and before bindElement.
//
// Each translated node also receives a JS expando carrying the pre-Printer
// source string ("_wi18nSrc" on text nodes, "_wi18nAttr_<name>" on elements
// per translated attribute). These expandos let SetLang() locate every node
// originally driven by Printer and re-translate without rebuilding the DOM.
// The expandos do not survive Node.cloneNode(true) — see copyTranslateStash
// for the post-clone re-stash used by array iteration.
func translateTextNodes(node js.Value) {
	if node.IsNull() || node.IsUndefined() {
		return
	}
	tag := node.Get("nodeName").String()
	if tag == "STYLE" || tag == "SCRIPT" {
		return
	}
	if node.Get("nodeType").Int() == 1 { // ELEMENT_NODE
		for _, a := range TranslatableAttrs {
			if !node.Call("hasAttribute", a).Bool() {
				continue
			}
			orig := node.Call("getAttribute", a).String()
			if orig == "" {
				continue
			}
			node.Set("_wi18nAttr_"+a, orig)
			node.Call("setAttribute", a, Printer(orig))
			if NodeAnnotator != nil {
				NodeAnnotator(orig, node)
			}
		}
	}
	children := node.Get("childNodes")
	n := children.Get("length").Int()
	for i := 0; i < n; i++ {
		child := children.Index(i)
		nodeType := child.Get("nodeType").Int()
		switch nodeType {
		case 3: // Node.TEXT_NODE
			orig := child.Get("nodeValue").String()
			child.Set("_wi18nSrc", orig)
			child.Set("nodeValue", Printer(orig))
			if NodeAnnotator != nil {
				NodeAnnotator(orig, child)
			}
		case 1: // Node.ELEMENT_NODE
			translateTextNodes(child)
		}
	}
}

// copyTranslateStash mirrors the _wi18nSrc / _wi18nAttr_* expandos from src
// to dst by walking both subtrees in lockstep. Required after cloneNode(true)
// because expando properties are not copied by the JS clone, but the cloned
// subtree has identical topology so positional walking is reliable.
func copyTranslateStash(src, dst js.Value) {
	if src.IsNull() || src.IsUndefined() || dst.IsNull() || dst.IsUndefined() {
		return
	}
	if src.Get("nodeType").Int() == 1 { // ELEMENT_NODE
		for _, a := range TranslatableAttrs {
			key := "_wi18nAttr_" + a
			v := src.Get(key)
			if !v.IsUndefined() && !v.IsNull() {
				dst.Set(key, v)
			}
		}
	}
	srcKids := src.Get("childNodes")
	dstKids := dst.Get("childNodes")
	n := srcKids.Get("length").Int()
	if dstKids.Get("length").Int() < n {
		n = dstKids.Get("length").Int()
	}
	for i := 0; i < n; i++ {
		s := srcKids.Index(i)
		d := dstKids.Index(i)
		if s.Get("nodeType").Int() == 3 { // TEXT_NODE
			v := s.Get("_wi18nSrc")
			if !v.IsUndefined() && !v.IsNull() {
				d.Set("_wi18nSrc", v)
			}
		} else if s.Get("nodeType").Int() == 1 {
			copyTranslateStash(s, d)
		}
	}
}

// ── Element lifecycle ───────────────────────────────────────────────────────

// elementConstructor is called when the custom element is instantiated.
// Creates the shadow root, loads HTML/CSS, initializes the module, and sets up
// the data binding.
func elementConstructor(self js.Value, tagName string, def *modDef) {
	G.Logf(3, "elementConstructor: %q\n", tagName)

	// Creates shadow root (open or closed per ComponentOpts.Closed).
	mode := "open"
	if def.closed {
		mode = "closed"
	}
	shadowRoot := self.Call("attachShadow", map[string]any{"mode": mode})

	// Injects CSS
	if def.css != "" {
		cssNode := domCreateStyleNode(def.css)
		shadowRoot.Call("appendChild", cssNode)
	}

	// Container span in the shadow root
	container := domCreateElement("SPAN")
	shadowRoot.Call("appendChild", container)

	// Parses the HTML template
	tmpl := domCreateTemplate(def.html)
	content := tmpl.Get("content").Call("cloneNode", true)

	// If the template has a single child element, use it directly.
	// Otherwise, wrap all childNodes in a <span> wrapper
	// so that bindElement always receives a single root node.
	var htmlRoot js.Value
	children := content.Get("children")
	if children.Get("length").Int() == 1 && content.Get("childNodes").Get("length").Int() == 1 {
		htmlRoot = children.Index(0)
	} else {
		htmlRoot = domCreateSpan()
		for content.Get("childNodes").Get("length").Int() > 0 {
			htmlRoot.Call("appendChild", content.Get("childNodes").Index(0))
		}
	}

	// Applies the active Printer to every TextNode of the template
	// (skipping children of <style>/<script>). With the default ByPass
	// this is a no-op; when wi18n is imported, Printer translates each
	// TextNode's numeric index into the localized string.
	translateTextNodes(htmlRoot)

	// Reads element attributes for initial data
	var attrs [][2]string
	nAttrs := attrLen(self)
	for i := 0; i < nAttrs; i++ {
		n, v := attrAt(self, i)
		attrs = append(attrs, [2]string{n, v})
	}

	// Instantiates the module
	mod := def.factory()
	data := mod.InitData()
	if data == nil {
		data = map[string]any{}
	}

	// Binds data to the DOM
	rd := bindElement(data, container, htmlRoot, attrs)

	// Stores a reference to the state in the node registry
	nodeID, st := getOrCreateState(self)
	st.State = rd.state
	st.ShadowRoot = shadowRoot

	// Set up render lifecycle cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	st.CancelRender = cancel
	st.RenderDone = make(chan struct{})

	// Marks the element with its modName for debug
	self.Set("_pranaTag", tagName)
	self.Set("_pranaNodeId", nodeID)

	// Tracks the instance so Update() can update the CSS
	instanceRegistry[tagName] = append(instanceRegistry[tagName], self)

	// Launches goroutine that waits for connection and then calls Render
	go waitAndRender(ctx, st.RenderDone, self, mod, rd, attrs)
}

// elementConnected is called when the element is inserted into the DOM.
func elementConnected(self js.Value) {
	self.Set("_pranaConnected", true)
	G.Logf(4, "elementConnected: %s\n", self.Get("_pranaTag").String())
}

// elementAttrChanged is called when an observed attribute changes.
func elementAttrChanged(self js.Value, name, oldVal, newVal string) {
	if oldVal == newVal {
		return
	}
	G.Logf(4, "elementAttrChanged: %s attr=%q %q→%q\n",
		self.Get("_pranaTag").String(), name, oldVal, newVal)

	// Checks if the new value is a reference (should not be propagated)
	segs, err := expr.ParseText(newVal)
	if err != nil {
		return
	}
	for i := range segs {
		if segs[i].IsRef {
			return // value is still a template, do not propagate
		}
	}

	// Propagates to the data map and triggers local sync.
	st := getState(self)
	if st == nil || st.State == nil {
		return
	}
	st.State.Data.M[name] = coerceToType(newVal, st.State.Data.M[name])

	// If we are OUTSIDE a sync chain (syncDepth==0), it is an external change
	// (e.g. user JavaScript). Start a new epoch so that syncLocal proceeds
	// even if the component has already been synced.
	// If we are INSIDE a chain (syncDepth>0), use the current epoch:
	// syncLocal will be skipped if the component has already been synced in this epoch.
	if syncDepth == 0 {
		syncEpoch++
	}
	st.State.syncLocal(nil)
}

// elementDisconnected is called when the element is removed from the DOM.
func elementDisconnected(self js.Value) {
	tag := self.Get("_pranaTag").String()
	G.Logf(3, "elementDisconnected: %s\n", tag)
	nodeID, ok := getNodeID(self)
	if !ok {
		return
	}

	// Cancel the waitAndRender goroutine and capture the done channel before
	// removing state, so the cleanup goroutine below can safely wait.
	st := nodeRegistry[nodeID]
	var renderDone chan struct{}
	if st != nil {
		if st.CancelRender != nil {
			st.CancelRender()
		}
		renderDone = st.RenderDone
	}

	releaseTwoWayBindings(nodeID)
	delete(nodeRegistry, nodeID)

	// Removes from the list of live instances
	instances := instanceRegistry[tag]
	for i, inst := range instances {
		if inst.Equal(self) {
			instanceRegistry[tag] = append(instances[:i], instances[i+1:]...)
			break
		}
	}

	// Wait for waitAndRender to exit in a goroutine — do not block the JS
	// event loop. Any additional post-disconnect cleanup goes here.
	if renderDone != nil {
		go func() { <-renderDone }()
	}
}

// ── Wait for connection and call Render ─────────────────────────────────────

// waitAndRender waits for the element to connect to the DOM then calls Render.
// It exits early if ctx is cancelled (element disconnected before Render ran).
// done is closed on every exit path so elementDisconnected can await teardown.
func waitAndRender(ctx context.Context, done chan struct{}, self js.Value, mod PranaMod, rd *ReactiveData, attrs [][2]string) {
	defer close(done)

	// Poll until connected=true, honouring cancellation on each tick.
	for {
		if ctx.Err() != nil {
			return
		}
		if isConnected(self) {
			break
		}
		tick := make(chan struct{})
		jsGlobal.Call("setTimeout", js.FuncOf(func(this js.Value, args []js.Value) any {
			close(tick)
			return nil
		}), 10)
		select {
		case <-ctx.Done():
			return
		case <-tick:
		}
	}

	// 100ms stabilization wait, still honouring cancellation.
	tick := make(chan struct{})
	jsGlobal.Call("setTimeout", js.FuncOf(func(this js.Value, args []js.Value) any {
		close(tick)
		return nil
	}), 100)
	select {
	case <-ctx.Done():
		return
	case <-tick:
	}

	// Sets up the trigger function for the module
	triggerFn := buildTrigger(self, rd)

	pObj := &PranaObj{
		This:    rd,
		Dom:     rd.state.dom,
		Element: self,
		Trigger: triggerFn,
	}

	G.Logf(3, "waitAndRender: calling Render() for %s\n", self.Get("_pranaTag").String())
	mod.Render(pObj)
}

// isConnected checks if the element is connected to the DOM.
func isConnected(self js.Value) bool {
	v := self.Get("_pranaConnected")
	return !v.IsUndefined() && v.Bool()
}

// triggerRoute is one handler dispatch produced by resolveTrigger: the handler
// name to look up in a prana ancestor, and whether the fired event's name must
// be prepended to the call args. prependEvent is true for the @all/@else
// catch-alls (which need to know which event fired) and false for a specific
// @<event> handler.
type triggerRoute struct {
	handler      string
	prependEvent bool
}

// resolveTrigger decides which handler(s) a fired event routes to, given a
// getter for the emitting element's attributes. It returns at most two routes
// in dispatch order:
//
//   - the primary route — the specific @<event> handler if declared, otherwise
//     the @else catch-all ("every event not otherwise wired"). These two can
//     never fire for the same event, so at most one of them appears;
//   - the @all spy route, if declared — always LAST, so an assertion driven
//     from it observes DOM/state already mutated by the primary handler.
//
// @all/@else handlers receive the event name as their first argument
// (prependEvent); specific handlers do not. resolveTrigger is pure (no DOM) so
// it is unit-testable without a document.
func resolveTrigger(getAttr func(name string) string, eventName string) []triggerRoute {
	routes := make([]triggerRoute, 0, 2)
	if named := getAttr("@" + eventName); named != "" {
		routes = append(routes, triggerRoute{handler: named})
	} else if elseH := getAttr("@else"); elseH != "" {
		routes = append(routes, triggerRoute{handler: elseH, prependEvent: true})
	}
	if allH := getAttr("@all"); allH != "" {
		routes = append(routes, triggerRoute{handler: allH, prependEvent: true})
	}
	return routes
}

// dispatchToAncestor walks up the prana ancestor chain from self and invokes
// the first ancestor that declares a non-placeholder handler named handlerName,
// passing args. Returns false if the chain is exhausted with no live handler.
//
// Triggers bubble through container prana elements: if the immediate parent
// does not declare the handler (e.g. a transparent wrapper like <w-tabs>
// hosting other prana children), the walk continues upward. This keeps
// "navFirst" (declared on the wlate root) reachable from <w-navbar> even when
// <w-tabs> sits between them.
func dispatchToAncestor(self js.Value, handlerName string, args []any) bool {
	for cur := findParentPranaElement(self); !cur.IsNull() && !cur.IsUndefined(); cur = findParentPranaElement(cur) {
		pst := getPranaState(cur)
		if pst == nil {
			continue
		}
		handler := getField(pst.Data.M, handlerName)
		if handler == nil {
			continue
		}
		switch fn := handler.(type) {
		case func(...any):
			if fn == nil {
				continue // nil placeholder — a real handler may sit higher up
			}
			G.Logf(4, "trigger: calling %q with %d args\n", handlerName, len(args))
			fn(args...)
			return true
		case TriggerHandler:
			// TriggerHandler(nil) is the conventional placeholder used in
			// InitData() while the real handler is wired up in Render().
			if fn == nil {
				continue
			}
			G.Logf(4, "trigger: calling %q with %d args\n", handlerName, len(args))
			fn(args...)
			return true
		default:
			G.Logf(1, "trigger: handler %q is not a function\n", handlerName)
			return false
		}
	}
	return false
}

// buildTrigger creates the trigger function that fires events from a child
// module to handlers declared on prana ancestors via @<event> attributes, plus
// the @all (spy: every event) and @else (catch-all: every un-wired event)
// channels. See resolveTrigger for routing and dispatchToAncestor for bubbling.
func buildTrigger(self js.Value, rd *ReactiveData) func(eventName string, args ...any) {
	return func(eventName string, args ...any) {
		routes := resolveTrigger(func(name string) string { return attrVal(self, name) }, eventName)
		if len(routes) == 0 {
			G.Logf(4, "trigger: %q has no @%s, @else or @all on %s\n",
				eventName, eventName, self.Get("_pranaTag").String())
			return
		}
		for _, r := range routes {
			callArgs := args
			if r.prependEvent {
				callArgs = append([]any{eventName}, args...)
			}
			if !dispatchToAncestor(self, r.handler, callArgs) {
				G.Logf(3, "trigger: %q without handler %q in any prana ancestor\n", eventName, r.handler)
			}
		}
	}
}

// ── Prana navigation helpers ────────────────────────────────────────────────

// findParentPranaElement searches for the closest ancestor that is a prana element.
func findParentPranaElement(self js.Value) js.Value {
	cur := self.Get("parentNode")
	for !cur.IsNull() && !cur.IsUndefined() {
		// Checks if it is a shadow host (traverses shadow boundaries)
		host := cur.Get("host")
		if !host.IsUndefined() && !host.IsNull() {
			cur = host
		}
		if !cur.Get("_pranaTag").IsUndefined() {
			return cur
		}
		cur = cur.Get("parentNode")
	}
	return js.Null()
}

// getPranaState returns the PranaState of a prana element by its nodeID.
func getPranaState(el js.Value) *PranaState {
	st := getState(el)
	if st == nil {
		return nil
	}
	return st.State
}

// ── onChange: external observer ──────────────────────────────────────────────

// OnChange creates an external observer on a data map.
// The callback fn is called with ("S"=set/"D"=delete, target, property, value).
// Equivalent to the prana.onChange() from the original JS.
// Returns an *ObservedData that encapsulates the data with notification.
type ObservedData struct {
	M  map[string]any
	fn func(op string, target map[string]any, property string, value any)
}

func OnChange(data map[string]any, fn func(op string, target map[string]any, property string, value any)) *ObservedData {
	return &ObservedData{M: data, fn: fn}
}

func (o *ObservedData) Set(key string, val any) {
	o.M[key] = val
	if o.fn != nil {
		go func() {
			done := make(chan struct{})
			jsGlobal.Call("setTimeout", js.FuncOf(func(this js.Value, args []js.Value) any {
				o.fn("S", o.M, key, val)
				close(done)
				return nil
			}), 100)
			<-done
		}()
	}
}

func (o *ObservedData) Delete(key string) {
	delete(o.M, key)
	if o.fn != nil {
		go func() {
			done := make(chan struct{})
			jsGlobal.Call("setTimeout", js.FuncOf(func(this js.Value, args []js.Value) any {
				o.fn("D", o.M, key, nil)
				close(done)
				return nil
			}), 100)
			<-done
		}()
	}
}

// ── Dynamic CSS update ──────────────────────────────────────────────────────

// Update replaces the CSS of an already registered custom element and updates
// the <style> in the Shadow DOM of all live instances.
// Must be called by Customizable modules when ReplaceCSS is invoked.
func Update(tagName string, cssContent string) {
	def, exists := moduleRegistry[tagName]
	if !exists {
		G.Logf(1, "Update: module %q not found\n", tagName)
		return
	}
	def.css = cssContent

	// Updates the <style> of all live instances
	for _, self := range instanceRegistry[tagName] {
		// For closed shadow roots element.shadowRoot returns null; use the
		// reference stored at construction time instead.
		var shadowRoot js.Value
		if st := getState(self); st != nil && !st.ShadowRoot.IsNull() && !st.ShadowRoot.IsUndefined() {
			shadowRoot = st.ShadowRoot
		} else {
			shadowRoot = self.Get("shadowRoot")
		}
		if shadowRoot.IsNull() || shadowRoot.IsUndefined() {
			continue
		}
		styleNode := shadowRoot.Call("querySelector", "style")
		if styleNode.IsNull() || styleNode.IsUndefined() {
			// Instance without <style> (css was empty in the original Register);
			// creates a new <style> as the first child.
			styleNode = domCreateStyleNode(cssContent)
			shadowRoot.Call("insertBefore", styleNode, shadowRoot.Get("firstChild"))
			continue
		}
		styleNode.Set("innerText", cssContent)
	}
}

// ── main ────────────────────────────────────────────────────────────────────

// Main must be called from main() to keep the WASM alive and define the
// custom elements. Blocks indefinitely.
//
// Before defining the modules, Main() waits on InitWG. This allows side-effect
// packages (e.g. wi18n) to register asynchronous initialization via
// InitWG.Add(1) in their init() and InitWG.Done() when ready.
func Main() {
	G.Logf(2, "wings: starting")

	G.Logf(2, "wings: waiting for async initializers")
	InitWG.Wait()

	G.Logf(2, "wings: defining %d modules", len(moduleRegistry))
	DefineAll()

	// Keep the WASM runtime alive. The deadlock detector does not consider
	// pending js.FuncOf callbacks as future work, so if every Go goroutine
	// is parked on a channel (including the ones inside fetch wrappers used
	// by event handlers like wi18n.SetLang) it will panic with
	// "all goroutines are asleep". A perpetual timer registers a runtime
	// timer entry, which IS recognised as future work and keeps the
	// detector quiet without busy-spinning.
	go func() {
		for range time.Tick(time.Hour) {
		}
	}()
	<-make(chan struct{})
}

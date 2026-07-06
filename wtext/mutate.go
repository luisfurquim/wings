//go:build js && wasm

package wtext

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/wings/epubhtml"
)

// EditorCore reads and writes over the live DOM. Every write follows the
// same bracket: beginWrite (closes any pending typing step), mutate,
// commitWrite (drain the mutations into one undo step or the open Txn).

// ── Reads ───────────────────────────────────────────────────────────────

// Text returns the plain text covered by s.
func (e *Editor) Text(s Selection) (string, error) {
	rng, err := e.rangeFor(s)
	if err != nil {
		return "", err
	}
	return rng.Call("toString").String(), nil
}

// InMark reports whether both ends of s sit inside a tag mark element. At
// a collapsed caret a pending mark overrides the tree: an armed toggle
// reads as already applied, so the toolbar reflects what the next typing
// will produce.
func (e *Editor) InMark(s Selection, tag string) (bool, error) {
	from, err := e.resolve(s.From)
	if err != nil {
		return false, err
	}
	to, err := e.resolve(s.To)
	if err != nil {
		return false, err
	}
	if s.Collapsed() {
		if p, ok := e.pending[strings.ToLower(tag)]; ok {
			return p.on, nil
		}
	}
	return e.markAncestor(from, tag).Truthy() &&
		e.markAncestor(to, tag).Truthy(), nil
}

// BlockType returns the tag of the block containing the start of s.
func (e *Editor) BlockType(s Selection) (string, error) {
	from, err := e.resolve(s.From)
	if err != nil {
		return "", err
	}
	block := e.blockAncestor(from)
	if !block.Truthy() {
		return "", nil
	}
	return strings.ToLower(block.Get("tagName").String()), nil
}

// HasClass reports whether the block containing the start of s carries
// the named class.
func (e *Editor) HasClass(s Selection, name string) (bool, error) {
	from, err := e.resolve(s.From)
	if err != nil {
		return false, err
	}
	block := e.blockAncestor(from)
	if !block.Truthy() {
		return false, nil
	}
	return block.Get("classList").Call("contains", name).Bool(), nil
}

// markAncestor returns the nearest tag mark element containing node
// (undefined when none exists inside the editor root).
func (e *Editor) markAncestor(node js.Value, tag string) js.Value {
	tag = strings.ToLower(tag)
	for cur := node; cur.Truthy() && !cur.Equal(e.root); cur = cur.Get("parentNode") {
		if cur.Get("nodeType").Int() == 1 &&
			strings.ToLower(cur.Get("tagName").String()) == tag {
			return cur
		}
	}
	return js.Undefined()
}

// blockAncestor returns the nearest profile block containing node.
func (e *Editor) blockAncestor(node js.Value) js.Value {
	for cur := node; cur.Truthy() && !cur.Equal(e.root); cur = cur.Get("parentNode") {
		if cur.Get("nodeType").Int() == 1 &&
			epubhtml.IsBlock(strings.ToLower(cur.Get("tagName").String())) {
			return cur
		}
	}
	return js.Undefined()
}

// ── Range plumbing ──────────────────────────────────────────────────────

// textSlice is one text node's intersection with a range, computed as a
// snapshot BEFORE any mutation (splitting reshuffles offsets).
type textSlice struct {
	node       js.Value
	start, end int
}

// textSlices collects the text nodes covered by rng with their in-node
// boundaries.
func (e *Editor) textSlices(rng js.Value) []textSlice {
	var out []textSlice
	walker := e.doc.Call("createTreeWalker", e.root, 4 /* NodeFilter.SHOW_TEXT */)
	for {
		node := walker.Call("nextNode")
		if !node.Truthy() {
			break
		}
		if !rng.Call("intersectsNode", node).Bool() {
			continue
		}
		s := textSlice{node: node, end: node.Get("length").Int()}
		if node.Equal(rng.Get("startContainer")) {
			s.start = rng.Get("startOffset").Int()
		}
		if node.Equal(rng.Get("endContainer")) {
			s.end = rng.Get("endOffset").Int()
		}
		if s.start < s.end {
			out = append(out, s)
		}
	}
	return out
}

// carve isolates the [start,end) portion of a slice's text node,
// returning the exact node to act on.
func carve(s textSlice) js.Value {
	node := s.node
	length := node.Get("length").Int()
	if s.end < length {
		node.Call("splitText", s.end)
	}
	if s.start > 0 {
		node = node.Call("splitText", s.start)
	}
	return node
}

// ── Writes ──────────────────────────────────────────────────────────────

// Wrap applies a semantic mark to the text covered by s. A collapsed s
// arms the mark as pending instead: it applies to the next text typed at
// the caret (see pending.go).
func (e *Editor) Wrap(s Selection, m Mark) error {
	if m.tag == "" {
		return ErrBadMark
	}
	href := ""
	if m.tag == "a" {
		canon, err := epubhtml.CanonicalizeHref(m.href, e.profile.LinkPolicy)
		if err != nil {
			return err
		}
		href = canon
	}
	if s.Collapsed() {
		m.href = href
		return e.armPendingOn(s, m)
	}
	rng, err := e.rangeFor(s)
	if err != nil {
		return err
	}
	e.beginWrite()
	defer e.commitWrite()
	for _, slice := range e.textSlices(rng) {
		if e.markAncestor(slice.node, m.tag).Truthy() {
			continue // already marked
		}
		node := carve(slice)
		wrapper := e.doc.Call("createElement", m.tag)
		if href != "" {
			wrapper.Call("setAttribute", "href", href)
			if strings.HasPrefix(href, "http://") {
				wrapper.Call("setAttribute", "data-wings-insecure", "")
			}
		}
		node.Get("parentNode").Call("insertBefore", wrapper, node)
		wrapper.Call("appendChild", node)
	}
	return nil
}

// Unwrap removes the mark tag from the text covered by s. Partially
// covered mark elements are split so only the covered portion loses the
// mark. A collapsed s arms a pending removal instead: the next text typed
// at the caret escapes the mark (see pending.go).
func (e *Editor) Unwrap(s Selection, tag string) error {
	if !epubhtml.IsMark(tag) {
		return ErrBadMark
	}
	if s.Collapsed() {
		return e.armPendingOff(s, tag)
	}
	rng, err := e.rangeFor(s)
	if err != nil {
		return err
	}
	e.beginWrite()
	defer e.commitWrite()
	for _, slice := range e.textSlices(rng) {
		mark := e.markAncestor(slice.node, tag)
		if !mark.Truthy() {
			continue
		}
		node := carve(slice)
		e.liftOutOf(node, mark)
	}
	return nil
}

// liftOutOf extracts node from inside mark, splitting mark around it:
// siblings after node (within mark) move to a shallow clone inserted
// after mark, node lands between the two, and emptied husks are removed.
func (e *Editor) liftOutOf(node, mark js.Value) {
	// Climb: split every level between node and mark.
	for {
		parent := node.Get("parentNode")
		if !parent.Truthy() {
			return
		}
		after := parent.Call("cloneNode", false)
		for sib := node.Get("nextSibling"); sib.Truthy(); sib = node.Get("nextSibling") {
			after.Call("appendChild", sib)
		}
		grand := parent.Get("parentNode")
		grand.Call("insertBefore", after, parent.Get("nextSibling"))
		grand.Call("insertBefore", node, after)
		if !parent.Call("hasChildNodes").Bool() {
			parent.Call("remove")
		}
		if !after.Call("hasChildNodes").Bool() {
			after.Call("remove")
		}
		if parent.Equal(mark) {
			return
		}
	}
}

// SetBlock converts every block touched by s to tag. Attributes cross
// only if the target tag's policy admits them, values re-checked — an
// attribute legal on the old block is not automatically legal on the new.
func (e *Editor) SetBlock(s Selection, tag string) error {
	tag = strings.ToLower(tag)
	if !epubhtml.IsBlock(tag) {
		return ErrBadBlock
	}
	blocks, err := e.blocksIn(s)
	if err != nil {
		return err
	}
	e.beginWrite()
	defer e.commitWrite()
	for _, block := range blocks {
		if strings.ToLower(block.Get("tagName").String()) == tag {
			continue
		}
		repl := e.doc.Call("createElement", tag)
		names := block.Call("getAttributeNames")
		for i := 0; i < names.Get("length").Int(); i++ {
			name := strings.ToLower(names.Index(i).String())
			if epubhtml.AttrFor(tag, name) == epubhtml.AttrDrop {
				continue
			}
			// Same-kind policy on the target tag: copy, value re-checked
			// by kind (class names against the registry).
			val := block.Call("getAttribute", name).String()
			if epubhtml.AttrFor(tag, name) == epubhtml.AttrClass {
				var keep []string
				for _, cls := range strings.Fields(val) {
					if e.classDefined(cls) {
						keep = append(keep, cls)
					}
				}
				if len(keep) == 0 {
					continue
				}
				val = strings.Join(keep, " ")
			}
			repl.Call("setAttribute", name, val)
		}
		for block.Call("hasChildNodes").Bool() {
			repl.Call("appendChild", block.Get("firstChild"))
		}
		block.Get("parentNode").Call("replaceChild", repl, block)
	}
	return nil
}

// blocksIn returns the distinct blocks touched by s.
func (e *Editor) blocksIn(s Selection) ([]js.Value, error) {
	rng, err := e.rangeFor(s)
	if err != nil {
		return nil, err
	}
	var blocks []js.Value
	add := func(b js.Value) {
		if !b.Truthy() {
			return
		}
		for _, seen := range blocks {
			if seen.Equal(b) {
				return
			}
		}
		blocks = append(blocks, b)
	}
	slices := e.textSlices(rng)
	if len(slices) == 0 {
		// Collapsed selection: the block under the caret.
		from, rerr := e.resolve(s.From)
		if rerr != nil {
			return nil, rerr
		}
		add(e.blockAncestor(from))
		return blocks, nil
	}
	for _, slice := range slices {
		add(e.blockAncestor(slice.node))
	}
	return blocks, nil
}

// ApplyClass adds a registered class to every block touched by s.
func (e *Editor) ApplyClass(s Selection, name string) error {
	return e.eachBlockClass(s, name, "add")
}

// RemoveClass removes the class from every block touched by s.
func (e *Editor) RemoveClass(s Selection, name string) error {
	return e.eachBlockClass(s, name, "remove")
}

// eachBlockClass applies a classList verb on the selection's blocks.
func (e *Editor) eachBlockClass(s Selection, name, verb string) error {
	if err := epubhtml.ValidClassName(name); err != nil {
		return err
	}
	if !e.classDefined(name) {
		return fmt.Errorf("%w: %q", ErrUnknownClass, name)
	}
	blocks, err := e.blocksIn(s)
	if err != nil {
		return err
	}
	e.beginWrite()
	defer e.commitWrite()
	for _, block := range blocks {
		block.Get("classList").Call(verb, name)
	}
	return nil
}

// Delete removes the content covered by s.
func (e *Editor) Delete(s Selection) error {
	rng, err := e.rangeFor(s)
	if err != nil {
		return err
	}
	e.beginWrite()
	defer e.commitWrite()
	rng.Call("deleteContents")
	e.canonicalize()
	return nil
}

// Replace substitutes the content covered by s with a Fragment. Inline
// fragments flow in place; a fragment carrying blocks splits the target
// block and lays its blocks between the halves.
func (e *Editor) Replace(s Selection, f Fragment) error {
	if len(f.errs) > 0 {
		return fmt.Errorf("%w: %v", ErrBadFragment, f.errs)
	}
	rng, err := e.rangeFor(s)
	if err != nil {
		return err
	}
	e.beginWrite()
	defer e.commitWrite()
	rng.Call("deleteContents")
	if !f.Empty() {
		if fragmentHasBlocks(f) {
			e.insertBlocks(rng, f)
		} else {
			rng.Call("insertNode", e.materialize(f))
		}
	}
	e.canonicalize()
	return nil
}

// fragmentHasBlocks reports whether any top-level node is a block.
func fragmentHasBlocks(f Fragment) bool {
	for i := range f.nodes {
		if f.nodes[i].tag != "" && epubhtml.IsBlock(f.nodes[i].tag) {
			return true
		}
	}
	return false
}

// insertBlocks inserts a block-carrying fragment at the (collapsed)
// range: the enclosing block is split at the caret and the fragment's
// nodes land between the halves.
func (e *Editor) insertBlocks(rng js.Value, f Fragment) {
	container := rng.Get("startContainer")
	block := e.blockAncestor(container)
	if !block.Truthy() {
		// Caret at root level: append at the range position directly.
		rng.Call("insertNode", e.materialize(f))
		return
	}
	// Split the block: everything after the caret moves to a clone.
	splitRange := e.doc.Call("createRange")
	splitRange.Call("setStart", container, rng.Get("startOffset").Int())
	splitRange.Call("setEndAfter", block.Get("lastChild"))
	tail := splitRange.Call("extractContents")
	after := block.Call("cloneNode", false)
	after.Call("appendChild", tail)
	parent := block.Get("parentNode")
	parent.Call("insertBefore", after, block.Get("nextSibling"))
	parent.Call("insertBefore", e.materialize(f), after)
}

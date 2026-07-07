// Package wtext is the engine of the w-text rich editor: a pluggable
// contenteditable core whose every write goes through the epubhtml content
// policy, so forbidden markup cannot exist in editor content by
// construction.
//
// The package splits along the same policy/mechanism line as the rest of
// wings. This file and its siblings without a build tag are portable: the
// plugin API (EditorCore, plugins, toolbar items), the safe-by-construction
// Fragment builder, the Mark constructors and the undo bookkeeping compile
// and test under the native toolchain — a plugin author can unit-test
// against a fake EditorCore without a browser. The js/wasm files implement
// the mechanism: the filtering walker, the event spine, mutation and undo
// over the live DOM.
package wtext

import (
	"errors"

	"github.com/luisfurquim/wings/epubhtml"
)

// Point addresses a position in the editor. Node is an opaque handle
// minted by the core — never a DOM reference — valid only until the end of
// the current pipeline turn; a plugin that wants to act on the next event
// re-queries Sel(). Offset counts UTF-16 code units, the DOM's native text
// unit.
type Point struct {
	Node   int
	Offset int
}

// Selection is a directed range between two Points. From and To may be
// equal (a caret).
type Selection struct {
	From, To Point
}

// Collapsed reports whether the selection is a caret.
func (s Selection) Collapsed() bool { return s.From == s.To }

// Errors returned by EditorCore implementations.
var (
	// ErrStaleSelection reports a Point whose handle expired, whose node
	// left the document, or whose offset no longer fits the node.
	ErrStaleSelection = errors.New("wtext: stale selection")
	// ErrBadMark reports a zero Mark (one not made by a constructor).
	ErrBadMark = errors.New("wtext: invalid mark")
	// ErrBadBlock reports a tag outside the profile's block set.
	ErrBadBlock = errors.New("wtext: tag is not an allowed block")
	// ErrUnknownClass reports a class name not defined via DefineClass.
	ErrUnknownClass = errors.New("wtext: class not defined via DefineClass")
	// ErrBadFragment reports a Fragment whose Builder recorded errors.
	ErrBadFragment = errors.New("wtext: invalid fragment")
	// ErrReservedClass reports a user style name in the wt- namespace,
	// which belongs to wings' own utility classes.
	ErrReservedClass = errors.New("wtext: the wt- class prefix is reserved")
	// ErrNoFormatting reports a create-style call over a selection that
	// carries no classes to capture.
	ErrNoFormatting = errors.New("wtext: selection carries no formatting to capture")
	// ErrNoSelection reports an action that needs a selection inside the
	// editor when there is none.
	ErrNoSelection = errors.New("wtext: no selection inside the editor")
)

// EditorCore is a plugin's only gate to the document. Reads return plain
// values (no DOM node ever crosses this interface); writes are filtered by
// the epubhtml policy and become undo steps — one step per call, or one
// per Txn when grouped.
type EditorCore interface {
	// Sel returns the current selection. ok is false when the document
	// selection is not inside the editor.
	Sel() (sel Selection, ok bool)
	// Text returns the plain text covered by s.
	Text(s Selection) (string, error)
	// InMark reports whether both ends of s sit inside a mark element tag.
	// At a collapsed s an armed pending mark overrides the tree, so a
	// toggle reads back what the next typing will produce.
	InMark(s Selection, tag string) (bool, error)
	// BlockType returns the tag of the block containing the start of s.
	BlockType(s Selection) (string, error)
	// HasClass reports whether the block containing the start of s
	// carries the named class.
	HasClass(s Selection, name string) (bool, error)
	// Classes returns every class name registered via DefineClass, sorted.
	Classes() []string
	// ClassCSS returns the sanitized CSS registered for name.
	ClassCSS(name string) (css string, ok bool)
	// ClassesAt returns the classes in effect at the start of s, ordered
	// outside-in: the containing block's classes first, then each
	// enclosing span's, outermost first — so merging their CSS in order
	// makes the innermost formatting win. At a collapsed s armed pending
	// classes overlay the tree, like InMark.
	ClassesAt(s Selection) ([]string, error)

	// Wrap applies a semantic mark to the text covered by s. A collapsed
	// s arms the mark as pending: it applies to the next text typed at
	// the caret and disarms if the caret moves first.
	Wrap(s Selection, m Mark) error
	// Unwrap removes the mark tag from the text covered by s. A collapsed
	// s arms a pending removal: the next text typed at the caret escapes
	// the mark.
	Unwrap(s Selection, tag string) error
	// SetBlock converts every block touched by s to tag. Attributes are
	// re-filtered against the target tag's policy.
	SetBlock(s Selection, tag string) error
	// ApplyClass applies a registered class to s with the Word split: the
	// class's character declarations ride spans over the exact range, its
	// paragraph declarations mark every block touched by s. A collapsed s
	// arms the character half as pending, like Wrap.
	ApplyClass(s Selection, name string) error
	// RemoveClass removes the class from the spans and blocks touched by
	// s. A collapsed s arms a pending removal, like Unwrap.
	RemoveClass(s Selection, name string) error
	// DefineClass registers a named style: the name becomes usable in
	// class attributes and the CSS (sanitized) is installed with the
	// editor. Defining an existing name replaces its CSS.
	DefineClass(name, css string) error
	// Replace substitutes the content covered by s with a Fragment.
	Replace(s Selection, f Fragment) error
	// Delete removes the content covered by s.
	Delete(s Selection) error
	// Txn groups every write fn performs into a single undo step. An
	// error rolls the group back and is returned.
	Txn(fn func(EditorCore) error) error
}

// KeyCtx is the portable projection of a keyboard event, handed to
// EditionPlugin.OnKey. Consume and Stop control the pipeline from inside
// the handler — they are meaningless anywhere else, which is why they live
// here and not on EditorCore.
type KeyCtx struct {
	Key                    string // KeyboardEvent.key ("b", "Enter", "ArrowLeft"...)
	Code                   string // KeyboardEvent.code ("KeyB"...)
	Ctrl, Alt, Shift, Meta bool

	consumed, stopped bool
}

// Consume suppresses the browser's default action for this key.
func (c *KeyCtx) Consume() { c.consumed = true }

// Stop ends the OnKey broadcast: later plugins do not see this event.
func (c *KeyCtx) Stop() { c.stopped = true }

// EditionPlugin hooks the two-phase editing pipeline: OnKey runs before
// the input happens (cancelable via KeyCtx — shortcuts live here) and
// OnChanged runs after a change settled (reactive — highlighters,
// autocompleters and friends coexist; nobody consumes).
type EditionPlugin interface {
	OnKey(core EditorCore, k *KeyCtx)
	OnChanged(core EditorCore, s Selection)
}

// ClipboardPlugin transforms pasted or dropped content. It runs after the
// core sanitized the payload into a Fragment and before insertion — a
// plugin never sees, and can never leak, the raw clipboard markup.
type ClipboardPlugin interface {
	OnPaste(core EditorCore, f Fragment) (Fragment, error)
}

// ToolbarPlugin declares toolbar items; the w-text widget renders them
// with wings' own widgets (w-button, w-combobox) and keeps their state
// fresh on selection changes.
type ToolbarPlugin interface {
	Items() []ToolbarItem
}

// ToolbarItem is a sealed per-kind interface: only this package defines
// the kinds the widget knows how to draw. Future custom controls extend
// the sealed set here rather than opening it.
type ToolbarItem interface{ isToolbarItem() }

// ToggleItem is a two-state button (bold, italic...). Label is a message
// id resolved through the i18n catalog.
type ToggleItem struct {
	ID, Label, Icon string
	Do              func(EditorCore) error
	Active          func(EditorCore) bool
}

func (ToggleItem) isToolbarItem() {}

// ButtonItem is a plain action button.
type ButtonItem struct {
	ID, Label, Icon string
	Do              func(EditorCore) error
	Enabled         func(EditorCore) bool
}

func (ButtonItem) isToolbarItem() {}

// Option is one choice of a SelectItem.
type Option struct {
	Value, Label string
}

// SelectItem is a dropdown (block style picker...).
type SelectItem struct {
	ID, Label string
	Options   func(EditorCore) []Option
	Current   func(EditorCore) string
	Pick      func(EditorCore, string) error
}

func (SelectItem) isToolbarItem() {}

// InputItem is a button that opens a small text prompt — a w-input in a
// popover; confirming calls Do with the typed value. Label and
// Placeholder are message ids resolved through the i18n catalog. It is
// the affordance for values a click cannot express: a style name, a
// link's URL.
type InputItem struct {
	ID, Label, Icon, Placeholder string
	Do                           func(EditorCore, string) error
}

func (InputItem) isToolbarItem() {}

// Separator draws a divider between item groups.
type Separator struct{}

func (Separator) isToolbarItem() {}

// InitPlugin is an optional interface any plugin may implement to run
// setup against the editor at attach time — defining the utility classes
// a toolbar will apply, for instance. It runs once per editor instance,
// before content loads, so persisted documents using those classes
// survive the class filter.
type InitPlugin interface {
	Init(core EditorCore) error
}

// Profile is the pluggable behaviour of one editor instance. Templates
// select a registered profile by name: <w-text profile="basic">.
type Profile struct {
	Toolbar   []ToolbarPlugin
	Edition   []EditionPlugin
	Clipboard []ClipboardPlugin
	// LinkPolicy optionally restricts where links may point (host
	// allowlists and such). It runs on top of the structural URL
	// canonicalization, which is not optional.
	LinkPolicy epubhtml.LinkPolicy
}

// profiles is the named-profile registry.
var profiles = map[string]Profile{}

// RegisterProfile makes a profile selectable from templates by name. A
// later call for the same name replaces the previous profile.
func RegisterProfile(name string, p Profile) {
	profiles[name] = p
}

// ProfileFor returns the registered profile, or ok=false.
func ProfileFor(name string) (p Profile, ok bool) {
	p, ok = profiles[name]
	return p, ok
}

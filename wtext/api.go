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
	"fmt"

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
	// ErrConfigKey reports a config key that is empty, too long, or
	// carries whitespace/control/quote characters.
	ErrConfigKey = errors.New("wtext: invalid config key")
	// ErrConfigValue reports a config value over MaxConfigValueLen.
	ErrConfigValue = errors.New("wtext: config value too long")
	// ErrConfigFull reports a store already holding MaxConfigKeys entries.
	ErrConfigFull = errors.New("wtext: config store full")
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
	// DocText returns the plain text of the whole document, with a
	// newline at every block boundary and <br> — a bare textContent read
	// would fuse the last word of one paragraph with the first of the
	// next. It is the whole-document read for passive observers (word
	// counters and such): a plugin cannot ask for it through Text, since
	// Point handles are minted by the core and no Selection spanning the
	// document can be fabricated from outside.
	DocText() string
	// Content serializes the whole document as a complete EPUB-style
	// content document — the same string the widget persists through
	// &value: body markup (which by construction only holds what the
	// policy let in) plus the registered classes the tree actually uses,
	// as rules in a head <style>. It is the whole-document read for
	// exporters (EPUB packagers and such). The markup is the browser's
	// HTML serialization; an XML consumer re-serializes it (see the
	// wtextepub module).
	Content() string
	// SetContent replaces the WHOLE document with html — the write half of
	// Content, for importers (an EPUB reader and such). The string is
	// hostile input like any other and buys no trust from having been
	// handed over by a plugin: it goes through the same DOMParser + policy
	// walker a paste does, its head properties and class rules are
	// re-validated one by one, and its remembered webfonts are re-followed
	// through the store allowlist. The undo stack is cleared — history
	// cannot reach across a content load — so an import is not undoable:
	// a plugin about to discard the user's document should ask first
	// (see PendingDecision).
	SetContent(html string) error
	// InMark reports whether both ends of s sit inside a mark element tag.
	// At a collapsed s an armed pending mark overrides the tree, so a
	// toggle reads back what the next typing will produce.
	InMark(s Selection, tag string) (bool, error)
	// BlockType returns the tag of the block containing the start of s.
	BlockType(s Selection) (string, error)
	// HasClass reports whether the block containing the start of s
	// carries the named class.
	HasClass(s Selection, name string) (bool, error)
	// ClassSpanned reports whether name is applied at s via an actual
	// <span class> — the character half of the Word split — as opposed to
	// merely inherited from the enclosing block's own class list. A mixed
	// style (character + paragraph declarations, e.g. one captured by
	// CreateStyle) shows up in ClassesAt for the whole block once its
	// paragraph half is applied anywhere in it; this method is how a
	// caller distinguishes "the block has it" from "this exact range has
	// its character formatting too", so "is this style fully current
	// here" doesn't report a false positive for untouched text sharing
	// the block with styled text.
	ClassSpanned(s Selection, name string) (bool, error)
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
	// StyleLayersAt returns the CSS actually in effect at the start of s,
	// as declaration lists ordered outside-in like ClassesAt — merging
	// them in order (epubhtml.MergeCSS) is "the formatting the user is
	// looking at".
	//
	// It is NOT ClassesAt plus a lookup. Since a document's own stylesheet
	// is carried with its selectors intact, most of an imported book's
	// formatting comes from rules that are not registered classes at all
	// — a `span.dropcaps` the editor never named — and a caller walking
	// ClassesAt sees nothing of it. This reports both: the declarations of
	// every document rule matching an element in the chain, and of every
	// registered class those elements carry.
	//
	// Rule DECLARATIONS, deliberately, not computed values: a computed
	// style cannot say where a value came from, so it hands back inherited
	// and initial values indistinguishable from declared ones, and a style
	// built out of it stops describing the formatting and starts imposing
	// a whole context wherever it is applied.
	StyleLayersAt(s Selection) ([]string, error)
	// IsDocumentClass reports whether a preserved document rule SELECTS on
	// this class name — whether the class is a hook the loaded document's
	// own stylesheet depends on.
	//
	// Such a class is not the editor's to strip. A book's chapter title is
	// `<p class="chtitle">` and its face comes from `.chtitle *`; take the
	// class off and the rule stops matching, so the title loses its
	// typography the instant something "tidies up" its classes. Note that
	// being registered says nothing here: `chtitle` is BOTH a named style
	// (the sheet also had a plain `.chtitle` rule) and a hook.
	IsDocumentClass(name string) bool

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
	// Config reads a document property: the value stored in THIS document
	// (they persist inside Content()'s head and travel with &value),
	// falling back to the default the property's ConfigPlugin declared,
	// then to "". Keys are namespaced section.field (see ConfigKey) and
	// are a commons: any plugin may read any property — the EPUB exporter
	// reading the page margins a ruler plugin declared is the point.
	// Values are plain strings; whoever USES one runs it through the
	// validator of its destination (CSS, XML...), never the store.
	Config(key string) string
	// SetConfig stores a document property (bounded; see MaxConfig*). An
	// empty value deletes the stored entry, falling back to the default.
	SetConfig(key, value string) error

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
// id resolved through the i18n catalog. Help is a message id for this
// item's entry in the toolbar's composed help dialog; empty omits the
// item from it.
type ToggleItem struct {
	ID, Label, Icon, Help string
	Do                    func(EditorCore) error
	Active                func(EditorCore) bool
}

func (ToggleItem) isToolbarItem() {}

// ButtonItem is a plain action button. Help is a message id for this
// item's entry in the toolbar's composed help dialog; empty omits the
// item from it.
type ButtonItem struct {
	ID, Label, Icon, Help string
	Do                    func(EditorCore) error
	Enabled               func(EditorCore) bool
}

func (ButtonItem) isToolbarItem() {}

// Option is one choice of a SelectItem.
type Option struct {
	Value, Label string
	// Font, when set, previews the option in its own typeface: the widget
	// assigns it to the rendered option's style.fontFamily — a property
	// assignment, which the browser parses as a single value, so the
	// string cannot smuggle other declarations the way interpolating into
	// a style attribute could. Face pickers set it; most options don't.
	Font string
}

// SelectItem is a dropdown (block style picker...). Help is a message id
// for this item's entry in the toolbar's composed help dialog; empty
// omits the item from it.
type SelectItem struct {
	ID, Label, Help string
	Options         func(EditorCore) []Option
	Current         func(EditorCore) string
	Pick            func(EditorCore, string) error
	// NotInList, when set, receives text the user confirmed (Enter) that
	// matches no option — the face picker's webfont URL drop rides this.
	// Optional; absent, unmatched text just does nothing.
	NotInList func(EditorCore, string) error
}

func (SelectItem) isToolbarItem() {}

// InputItem is a button that opens a small text prompt — a w-input in a
// popover; confirming calls Do with the typed value. Label and
// Placeholder are message ids resolved through the i18n catalog. Help is
// a message id for this item's entry in the toolbar's composed help
// dialog; empty omits the item from it. It is the affordance for values a
// click cannot express: a style name, a link's URL.
type InputItem struct {
	ID, Label, Icon, Placeholder, Help string
	Do                                 func(EditorCore, string) error
}

func (InputItem) isToolbarItem() {}

// StatusItem is a passive read-out — the widget refreshes it alongside
// toggle/select state, and it never performs an action. Label is a
// message id naming the item (tooltip and its row in the help dialog).
// Format is a message id whose resolved text is a fmt template the
// widget fills with Args' values: the plugin computes the numbers, the
// catalog owns their presentation, and a translator reorders with
// explicit indexes (%[2]d) when the language needs it. Help is a message
// id for this item's entry in the composed help dialog; empty omits it.
type StatusItem struct {
	ID, Label, Format, Help string
	Args                    func(EditorCore) []any
}

func (StatusItem) isToolbarItem() {}

// Separator draws a divider between item groups.
type Separator struct{}

func (Separator) isToolbarItem() {}

// MenuPlugin declares items for the editor's side menu — document-level
// actions (export, import...) that would clutter the toolbar, whose
// vocabulary is formatting used keystroke-by-keystroke. The widget
// renders the menu as an accordion column beside the editing surface,
// one section per distinct Group, in first-seen declaration order — and
// renders NO menu at all when no plugin declares an item, so profiles
// without document actions keep today's exact layout.
type MenuPlugin interface {
	MenuItems() []MenuItem
}

// MenuItem is a sealed per-kind interface, ToolbarItem's counterpart for
// the side menu: only this package defines the kinds the widget knows
// how to draw.
type MenuItem interface{ isMenuItem() }

// MenuAction is a plain action inside a menu group. Group is the message
// id of the section it lands in — wtext-export and wtext-import are the
// standard ids (with built-in defaults); a plugin inventing a new group
// supplies its label like any other. Label names the item, Help is its
// entry in the composed help dialog (empty omits it), and Do runs on
// click against the live document.
type MenuAction struct {
	Group, ID, Label, Help string
	Do                     func(EditorCore) error
	Enabled                func(EditorCore) bool
}

func (MenuAction) isMenuItem() {}

// MenuInput is a menu action that needs a typed value first — a filename,
// a target — InputItem's counterpart for the side menu: the widget
// renders a button that opens a small prompt (a w-input in a popover) and
// confirming calls Do with the typed value. Value optionally seeds the
// prompt (a Save-As dialog never opens empty: Enter keeps the suggestion,
// typing replaces it — the text opens selected); the empty string leaves
// it blank. An empty confirmation is discarded, never delivered to Do.
type MenuInput struct {
	Group, ID, Label, Placeholder, Help string
	Value                               func(EditorCore) string
	Do                                  func(EditorCore, string) error
}

func (MenuInput) isMenuItem() {}

// DefaultUploadLen bounds a MenuUpload file whose MaxLen is 0.
const DefaultUploadLen = 1 << 20

// MenuUpload is a menu action that acts on a FILE the user picks —
// MenuInput's counterpart for import: the widget renders a button that
// opens the browser's file picker, reads the chosen file and calls Do
// with its bytes. The plugin never sees the file input, only the payload,
// which is hostile input like any other — Do parses and validates it.
type MenuUpload struct {
	Group, ID, Label, Help string
	// Accept filters what the picker offers (the <input accept> syntax,
	// e.g. ".json,application/json"). A convenience for the user, never a
	// check: a file that gets through still faces Do's own validation.
	Accept string
	// MaxLen bounds the file; 0 means DefaultUploadLen. A larger file is
	// refused before a byte of it reaches Go.
	MaxLen int
	Do     func(core EditorCore, data []byte) error
}

func (MenuUpload) isMenuItem() {}

// PendingDecision is the error a plugin action returns when it cannot
// finish without an answer only the user can give — an import about to
// overwrite existing styles, say. A plugin never touches the DOM, so it
// does not open a dialog: it DECLARES the question (a title, a message,
// what is at stake, the options) and the widget asks it, with the app's
// own widgets, in the app's language, then calls Resume with the chosen
// option's Value.
//
// Remember names the answer for the "don't ask again" checkbox: ticking
// it stores the PICKED OPTION under that key, so the widget can answer
// the same question by itself from then on — what gets remembered is the
// policy, not merely the silence.
type PendingDecision struct {
	Title, Message string // message ids
	// Detail lists what is at stake (the colliding style names...). It is
	// user data, shown as-is: never a message id.
	Detail []string
	// Options are the answers, in display order. The first is the one a
	// keyboard user reaches first; none is "default" — an unanswered
	// question resumes nothing.
	Options []DecisionOption
	// Remember is the storage key of the "don't ask again" answer; empty
	// means the question is always asked.
	Remember string
	// Resume finishes the action with the chosen option's Value. It runs
	// on the live document, possibly long after Do returned.
	Resume func(core EditorCore, choice string) error
}

// Error makes PendingDecision an error, so it rides the ordinary error
// return out of Do and the widget picks it out with errors.As.
func (d *PendingDecision) Error() string {
	return "wtext: pending decision: " + d.Title
}

// Valid reports whether the decision can actually be asked and answered:
// an option to pick and a Resume to call. The widget checks it before
// opening anything — a malformed decision degrades to a logged error,
// never a dialog the user cannot escape.
func (d *PendingDecision) Valid() bool {
	return d != nil && d.Resume != nil && len(d.Options) > 0
}

// Allows reports whether choice is one of the declared options. The
// widget validates a REMEMBERED answer through it: the store it comes
// from (localStorage) is user-writable, so what comes back is input, not
// state.
func (d *PendingDecision) Allows(choice string) bool {
	for _, o := range d.Options {
		if o.Value == choice {
			return true
		}
	}
	return false
}

// DecisionOption is one answer to a PendingDecision: the Value handed to
// Resume (and stored by "don't ask again"), and what the option says.
//
// Label is a message id, resolved through the app's catalog — right for
// the answers a plugin writes itself ("replace", "keep both"). Text is
// USER DATA, shown as-is, for answers that come out of the document being
// acted upon (the chapters of an imported book): sending those through
// the catalog would translate a chapter that happened to be titled like a
// message id. Text wins when both are set.
type DecisionOption struct {
	Value, Label, Text string
}

// ConfigPlugin declares sections of USER-editable document configuration
// — the iOS Settings model: each plugin registers the schema of what it
// needs configured (the EPUB exporter its book metadata, a page plugin
// its paper size and margins), the widget renders one central settings
// UI per section, and the VALUES live in the document itself, readable
// by every plugin (see EditorCore.Config).
type ConfigPlugin interface {
	ConfigSections() []ConfigSection
}

// ConfigSection is one settings page: a labelled group of fields. Its ID
// namespaces the fields' keys (section.field — see ConfigKey).
type ConfigSection struct {
	ID, Label, Help string
	Fields          []ConfigField
}

// ConfigField is a sealed per-kind interface, ToolbarItem's counterpart
// for settings fields: only this package defines the kinds the widget
// knows how to draw.
type ConfigField interface{ isConfigField() }

// ConfigText is a free-text property (a title, an author). Default seeds
// the value until the user edits it — the plugin's own zero-config
// experience, which a webdev tunes by constructing the plugin with
// different defaults.
type ConfigText struct {
	ID, Label, Help, Default string
}

func (ConfigText) isConfigField() {}

// ConfigChoice is a closed-vocabulary property (paper size, orientation).
type ConfigChoice struct {
	ID, Label, Help, Default string
	Options                  []Option
}

func (ConfigChoice) isConfigField() {}

// ConfigKey is the store key of a section's field: section.field.
func ConfigKey(sectionID, fieldID string) string { return sectionID + "." + fieldID }

// configDefaults flattens a profile's ConfigPlugins into the key→Default
// map the read chain falls back to.
func configDefaults(p Profile) map[string]string {
	defs := map[string]string{}
	for _, plug := range p.Config {
		for _, s := range plug.ConfigSections() {
			for _, f := range s.Fields {
				switch fd := f.(type) {
				case ConfigText:
					defs[ConfigKey(s.ID, fd.ID)] = fd.Default
				case ConfigChoice:
					defs[ConfigKey(s.ID, fd.ID)] = fd.Default
				}
			}
		}
	}
	return defs
}

// Bounds of the per-document config store — bounded everything: a hostile
// or runaway document must not grow memory or its own persisted head
// without limit.
const (
	MaxConfigKeys     = 256
	MaxConfigKeyLen   = 128
	MaxConfigValueLen = 4096
)

// maxDocClasses bounds how many class rules one document (or one style
// library file) may define. It lives here, in the portable half, because
// both the js loader and the native library parser answer to it.
const maxDocClasses = 256

// validateConfigKey enforces the key shape shared by the js store and the
// portable fake: non-empty, bounded, no control characters or whitespace
// (keys become <meta name="wt-cfg-KEY"> attributes in the persisted head).
func validateConfigKey(key string) error {
	if key == "" || len(key) > MaxConfigKeyLen {
		return fmt.Errorf("%w: %q", ErrConfigKey, key)
	}
	for _, r := range key {
		if r <= ' ' || r == 0x7f || r == '"' || r == '\'' {
			return fmt.Errorf("%w: %q", ErrConfigKey, key)
		}
	}
	return nil
}

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
	Menu      []MenuPlugin
	Config    []ConfigPlugin
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

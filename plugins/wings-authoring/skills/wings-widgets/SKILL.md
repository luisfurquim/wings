---
name: wings-widgets
description: Use WINGS's built-in widgets instead of hand-rolling them — button (w-button), text input (w-input), rich-text editor (w-text), tabbed containers (w-tabs/w-tabbutton/w-tab), dialogs (w-dialog), multi-select combobox (w-combobox), record navbar (w-navbar), and the skin picker (skin-switcher). Use when an app needs any of these controls.
---

# Built-in widgets

WINGS ships custom elements for common UI so you don't hand-roll them. Each is
**blank-imported** to register, then used as a tag in your template. They emit
**component events** routed to your parent handlers with `@event="handler"` —
the same mechanism in `wings-component` (handler is a `func(args ...any)` set in
`Render`; remember the `TriggerHandler(nil)` placeholder in `InitData`).

Note on names: `@event` attribute values (handler names) may be camelCase —
attribute *values* are not lowercased. But any **bound attribute name** you pass
a widget (e.g. `nav_input`) must be snake_case (gotcha #1).

## Button — `w-button`

```go
import _ "github.com/luisfurquim/wings/widget/button"
```
```html
<w-button variant="primary" @click="onSave">Save</w-button>
<w-button variant="danger" size="sm" shape="pill">Delete</w-button>
<w-button loading="true">Saving…</w-button>

<!-- icon slots -->
<w-button variant="primary">
  <svg slot="prefix">…</svg>
  Submit
</w-button>
```

Attributes:
- `variant` — `secondary` (default) | `primary` | `ghost` | `danger` | `success`
- `size`    — `sm` | `md` (default) | `lg`
- `shape`   — `default` | `pill` | `square`
- `type`    — native-like form behavior: inside a `<form>`, a click submits it
  (`requestSubmit`, so constraint validation runs first — invalid `w-input`s
  block it) or, with `type="reset"`, resets it; `type="button"` opts out.
  Irrelevant outside a form.
- `loading` — `"true"` shows a spinner; button is non-interactive while loading
  (and never submits)
- `disabled` — standard HTML; reflected to inner `<button>` for a11y + keyboard

**Boolean attributes take the value `"true"`** (not bare presence) because the
WINGS observed-attribute system uses `coerceToType` against the InitData default:
`loading="true"` sets the bool to `true`; bare `loading` sets it to `false`.
Exception: `disabled` is a standard HTML boolean and IS handled by bare presence.

Clicks bubble naturally through open shadow DOM — use `@click` on the host as normal.
No Go-side `Render` logic needed for clicks.

**CSS hooks**: `::part(root)`, `::part(prefix)`, `::part(label)`, `::part(suffix)`,
`::part(spinner)`. Host attributes: `[variant]`, `[size]`, `[shape]`, `[disabled]`,
`[loading]`. All `--wings-button-*` tokens apply (see `wings-skins`).

## Text input — `w-input`

```go
import _ "github.com/luisfurquim/wings/widget/input"
```
```html
<w-input label="Email" type="email" placeholder="you@example.com"
         helper="We'll never share this."
         required="true" clearable="true"
         @change="onEmailChange" @clear="onEmailClear"></w-input>

<!-- password with icon prefix via slot -->
<w-input label="Password" type="password">
  <svg slot="prefix">…</svg>
</w-input>

<!-- show validation error from parent -->
<w-input label="Username" error="{{user_error}}"></w-input>
```

Attributes (all observed — template re-syncs on change):
- `type`      — `text` (default) | `email` | `password` | `search` | `number` | `tel` | `url`
- `label`     — label text; activates the label zone. Use `<slot name="label">` for HTML.
- `placeholder`/`value`/`helper`/`error`/`maxlength` — as expected
- i18n: `label`, `helper`, `error` (and `placeholder`) are translatable
  attributes by default — `gen_i18n` indexes them and they switch on `SetLang`.
  No extra wiring; add `translate="no"` on the element to opt a value out.
- `size`      — `sm` | `md` (default) | `lg`
- `required`  — `"true"` shows the `*` mark (same boolean convention as `w-button`)
- `clearable` — `"true"` shows × when field has content
- `disabled`  — standard HTML; reflected to inner `<input>`

**Surface form** (outlined / filled / underlined) is controlled by the active
**Material skin**, not by a per-instance attribute. Apply `wings.ApplySkin("filled")`
(or `"underlined"`) globally; all inputs update immediately. See `wings-skins`.

Events (native semantics): `@input` args[0] = current string value, fires on
every keystroke; `@change` args[0] = current string value, fires when the user
leaves the field (blur), after the two-way write-back — a bound
FieldCodec/Validator has already run. `@clear` fires when × is clicked.

**Form participation**: `w-input` is form-associated. Inside a native `<form>`,
an invalid field (native constraints or a bound Validator) blocks
`form.checkValidity()`/submission; give the host a `name` attribute and the
value is included in the submitted data. `form.reset()` (or a
`w-button type="reset"`) restores fields to their default and clears
validation; an ancestor `<fieldset disabled>` disables the field.

### Typed two-way binding + native validation

Bind a **pointer** to a typed field with `&value` and `w-input` validates it on
blur, natively. The field type owns the string conversion (so its Go type is
never replaced by a raw string) and returns a message **id** — never literal
text — so the message stays in HTML and is translated by `gen_i18n`.

Use a ready-made type from `wings/field`:

```go
import "github.com/luisfurquim/wings/field"

// in the parent component's InitData()
"email": field.NewEmail("email-bad"),            // POINTER; "email-bad" = message id
"age":   field.NewInt(0, 120, "age-nan", "age-bad"), // distinct not-a-number / range ids
```
```html
<w-input type="email" required="true" &value="{{email}}">
  <span slot="errors" id="email-bad">Invalid email address</span>
</w-input>
```

`field` provides `NewText()`, `NewEmail(id)`, `NewInt(min,max,notIntID,rangeID)`,
`NewPattern(re,id)`. Empty is always valid — use `required="true"` for empties.

For custom rules, implement the two small interfaces yourself:

```go
type Email struct{ raw string }
func (e *Email) FromString(s string) { e.raw = strings.TrimSpace(s) }   // ingest
func (e *Email) String() string      { return e.raw }                    // project back
func (e *Email) Validate() string {                                      // "" = valid
    if e.raw == "" || strings.Contains(e.raw, "@") { return "" }
    return "email-bad"   // id of a <span slot="errors"> below the field
}
```

Rules:
- Bind a **`*Type`** (pointer), not a value — pointer-receiver methods must satisfy
  `wings.FieldCodec`/`wings.Validator`, else the bind logs and keeps the value.
- `Validate()` returns a **message id**, resolved against `<span slot="errors" id="…">`
  children (translated in place). Empty string = valid.
- Native validity participates in `form.checkValidity()` — the host is a
  form-associated custom element (ElementInternals mirrors the inner input's
  ValidityState), so a wrapping `<form>` will not submit while any field is
  invalid. The message also shows below the field.
- A static `error="…"` attribute still works for manually-driven errors.

**Host state attributes** (set by the widget for external CSS hooks):
- `[data-focused]`   — present while the `<input>` has focus
- `[data-has-value]` — present while value ≠ `""`
- `[data-empty]`     — present while value = `""`
- `[data-invalid]`   — present when `error` attribute is non-empty

Floating label (pure external CSS, zero Go changes):
```css
w-input[data-focused]::part(label),
w-input[data-has-value]::part(label) {
  transform: translateY(-1.5em) scale(0.8);
  color: var(--wings-primary);
}
```

**CSS hooks**: `::part(root)`, `::part(label-wrap)`, `::part(label)`,
`::part(required-mark)`, `::part(field)`, `::part(prefix)`, `::part(input)`,
`::part(suffix)`, `::part(clear-btn)`, `::part(feedback)`, `::part(helper)`,
`::part(error)`, `::part(count)`. All `--wings-input-*` tokens apply (see `wings-skins`).

Named slots: `label`, `prefix`, `suffix`, `helper`, `error`.

---

## Tabs — `w-tabs` / `w-tabbutton` / `w-tab`

```go
import (
	_ "github.com/luisfurquim/wings/widget/tabs"
	_ "github.com/luisfurquim/wings/widget/tabbutton"
	_ "github.com/luisfurquim/wings/widget/tab"
)
```
`w-tabs` is **controlled**: the visible panel is the host's `active` (a `w-tab`
`tid` or index). Two shapes:

```html
<!-- Shape 1: buttons + panels as adjacent children; a click sets active for you -->
<w-tabs mode="panel">
  <w-tabbutton active>Overview</w-tabbutton>
  <w-tab><h2>Overview</h2>…</w-tab>
  <w-tabbutton>Details</w-tabbutton>
  <w-tab>…</w-tab>
</w-tabs>

<!-- Shape 2: headless/controlled — no w-tabbutton; you drive `active` -->
<w-tabs mode="detached" active="{{current}}">
  <w-tab tid="a">…</w-tab>
  <w-tab tid="b">…</w-tab>
</w-tabs>
```
Modes: `panel` (default), `detached` (chip buttons, transparent panels), `menu`
(left column), `accordion` (each button becomes a native `<summary>`; a `w-tab`
with `active` starts open). `@change` on `w-tabs` fires `args[0]` = selected tid
on user action (not at init, not for programmatic `active`). Panels keep their
DOM across switches. **Don't reinvent tabs with `?cond` + click handlers — use
this.** (For one-button-to-one disclosure, native `<details>` is fine.)

## Dialog — `w-dialog`

```go
import _ "github.com/luisfurquim/wings/widget/dialog"
```
```html
<w-dialog ?show_save_dialog title="Unsaved changes"
          buttons="save,discard,cancel"
          @save="on_save" @discard="on_discard" @cancel="on_cancel">
  <p>You have unsaved changes.</p>   <!-- content via slot -->
</w-dialog>
```
- Visibility is **the parent's** to control: a `?show_…` conditional on the tag
  (toggle the bool in your data to open/close).
- `buttons` is the authoritative, ordered set; valid ids: `save`, `discard`,
  `overwrite`, `cancel`. Each fires the matching event (`@save`, …).
- `title` optional; body goes in the default slot.
- **Don't nest a `<w-dialog>` inside a `<w-tab>` panel (or anything else whose
  own CSS sets a non-`none` `backdrop-filter`, `filter`, `transform`, or
  `contain`)** — its overlay is `position: fixed`, meant to cover the
  viewport, but a fixed-position element is positioned relative to the
  nearest ancestor that establishes a new containing block instead, and
  `w-tab`'s `:host` sets `backdrop-filter: blur(var(--wings-surface-blur,
  0))` unconditionally (its atmosphere-opt-in convention) — `blur(0)` still
  counts as non-`none` per spec, even doing nothing visually. The symptom is
  exactly "dark overlay tint, no visible box": the dialog rendered, just
  scrolled off with the ancestor's own content instead of centering on the
  viewport. If you must open one from inside such a container (this is
  exactly why `w-text`'s own help dialog does it), create/mount the
  `<w-dialog>` at `document.body` instead of in place — but note wings'
  `@save`/`@cancel`/… attribute wiring resolves its handler by walking UP the
  DOM ancestor chain from the dialog, so it can no longer reach a handler
  declared on an ancestor once the dialog sits outside that tree; wire a
  plain DOM listener directly on the internal button instead
  (`dlg.shadowRoot.querySelector("#dlg-cancel")` — present synchronously the
  moment the element is created, since its default `InitData` already shows
  it before `Render` ever runs).

## Rich-text editor — `w-text`

```go
import (
	_ "github.com/luisfurquim/wings/widget/text"
	_ "github.com/luisfurquim/wings/widget/button"   // toolbar is built from these
	_ "github.com/luisfurquim/wings/widget/combobox"
)
```
```html
<w-text label="Biography" profile="basic" placeholder="Tell readers…"
        &value="{{bio}}"></w-text>
```
A pluggable `contenteditable` editor in the `w-input` family. Bind `&value` to a
`FieldCodec` (e.g. `field.NewText()`): it seeds the editor and reads back on
blur, as a **complete EPUB-style document** — the head `<style>` carries the
CSS of the named classes the content uses, so styles round-trip through
storage (loading also accepts a bare fragment; head rules are re-validated
before adoption). Attributes: `label`, `profile` (default `"basic"`),
`placeholder`, `helper`, `error`, `required`, `disabled`. Events: `@input`
(coalesced, args[0] = HTML) and `@change` (on blur after write-back). Form
lifecycle like `w-input` (`form.reset()` restores mount-time content **and**
clears undo; `<fieldset disabled>` → read-only). `::part()` surfaces include
`toolbar` and `editor`; host reflects `[data-focused]`/`[data-empty]`/…

**Don't hand-roll a `contenteditable`** — the security model is the whole point
and it is not something to reproduce ad hoc. Everything entering the document
(typing, paste, drop, and the initial `&value`, which may be stored/untrusted)
is sanitized by allowlist-copy through the browser's own `DOMParser`, so
scripts, `on*` handlers, `<img>`/`<iframe>`, and `javascript:`/`data:`
links can't exist in content. A pasted element's `style=""` is never copied
verbatim either — `epubhtml.FilterCSS` (the lenient counterpart to
`SanitizeCSS`: drops what it doesn't recognize instead of rejecting the
whole declaration list, since a real document's inline style routinely
mixes a couple of supported properties with several this profile was
never meant to carry) reduces it to what the profile supports and the
survivors are registered as a class (`PasteClassName`, a deterministic
hash so identical styles repeated across many pasted elements share one
class). Native formatting (Ctrl+B, mobile callouts) is
intercepted — formatting happens only via the toolbar; a stray `<b>`/`<i>`
from a native path the intercept misses still canonicalizes to
`<strong>`/`<em>`, and content that already carries those tags (pasted,
loaded) round-trips as written. A pasted paragraph break expressed as 2+
consecutive `<br>` (some sources never wrap a paragraph in its own block) is
split into real block boundaries; one `<br>` stays a literal line break. A
mark-shaped wrapper whose actual children are blocks is unwrapped rather than
forced inline — Google Docs wraps its whole clipboard export in a
`<b style="font-weight:normal" id="docs-internal-guid-…">` purely to cancel
`<b>`'s default rendering, holding real `<p>`s despite `<b>` being inline-only
per spec; naively canonicalizing it to `<strong>` would both falsely bold the
paste and dissolve every paragraph inside it as "block inside inline".
Undo/redo is the editor's own, DOM-level and
bounded. Toggling a mark with a collapsed caret arms it as **pending** (Word
behaviour: the next typing comes out marked, or escapes the mark when
toggling off inside one; moving the caret first disarms) — in plugin terms,
`Wrap`/`Unwrap` on a collapsed selection arm instead of no-op, and `InMark`
reads the armed state back so toolbar toggles light up correctly. A
non-collapsed toggle over part of a range (`Wrap`/`Unwrap`/`ApplyClass`/
`RemoveClass`) carves and lifts text nodes, restructuring the DOM without
touching its text content — the core re-locates and reapplies the same
selection by character offset afterward, so the selection a plugin sees
next is the one the user actually acted on, not a leftover fragment or a
stale reference to a node the mutation moved.

The stock `basic` profile gives a bold/italic/underline/code toolbar + a block picker
(`p`, `h1`–`h6`, `blockquote`, `pre`). **Bold and Italic are CSS
(`wt-b`/`wt-i`, `font-weight`/`font-style`), not `<strong>`/`<em>`** — a
toolbar click is a visual toggle for nearly every user, not an assertion of
semantic importance, and `StyleToolbar.CreateStyle` only ever captures CSS
classes: a mark-based bold would silently vanish when a style built from bold
text got reapplied elsewhere. `code` stays a real mark (structural, not a
font weight); `<strong>`/`<em>` remain valid content, just not what the stock
buttons produce — the toggle still *recognizes* them, though: pasted text
that arrived as `<strong>` lights up Bold and can be cleared from the
toolbar, same as `wt-b` text, via `DualMarkToggle("wt-b", "strong")`/
`DualMarkActive("wt-b", "strong")`. **Don't build a custom Bold/Italic on
`Wrap(Strong())` for the CreateStyle reason above, and don't use the plain
`ClassMarkToggle`/`ClassMarkActive` pair either** unless you're sure the
content your profile handles never arrives with the semantic mark already
on it (register the class via `InitPlugin` either way).
Two more stock toolbars compose with `basic` — don't rebuild what they
already do:

- **`wtext.FontToolbar{}`** — font face/size pickers + the four alignment
  toggles. Configurable `Faces`/`Sizes`; defaults are the CSS generic
  families PLUS curated web-safe named stacks (Georgia, Palatino, Times,
  Arial, Verdana, Trebuchet, Courier — always end a custom stack in a
  generic). The face picker previews each option in its own typeface: set
  `wtext.Option.Font` (flows to w-combobox's per-option `font` key; applied
  as a style.fontFamily PROPERTY assignment — never interpolate font strings
  into a style attribute, that's an injection vector). Never inline styles:
  each pick is a utility
  class (`wt-ff-*`, `wt-fs-*`, `wt-al-*`), one per axis, pending-at-caret
  like bold.
- **`wtext.StyleToolbar{}`** — "create style from selection" (✎ prompts for
  a name, captures the covering classes into one named class, swaps them on
  the source range) + a picker applying registered styles ("(none)" clears).
  The `wt-` prefix is reserved; direct formatting stays on the range and
  overrides the style, as in Word. Requires `widget/input` (the prompt
  popover renders a `w-input`).
- **`wtext.CounterToolbar{}`** — a live char/letter/word count docked to the
  toolbar's right edge. Purely passive: one `StatusItem` whose `Args`
  closure recounts `core.DocText()` on every refresh. Chars are runes with
  spaces included but line breaks excluded; letters are `unicode.IsLetter`
  runes; words are whitespace-separated fields carrying at least one letter
  or digit (a standalone comma or dash is not a word). It's also the
  reference for writing your own read-outs (see `StatusItem` below).
- **`wtextepub.Menu{Cfg: wtextepub.Config{Title: …}}`** — "Export › EPUB"
  in the SIDE MENU (a `MenuPlugin`, NOT a toolbar plugin — document-level
  actions never go on the toolbar), from the SEPARATE module
  `github.com/luisfurquim/wings/wtextepub` (wings itself takes no ugarit
  dependency — respect that split; never add ugarit to an app's imports
  through wings). It prompts for the DOCUMENT name (Save-As, seeded with
  the book title): the typed string is the TOC entry and page `<title>`
  as is, its `Filename`-sanitized form is only the download name, and
  `Config.Title` stays the book's `dc:title`. It packages
  `core.Content()` as an EPUB 3 book and
  downloads it; book language is `wings.Locale` at click time;
  `Config.Title` is required. The nav/TOC
  is the first spine itemref (book opens on the TOC) and coverless books
  carry no cover landmark (both via ugarit v0.0.2). Its item
  label ids (`wtext-epub`, `wtext-epub-name`, `wtext-epub-help`) have NO built-in default —
  the app must provide the `<span slot="labels" id="…">` pair or the item
  shows the raw id (the `wtext-export` group id does have a default).

Classes apply with the **Word split**: character declarations become
`<span class>` on the exact range, paragraph declarations mark the touched
blocks. A `span` may exist only while carrying a registered class.

To customize, register your own `wtext.Profile` (toolbar/edition/clipboard
plugins) and select it by name:

```go
wtext.RegisterProfile("post", wtext.Profile{
    Toolbar: []wtext.ToolbarPlugin{
        wtext.BasicToolbar{}, wtext.FontToolbar{}, wtext.StyleToolbar{},
    },
    LinkPolicy: func(u *url.URL) error { /* restrict link hosts */ return nil },
})
```
```html
<w-text profile="post" &value="{{body}}"></w-text>
```
Build a toolbar by composing the exported helpers (`ToggleMark`, `MarkActive`,
`BlockCurrent`, `BlockPick`, `SwapClass`, `ClassPick`, `ClassToggle`,
`ClassCurrent`, `ClassActive`, `ClassMarkToggle`, `ClassMarkActive`,
`DualMarkToggle`, `DualMarkActive`) over the `wtext.EditorCore` API — which
also exposes the class registry (`Classes`, `ClassCSS`, `ClassesAt`,
`ClassSpanned`, `DefineClass`) and the whole-document reads: `DocText`
(plain text, with a newline at every block boundary so words in adjacent
paragraphs don't fuse) and `Content` (the persisted EPUB-style
serialization — what exporter plugins consume). A toolbar item
needing a typed value (style name, URL)
declares an `InputItem`; a passive read-out (a counter, a save indicator)
declares a `StatusItem` — `Format` is a message id resolving to a `fmt`
template, `Args` computes its values, and the widget re-renders it in the
same pass that refreshes toggle state (translators reorder with `%[2]d`);
a plugin needing setup at attach time (defining its
utility classes, say) implements `wtext.InitPlugin`. Document-level
actions (export, import…) do NOT go on the toolbar: implement
`MenuPlugin` (`Profile.Menu`) declaring `MenuAction{Group, ID, Label,
Help, Do}` — or `MenuInput` when the action needs a typed value first
(same prompt popover as the toolbar's `InputItem`, with a `Value` hook
seeding it, opened selected: Enter keeps, typing replaces) — the widget
renders one accordion section per `Group`
(`w-tabs mode="accordion"` = native `<details>`) in a column beside the
editor, only when items exist (a hamburger button collapses the column);
use the standard group ids `wtext-export`
/ `wtext-import` where they fit, and remember the app must
`import _ ".../widget/tabs"`, `".../widget/tabbutton"`, `".../widget/tab"`
for the menu to render. The portable half of
`wtext` (the `EditorCore` interface, the `Fragment` builder, the sealed
`Mark` constructors, the stock toolbars) unit-tests against a fake core with
the native toolchain — no browser needed. When you DO touch the js side, mind
`sec-wasm-go`: a `js.Value.Call`/`Get` on a number/string primitive panics,
and a panic is whole-app death.

**Help dialog contract.** `ToggleItem`/`ButtonItem`/`SelectItem`/`InputItem`/
`StatusItem` each carry a `Help` field (a message id, alongside `Label`) — this is the
entire mechanism by which a `ToolbarPlugin` "hands over its own help" to
compose one dialog covering every control: declare `Help` on the items
`Items()` already returns, nothing more. It's additive — old plugin code
with named-field struct literals still compiles; an item with `Help == ""`
(the zero value) is just left out of the dialog. The widget adds a
trailing "?" button itself (only when at least one active plugin's item
sets `Help`), walks `profile.Toolbar`'s `Items()`, and opens a `w-dialog`
(so a profile using any stock or custom toolbar with `Help` needs
`import _ "github.com/luisfurquim/wings/widget/dialog"` too) listing every
documented label + explanation in toolbar order — mounted at
`document.body` rather than in the toolbar itself (see the `w-dialog`
nesting gotcha above; `w-text` can sit inside a `w-tab` panel, which
triggers it). `BasicToolbar`,
`FontToolbar`, `StyleToolbar` document every item they declare — follow
that as the reference when adding `Help` to a custom toolbar. Localize the
`-help` ids the same way as `Label` ids (`<span slot="labels" id="…">`),
plus `wtext-help`/`wtext-help-title` for the button/dialog title.

## Combobox — `w-combobox`

```go
import _ "github.com/luisfurquim/wings/widget/combobox"
```
```html
<w-combobox options='["Alpha","Beta","Gamma"]' mode="single"
            value="Beta" placeholder="Type to filter…"
            @change="on_change" @notinlist="on_notinlist"></w-combobox>
```
- `options`: JSON array of strings or `{"label","value"}` objects.
- `mode`: `multi` (default, tags) or `single` (native-select-like). In `single`,
  `value` is authoritative and re-syncs silently (no `@change`) if the parent
  reverts it — good for controlled rollback after a confirm dialog.
- `@change` args = `[]any` of selected items; `@notinlist` = the typed string.

## Record navbar — `w-navbar`

```go
import _ "github.com/luisfurquim/wings/widget/navbar"
```
```html
<w-navbar nav_input="{{cur_record}}" total_count="{{record_count}}"
          @first="goFirst" @prev="goPrev" @next="goNext" @last="goLast"
          @prevmany="goPrevPage" @nextmany="goNextPage" @change="onSeek"></w-navbar>
```
Stateless: position/total are owned by the parent via the bound fields; the
position input is two-way bound.

## Skin picker — `skin-switcher`

```go
import _ "github.com/luisfurquim/wings/widget/skinswitcher"
```
```html
<skin-switcher></skin-switcher>
```
Self-contained: lists registered skins (import the ones you want — see
`wings-skins`), applies/deactivates them, and stays in sync via `OnSkinChange`.
No attributes needed.

## Also available

`w-test` / `w-test-report` (in-app test harness, `widget/test` + `widget/testreport`).
Full attribute tables and theming details: README "Built-in Widgets". Widgets
read `--wings-*` tokens, so they follow your skin automatically (`wings-skins`).

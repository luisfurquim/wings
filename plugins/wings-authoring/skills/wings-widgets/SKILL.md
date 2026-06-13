---
name: wings-widgets
description: Use WINGS's built-in widgets instead of hand-rolling them — tabbed containers (w-tabs/w-tabbutton/w-tab), dialogs (w-dialog), multi-select combobox (w-combobox), record navbar (w-navbar), and the skin picker (skin-switcher). Use when an app needs tabs, a modal/dialog, a filtering select, record navigation, or a theme picker.
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

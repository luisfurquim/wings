//go:build js && wasm

package wi18n

import (
	"strconv"
	"strings"
	"syscall/js"

	"github.com/luisfurquim/wprana"
)

// TranslateCheck mode constants.
const (
	TranslateCheckNone      byte = iota // no enforcement (default)
	TranslateCheckHighlight             // mark unrevised nodes with inline style
	TranslateCheckHold                  // block rendering entirely
)

var (
	translateCheckMode byte
	unrevisedIdx       map[int]bool // set of catalog indices not yet human-revised
)

// SetTranslateCheck installs the translation-review enforcement mode.
// Must be called before wi18n's init goroutine finishes (i.e., from an init()
// in a package that imports wi18n, or from main() before any await on InitWG).
func SetTranslateCheck(mode byte) {
	translateCheckMode = mode
}

// applyTranslateCheck is called by loadAndInstall after the bundle is ready.
// Returns true only in Hold mode when unrevised entries exist — the caller
// must NOT call wprana.InitWG.Done() in that case.
func applyTranslateCheck(bundle *localeBundle) (blocked bool) {
	switch translateCheckMode {
	case TranslateCheckNone:
		return false
	case TranslateCheckHighlight:
		updateHighlightSet(bundle)
		installNodeAnnotator()
		return false
	case TranslateCheckHold:
		updateHighlightSet(bundle)
		if len(unrevisedIdx) == 0 {
			return false
		}
		injectIcarusScreen(bundle)
		return true
	}
	return false
}

// updateHighlightSet rebuilds unrevisedIdx from the bundle's revised flags.
func updateHighlightSet(bundle *localeBundle) {
	m := make(map[int]bool, len(bundle.revised))
	for i, rev := range bundle.revised {
		if !rev {
			m[i] = true
		}
	}
	unrevisedIdx = m
}

// installNodeAnnotator sets wprana.NodeAnnotator to mark unrevised DOM nodes
// with an inline style (yellow outline). Any subsequent edit by the translator
// in wlate that flips Revised=true will remove the entry from unrevisedIdx,
// so a retranslation cycle clears the marker automatically.
func installNodeAnnotator() {
	wprana.NodeAnnotator = func(rawIndex string, node js.Value) {
		idx, err := strconv.Atoi(rawIndex)
		if err != nil {
			return
		}
		style := node.Get("style")
		if style.IsUndefined() || style.IsNull() {
			return
		}
		if unrevisedIdx[idx] {
			style.Call("setProperty", "outline", "2px solid #f5a623", "important")
			style.Call("setProperty", "background-color", "rgba(245,166,35,0.12)", "important")
		} else {
			style.Call("removeProperty", "outline")
			style.Call("removeProperty", "background-color")
		}
	}
}

// injectIcarusScreen injects a full-viewport WINGS "Flight of Icarus" overlay
// into document.body. The page behind it is blocked; no dismiss button exists.
func injectIcarusScreen(bundle *localeBundle) {
	var sb strings.Builder
	sb.WriteString(`<div id="wings-icarus-screen" style="position:fixed;inset:0;z-index:2147483647;background:#0a0a1a;color:#e8e8ff;font-family:monospace;display:flex;flex-direction:column;align-items:center;justify-content:center;overflow:auto;padding:2rem;box-sizing:border-box">`)

	// SVG wings logo (minimalist feather silhouette)
	sb.WriteString(`<svg width="96" height="96" viewBox="0 0 96 96" fill="none" xmlns="http://www.w3.org/2000/svg" style="margin-bottom:1.5rem">`)
	sb.WriteString(`<path d="M48 80 C20 60 4 36 12 16 C28 32 40 48 48 80Z" fill="#4a90d9" opacity="0.85"/>`)
	sb.WriteString(`<path d="M48 80 C76 60 92 36 84 16 C68 32 56 48 48 80Z" fill="#357abd" opacity="0.85"/>`)
	sb.WriteString(`<circle cx="48" cy="14" r="6" fill="#f5a623"/>`)
	sb.WriteString(`</svg>`)

	sb.WriteString(`<div style="font-size:1.6rem;font-weight:bold;letter-spacing:.1em;color:#f5a623;margin-bottom:.5rem">WINGS</div>`)
	sb.WriteString(`<div style="font-size:.9rem;color:#8888aa;margin-bottom:2rem;letter-spacing:.05em">WEB IN GO SPHERE</div>`)
	sb.WriteString(`<div style="font-size:1.1rem;color:#ff6b6b;margin-bottom:.4rem;font-weight:bold">&#x2708; Flight of Icarus</div>`)
	sb.WriteString(`<div style="font-size:.85rem;color:#aaaacc;margin-bottom:2rem;text-align:center;max-width:36rem">`)
	sb.WriteString(`This build contains unrevised machine translations.<br>`)
	sb.WriteString(`All entries must be reviewed before deployment.`)
	sb.WriteString(`</div>`)

	// Table of unrevised entries
	sb.WriteString(`<div style="background:#11112a;border:1px solid #334;border-radius:6px;padding:1rem;max-width:56rem;width:100%;max-height:40vh;overflow-y:auto">`)
	sb.WriteString(`<table style="border-collapse:collapse;width:100%;font-size:.8rem">`)
	sb.WriteString(`<thead><tr>`)
	sb.WriteString(`<th style="text-align:left;padding:.3rem .6rem;color:#8888cc;border-bottom:1px solid #334">Index</th>`)
	sb.WriteString(`<th style="text-align:left;padding:.3rem .6rem;color:#8888cc;border-bottom:1px solid #334">Content</th>`)
	sb.WriteString(`</tr></thead><tbody>`)

	for idx := range unrevisedIdx {
		content := ""
		if idx >= 0 && idx < len(bundle.text) {
			content = bundle.text[idx]
		}
		sb.WriteString(`<tr>`)
		sb.WriteString(`<td style="padding:.25rem .6rem;color:#f5a623;vertical-align:top;white-space:nowrap">`)
		sb.WriteString(strconv.Itoa(idx))
		sb.WriteString(`</td><td style="padding:.25rem .6rem;color:#ccccee;word-break:break-word">`)
		sb.WriteString(htmlEscape(content))
		sb.WriteString(`</td></tr>`)
	}

	sb.WriteString(`</tbody></table></div>`)
	sb.WriteString(`</div>`)

	doc := wprana.JSGlobal().Get("document")
	body := doc.Get("body")
	if body.IsUndefined() || body.IsNull() {
		// Body not yet present — wait for DOMContentLoaded.
		var fn js.Func
		fn = js.FuncOf(func(this js.Value, args []js.Value) any {
			fn.Release()
			doc.Get("body").Set("insertAdjacentHTML", "beforeend")
			doc.Get("body").Call("insertAdjacentHTML", "beforeend", sb.String())
			return nil
		})
		doc.Call("addEventListener", "DOMContentLoaded", fn)
		return
	}
	body.Call("insertAdjacentHTML", "beforeend", sb.String())
}

// htmlEscape escapes the five HTML special characters.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&#34;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

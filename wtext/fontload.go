//go:build js && wasm

package wtext

import (
	"fmt"
	"strings"
	"syscall/js"
)

// maxFontVariants caps how many @font-face files one font pulls — each
// (style × weight) splits into per-script SUBSET files, all needed.
const maxFontVariants = maxFontFaceRules

func init() {
	// The portable FontToolbar reaches the loader through this hook: the
	// plugin declaration stays natively testable while the mechanism
	// lives here.
	loadWebFont = AddFont
}

// AddFont loads a font from an allowlisted store URL — the same door the
// face picker's URL drop uses. Asynchronous: fetches the store CSS,
// extracts the @font-face variants (both steps bounded, origins
// re-verified), fetches each file and installs it via the browser's
// FontFace API, keeping the bytes in the registry for EPUB embedding.
// done receives the installed family name, or the error (also logged).
func AddFont(rawURL string, done func(family string, err error)) {
	finish := func(family string, err error) {
		if err != nil {
			G.Logf(1, "wtext: AddFont(%q): %v\n", rawURL, err)
		}
		if done != nil {
			done(family, err)
		}
	}
	go func() {
		p, err := parseFontURL(rawURL)
		if err != nil {
			finish("", err)
			return
		}
		css, err := fetchString(p.cssURL, maxFontCSSLen)
		if err != nil {
			finish("", err)
			return
		}
		srcs, err := parseFontFaceCSS(css, p.store)
		if err != nil {
			finish("", err)
			return
		}
		if len(srcs) > maxFontVariants {
			srcs = srcs[:maxFontVariants]
		}
		family := strings.ReplaceAll(p.family, `"`, "")
		fonts := js.Global().Get("document").Get("fonts")
		loaded := srcs[:0]
		for _, s := range srcs {
			data, err := fetchBytes(s.URL, MaxFontFileLen)
			if err != nil {
				G.Logf(1, "wtext: AddFont(%q): variant %s/%s: %v\n", rawURL, s.Style, s.Weight, err)
				continue
			}
			u8 := js.Global().Get("Uint8Array").New(len(data))
			js.CopyBytesToJS(u8, data)
			desc := map[string]any{"style": s.Style, "weight": s.Weight}
			if s.Range != "" {
				desc["unicodeRange"] = s.Range
			}
			face := js.Global().Get("FontFace").New(family, u8, desc)
			fonts.Call("add", face)
			s.Data = data
			loaded = append(loaded, s)
		}
		if len(loaded) == 0 {
			finish("", fmt.Errorf("%w: no variant loaded", ErrFontCSS))
			return
		}
		err = registerWebFont(WebFont{
			ID:     fontSlug(family),
			Label:  family,
			Family: `"` + family + `"`,
			// The store's CANONICAL css URL, not the raw string dropped:
			// a specimen page, an old css v1 link and a css2 link for the
			// same family all normalize to one reference, so a document
			// (or a library file) never persists two entries for one font.
			StoreURL: p.cssURL,
			Sources:  loaded,
		})
		if err != nil {
			finish("", err)
			return
		}
		finish(family, nil)
	}()
}

// fetchString GETs a text resource, bounded.
func fetchString(url string, limit int) (string, error) {
	data, err := fetchBytes(url, limit)
	return string(data), err
}

// fetchBytes GETs a binary resource, bounded — the fetch promise chain
// adapted to a channel, like wi18n's fetchText.
func fetchBytes(url string, limit int) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)
	fail := func(err error) { ch <- result{err: err} }

	var thenFn, bufThen, catchFn js.Func
	release := func() {
		thenFn.Release()
		bufThen.Release()
		catchFn.Release()
	}
	bufThen = js.FuncOf(func(_ js.Value, args []js.Value) any {
		buf := args[0]
		n := buf.Get("byteLength").Int()
		if n > limit {
			fail(fmt.Errorf("wtext: %s: %d bytes over the %d limit", url, n, limit))
			return nil
		}
		data := make([]byte, n)
		js.CopyBytesToGo(data, js.Global().Get("Uint8Array").New(buf))
		ch <- result{data: data}
		return nil
	})
	thenFn = js.FuncOf(func(_ js.Value, args []js.Value) any {
		resp := args[0]
		if !resp.Get("ok").Bool() {
			fail(fmt.Errorf("wtext: %s: HTTP %d", url, resp.Get("status").Int()))
			return nil
		}
		resp.Call("arrayBuffer").Call("then", bufThen).Call("catch", catchFn)
		return nil
	})
	catchFn = js.FuncOf(func(_ js.Value, args []js.Value) any {
		msg := "fetch failed"
		if len(args) > 0 && args[0].Truthy() {
			msg = args[0].Call("toString").String()
		}
		fail(fmt.Errorf("wtext: %s: %s", url, msg))
		return nil
	})
	js.Global().Call("fetch", url).Call("then", thenFn).Call("catch", catchFn)
	res := <-ch
	release()
	return res.data, res.err
}

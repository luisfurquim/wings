//go:build js && wasm

// Package wi18n provides internationalization support for wprana.
//
// The package is designed to be imported for its side effects only:
//
//	import _ "github.com/luisfurquim/wprana/wi18n"
//
// On init(), wi18n:
//  1. Detects the browser language via navigator.languages / navigator.language.
//  2. Registers itself on wprana.InitWG so that wprana.Main() waits for the
//     asynchronous load before defining the custom elements.
//  3. Spawns a goroutine that fetches <BasePath><lang>.json from the same
//     host/path used to load the current page, decodes it as a JSON array of
//     [Entry], and overrides wprana.Printer with a lookup function that
//     interprets the TextNode content as a decimal index into the catalog.
//
// BasePath defaults to "i18n/". Applications that ship their own catalog
// alongside a user-editable one (e.g. the wlate editor, which both consumes
// and produces i18n files) can call [SetBasePath] from an init() that runs
// after wi18n's init — typically by importing wi18n from a package that is
// itself imported later in the main module's dependency graph.
//
// If the package is NOT imported, wprana.Printer remains the default ByPass
// and every TextNode keeps the raw index produced by gen_i18n.
package wi18n

import (
	"strconv"
	"strings"
	"sync"
	"syscall/js"

	"github.com/luisfurquim/wprana"
)

// ── Package state ───────────────────────────────────────────────────────────

var (
	// lang is the BCP 47 tag selected for this session.
	lang string

	// table holds the translated strings indexed by the decimal number
	// that gen_i18n wrote into each TextNode.
	table []string

	basePathMu sync.RWMutex
	basePath   = "i18n/"
)

// printerToken is the one-time token consumed at package-level init so that
// SetPrinter calls from this package are authorized against wprana's token guard.
var printerToken = wprana.TakePrinterToken()

// Lang returns the language tag selected at init time.
func Lang() string {
	return lang
}

// SetBasePath overrides the default "i18n/" prefix used when fetching the
// catalog. Must be called before the async fetch goroutine starts reading
// the path — in practice, from an init() in a package imported by wi18n's
// consumer, since goroutines do not preempt synchronous init code.
func SetBasePath(p string) {
	if p != "" && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	basePathMu.Lock()
	basePath = p
	basePathMu.Unlock()
}

// BasePath returns the current catalog prefix.
func BasePath() string {
	basePathMu.RLock()
	defer basePathMu.RUnlock()
	return basePath
}

// ── init ────────────────────────────────────────────────────────────────────

func init() {
	lang = detectLang()
	setHTMLLang(lang)
	wprana.Locale = lang

	// Install locale-aware formatting immediately — it has no async deps.
	// Printer / SynPrinter are installed later by the catalog loader.
	wprana.FmtPrinter = fmtPrinter

	// Register ourselves as an async initializer so that wprana.Main() waits
	// until the JSON has been fetched and parsed before defining the modules.
	wprana.InitWG.Add(1)
	go loadAndInstall()
}

// loadAndInstall runs in a goroutine. It fetches the JSON for the current
// language (with fallbacks), decodes it, and replaces wprana.Printer with a
// lookup function. Calls wprana.InitWG.Done() on every exit path.
func loadAndInstall() {
	defer wprana.InitWG.Done()

	bundle, err := loadBundle(lang)
	if err != nil {
		wprana.G.Logf(1, "wi18n: %v; Printer stays as ByPass\n", err)
		return
	}

	// Seed the locale cache so a later SetLang(lang) is a cache hit.
	bundleMu.Lock()
	bundles[lang] = bundle
	bundleMu.Unlock()

	table = bundle.text

	// If the picked tag is not the one we originally detected, update the
	// <html lang="..."> attribute so that the DOM reflects what actually loaded.
	if bundle.picked != lang {
		lang = bundle.picked
		setHTMLLang(lang)
		wprana.Locale = lang
	}

	wprana.SetPrinter(lookup, printerToken)
	wprana.G.Logf(2, "wi18n: loaded %d entries for lang=%s\n", len(table), lang)

	if bundle.flex != nil {
		setFlexCatalog(bundle.flex, bundle.tag)
		wprana.SynPrinter = synPrinter
		wprana.G.Logf(2, "wi18n: loaded %d flex rules for lang=%s\n", len(bundle.flex), lang)
	}
}

// fallbackChain builds the ordered list of language tags to try, starting
// from the fully-qualified tag, then the base language (before the first '-'),
// then "en-US" as a last resort. Duplicates are removed.
func fallbackChain(tag string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add(tag)
	if i := strings.IndexByte(tag, '-'); i > 0 {
		add(tag[:i])
	}
	add("en-US")
	return out
}

// lookup is installed as wprana.Printer once the catalog is available. The
// input is the raw TextNode content that gen_i18n produced: a decimal index
// into the catalog. Any string that does not parse as a valid in-range
// index is returned unchanged, matching the ByPass behaviour for dynamic
// text.
func lookup(in string) string {
	idx, err := strconv.Atoi(in)
	if err != nil {
		return in
	}
	if idx < 0 || idx >= len(table) {
		return in
	}
	if table[idx] == "" {
		// Empty translation: fall back to the raw index so missing
		// translations are visually obvious instead of rendering blank.
		return in
	}
	return table[idx]
}

// ── Language detection ──────────────────────────────────────────────────────

// detectLang picks the language tag for this session. It honors an explicit
// <html lang="..."> attribute first (developer override), then falls back to
// navigator.languages / navigator.language. Returns "en-US" if nothing is
// available.
func detectLang() string {
	doc := wprana.JSGlobal().Get("document")
	if !doc.IsUndefined() && !doc.IsNull() {
		html := doc.Get("documentElement")
		if !html.IsUndefined() && !html.IsNull() {
			if tag := html.Call("getAttribute", "lang"); !tag.IsUndefined() && !tag.IsNull() && tag.String() != "" {
				return tag.String()
			}
		}
	}

	nav := wprana.JSGlobal().Get("navigator")

	langs := nav.Get("languages")
	if !langs.IsUndefined() && !langs.IsNull() {
		if n := langs.Get("length").Int(); n > 0 {
			return langs.Index(0).String()
		}
	}

	l := nav.Get("language")
	if !l.IsUndefined() && !l.IsNull() && l.String() != "" {
		return l.String()
	}

	return "en-US"
}

// setHTMLLang sets the lang attribute on <html>.
func setHTMLLang(l string) {
	doc := wprana.JSGlobal().Get("document")
	html := doc.Get("documentElement")
	if html.IsUndefined() || html.IsNull() {
		return
	}
	html.Call("setAttribute", "lang", l)
}

// ── HTTP fetch via JS fetch() ───────────────────────────────────────────────

// fetchText performs a GET on url (relative to the current page) and returns
// the response body as a string. Uses the browser's fetch() API and a Go
// channel to bridge the async JS promise back to the goroutine.
func fetchText(url string) (string, error) {
	type result struct {
		body string
		err  error
	}
	ch := make(chan result, 1)

	var thenFn, catchFn, textThen, textCatch js.Func

	textThen = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- result{body: args[0].String()}
		return nil
	})
	textCatch = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- result{err: jsError(args)}
		return nil
	})

	thenFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		resp := args[0]
		if !resp.Get("ok").Bool() {
			ch <- result{err: &fetchErr{status: resp.Get("status").Int(), url: url}}
			return nil
		}
		resp.Call("text").Call("then", textThen).Call("catch", textCatch)
		return nil
	})
	catchFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- result{err: jsError(args)}
		return nil
	})

	wprana.JSGlobal().Call("fetch", url).Call("then", thenFn).Call("catch", catchFn)

	r := <-ch
	thenFn.Release()
	catchFn.Release()
	textThen.Release()
	textCatch.Release()
	return r.body, r.err
}

// fetchErr carries a non-2xx HTTP status from fetch().
type fetchErr struct {
	status int
	url    string
}

func (e *fetchErr) Error() string {
	return "fetch " + e.url + ": status " + strconv.Itoa(e.status)
}

// jsErrVal wraps a JS error value in a Go error.
type jsErrVal struct{ msg string }

func (e *jsErrVal) Error() string { return e.msg }

func jsError(args []js.Value) error {
	if len(args) == 0 {
		return &jsErrVal{msg: "unknown js error"}
	}
	v := args[0]
	if v.IsUndefined() || v.IsNull() {
		return &jsErrVal{msg: "unknown js error"}
	}
	msg := v.Get("message")
	if msg.IsUndefined() || msg.IsNull() {
		return &jsErrVal{msg: v.String()}
	}
	return &jsErrVal{msg: msg.String()}
}

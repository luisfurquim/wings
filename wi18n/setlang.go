//go:build js && wasm

package wi18n

import (
	"encoding/json"
	"errors"
	"sync"

	"golang.org/x/text/language"

	"github.com/luisfurquim/wprana"
)

// localeBundle holds everything one locale needs at runtime: the text
// catalog, the inflections catalog (may be nil if the app uses no flex
// blocks), the parsed BCP 47 tag, and the picked tag (which may differ from
// the requested one after fallback resolution).
type localeBundle struct {
	text   []string
	flex   []FlexEntry
	tag    language.Tag
	picked string
}

var (
	bundleMu sync.RWMutex
	bundles  = map[string]*localeBundle{}
)

// SetLang switches the active locale at runtime. It loads the requested
// catalog (with fallback to base language and "en-US"), updates wprana.Locale
// and the <html lang> attribute, then asks every live custom element to
// re-translate its DOM in place. Inputs and component state survive the
// switch — only the bindings driven by Printer / SynPrinter are refreshed.
//
// SetLang ALWAYS does its work in a goroutine and returns immediately. This
// is mandatory: catalog loading uses fetch() and a synchronous wait would
// block the JS event loop, preventing the very fetch.then callback from
// firing — the page would deadlock. Doing the work in a goroutine yields
// control to JS so the fetch can resolve.
//
// The optional `done` callback is invoked from the goroutine once the
// switch completes (or fails). Pass nil if you don't need notification.
// The error is non-nil when no catalog could be loaded for the requested
// tag or any of its fallbacks; on error, the previous locale is untouched.
//
// Repeated calls to the same locale are cheap: catalogs are cached keyed
// by the originally requested tag.
func SetLang(tag string, done func(error)) {
	go func() {
		err := setLangSync(tag)
		if done != nil {
			done(err)
		}
	}()
}

func setLangSync(tag string) error {
	if tag == "" {
		return errors.New("wi18n.SetLang: empty tag")
	}

	bundleMu.RLock()
	cached, ok := bundles[tag]
	bundleMu.RUnlock()

	if !ok {
		loaded, err := loadBundle(tag)
		if err != nil {
			return err
		}
		bundleMu.Lock()
		bundles[tag] = loaded
		bundleMu.Unlock()
		cached = loaded
	}

	table = cached.text
	if cached.flex != nil {
		setFlexCatalog(cached.flex, cached.tag)
		wprana.SynPrinter = synPrinter
	}

	lang = cached.picked
	wprana.Locale = cached.picked
	setHTMLLang(cached.picked)
	wprana.Printer = lookup

	wprana.RetranslateAll()
	return nil
}

// loadBundle fetches the text + (optional) inflections catalog for one
// locale, walking the standard fallback chain. The returned bundle's
// `picked` field carries the actually loaded tag, which the caller exposes
// as the new wprana.Locale.
func loadBundle(requested string) (*localeBundle, error) {
	base := BasePath()

	var (
		body   string
		picked string
		err    error
	)
	for _, cand := range fallbackChain(requested) {
		url := base + cand + ".json"
		body, err = fetchText(url)
		if err == nil {
			picked = cand
			break
		}
		wprana.G.Logf(3, "wi18n: %s not available (%v), trying next\n", url, err)
	}
	if picked == "" {
		return nil, errors.New("wi18n: no catalog available for " + requested)
	}

	var entries []Entry
	if err := json.Unmarshal([]byte(body), &entries); err != nil {
		return nil, err
	}
	text := make([]string, len(entries))
	for i, e := range entries {
		text[i] = e.Content
	}

	tag, err := language.Parse(picked)
	if err != nil {
		tag = language.English
	}

	bundle := &localeBundle{
		text:   text,
		tag:    tag,
		picked: picked,
	}

	// Inflections are optional — apps without flex blocks publish no file.
	if flexBody, err := fetchText(base + picked + ".inflections.json"); err == nil {
		var flexEntries []FlexEntry
		if err := json.Unmarshal([]byte(flexBody), &flexEntries); err == nil {
			bundle.flex = flexEntries
		} else {
			wprana.G.Logf(1, "wi18n: failed to parse inflections for %s: %v\n", picked, err)
		}
	}

	return bundle, nil
}

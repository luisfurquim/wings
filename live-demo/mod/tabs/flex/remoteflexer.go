//go:build js && wasm

package flex

import (
	"encoding/json"
	"fmt"
	"sync"
	"syscall/js"

	"github.com/luisfurquim/wings"
)

// RemoteFlexer is a demonstration wings.CustomFlex engine that inflects a word
// by calling a backend — here, static JSON fixtures published at
// /flex-mock/<locale>/<lemma>.json (so the demo works on GitHub Pages, with no
// live server). Each fixture is a tiny dictionary entry:
//
//	{"singular":"maçã","plural":"maçãs"}
//
// It exists to ILLUSTRATE the contract for runtime inflection backed by a
// network source — not to be a real linguistic service.
//
// The interesting part is the async dance. wings.CustomFlex.Flex MUST be
// synchronous (blocking on fetch would deadlock the JS event loop, like
// SetLang). So on a cache miss Flex returns the word verbatim as a placeholder
// and kicks off the fetch in a goroutine; when the form arrives it fills the
// cache and calls notify, which re-runs the reactive sync — Flex is invoked
// again and this time returns the cached, inflected form. notify is just an
// obj.This.Set() the host component supplies (see flextab.go).
type RemoteFlexer struct {
	mu      sync.Mutex
	cache   map[string]string // "<locale>|<form>|<word>" → inflected form
	pending map[string]bool   // "<locale>|<word>" with a fetch in flight
	notify  func()            // re-sync trigger, installed by the host component
}

// NewRemoteFlexer builds an engine with empty caches.
func NewRemoteFlexer() *RemoteFlexer {
	return &RemoteFlexer{
		cache:   map[string]string{},
		pending: map[string]bool{},
	}
}

// SetNotify installs the re-sync callback the engine fires after an async fetch
// resolves. Typically obj.This.Set(<a key the block reads>, …) so the flex
// block re-renders.
func (f *RemoteFlexer) SetNotify(fn func()) {
	f.mu.Lock()
	f.notify = fn
	f.mu.Unlock()
}

// Priority elects this engine over the implicit catalog engine (priority 0)
// whenever it appears in a block as a *var. Implementing wings.Prioritized is
// optional; without it a CustomFlex counts as priority 0.
func (f *RemoteFlexer) Priority() uint { return 10 }

// String emits nothing: the engine is a pure participant, contributing no text
// at its own position in the block.
func (f *RemoteFlexer) String() string { return "" }

// Flex returns the inflected form of word. It reads the count from the block's
// %-selector to choose singular/plural and wings.Locale (at call time, never
// cached) to choose the language. On a cache miss it returns word verbatim and
// fetches in the background.
func (f *RemoteFlexer) Flex(word string, selectors ...wings.FlexSelector) (string, error) {
	n := 1
	for _, s := range selectors {
		if s.Sigil == '%' { // the count axis
			n = toInt(s.Value)
		}
	}
	// Demo pluralisation rule: 1 → singular, everything else → plural. A real
	// engine would consult the full CLDR plural categories per locale.
	form := "singular"
	if n != 1 {
		form = "plural"
	}

	locale := wings.Locale
	key := locale + "|" + form + "|" + word

	f.mu.Lock()
	cached, hit := f.cache[key]
	inFlight := f.pending[locale+"|"+word]
	f.mu.Unlock()

	if hit {
		return cached, nil
	}
	if !inFlight {
		go f.fetch(locale, word)
	}
	return word, nil // placeholder until the fetch resolves and re-syncs
}

// fetch retrieves both forms for (locale, word) from the backend, caches them,
// and triggers a re-sync. Runs in its own goroutine so it never blocks Flex.
func (f *RemoteFlexer) fetch(locale, word string) {
	f.mu.Lock()
	f.pending[locale+"|"+word] = true
	f.mu.Unlock()

	url := "/flex-mock/" + locale + "/" + encodeURIComponent(word) + ".json"
	body, err := fetchText(url)

	f.mu.Lock()
	delete(f.pending, locale+"|"+word)
	if err != nil {
		f.mu.Unlock()
		wings.G.Logf(1, "RemoteFlexer: fetch %s failed: %v (word stays verbatim)\n", url, err)
		return
	}
	var forms struct {
		Singular string `json:"singular"`
		Plural   string `json:"plural"`
	}
	if e := json.Unmarshal([]byte(body), &forms); e != nil {
		f.mu.Unlock()
		wings.G.Logf(1, "RemoteFlexer: bad JSON from %s: %v\n", url, e)
		return
	}
	f.cache[locale+"|singular|"+word] = forms.Singular
	f.cache[locale+"|plural|"+word] = forms.Plural
	notify := f.notify
	f.mu.Unlock()

	if notify != nil {
		notify() // re-run the sync: Flex is called again, now a cache hit
	}
}

// toInt coerces a selector value to an int, defaulting to 1 (singular).
func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 1
}

// encodeURIComponent percent-encodes a path segment (the lemma may be
// non-ASCII, e.g. "maçã"), via the browser's own function.
func encodeURIComponent(s string) string {
	return js.Global().Call("encodeURIComponent", s).String()
}

// fetchText GETs url and returns the response body. It bridges the JS fetch
// promise to a channel, so the calling goroutine blocks here while the JS event
// loop stays free — which is why fetch() must run off the main sync pass.
func fetchText(url string) (string, error) {
	type result struct {
		body string
		err  error
	}
	ch := make(chan result, 1)
	var onResp, onErr, onText, onTextErr js.Func

	release := func() {
		onResp.Release()
		onErr.Release()
		onText.Release()
		onTextErr.Release()
	}
	onText = js.FuncOf(func(_ js.Value, a []js.Value) any {
		ch <- result{body: a[0].String()}
		release()
		return nil
	})
	onTextErr = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		ch <- result{err: fmt.Errorf("reading body of %s", url)}
		release()
		return nil
	})
	onResp = js.FuncOf(func(_ js.Value, a []js.Value) any {
		resp := a[0]
		if !resp.Get("ok").Bool() {
			ch <- result{err: fmt.Errorf("%s: HTTP %d", url, resp.Get("status").Int())}
			release()
			return nil
		}
		resp.Call("text").Call("then", onText).Call("catch", onTextErr)
		return nil
	})
	onErr = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		ch <- result{err: fmt.Errorf("fetch %s failed", url)}
		release()
		return nil
	})

	js.Global().Call("fetch", url).Call("then", onResp).Call("catch", onErr)
	r := <-ch
	return r.body, r.err
}

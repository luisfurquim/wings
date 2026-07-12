//go:build js && wasm

package wi18n

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/text/language"

	"github.com/luisfurquim/wings"
)

// signaturesRequired reports whether catalog signature verification is enabled,
// i.e. a public key was configured via SetCatalogPublicKey. When it returns
// false, callers MUST NOT fetch .sig sidecars at all (no signing in use). When
// true, every loaded catalog must carry a valid .sig or be rejected.
//
// It lives HERE, not in the portable verify.go, on purpose: its only caller
// is this js-only file, and in verify.go the native golangci-lint pass
// (which cannot see js callers) flagged it unused and failed CI.
func signaturesRequired() bool {
	catalogPubKeyMu.RLock()
	defer catalogPubKeyMu.RUnlock()
	return catalogPubKey != nil
}

// localeBundle holds everything one locale needs at runtime: the text
// catalog, the inflections catalog (may be nil if the app uses no flex
// blocks), the format config (may be nil when no <lang>.fmt.json was
// published), the parsed BCP 47 tag, and the picked tag (which may differ
// from the requested one after fallback resolution).
type localeBundle struct {
	text    []string
	revised []bool // true = human-reviewed; parallel to text
	flex    []FlexEntry
	fmtCfg  *fmtConfig
	tag     language.Tag
	picked  string
}

var (
	bundleMu sync.RWMutex
	bundles  = map[string]*localeBundle{}
)

// SetLang switches the active locale at runtime. It loads the requested
// catalog (with fallback to base language and "en-US"), updates wings.Locale
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
		wings.SynPrinter = synPrinter
	}

	lang = cached.picked
	wings.Locale = cached.picked
	setHTMLLang(cached.picked)
	wings.SetPrinter(lookup, printerToken) // idempotent after first install

	if translateCheckMode == TranslateCheckHighlight {
		updateHighlightSet(cached)
	}

	wings.RetranslateAll()
	return nil
}

// loadBundle fetches the text + (optional) inflections catalog for one
// locale, walking the standard fallback chain. The returned bundle's
// `picked` field carries the actually loaded tag, which the caller exposes
// as the new wings.Locale.
func loadBundle(requested string) (*localeBundle, error) {
	// Validate BCP-47 format before using the tag in URL construction.
	// Prevents path-traversal via SetLang("../../etc/passwd").
	if _, err := language.Parse(requested); err != nil {
		return nil, fmt.Errorf("wi18n: invalid locale tag %q: %w", requested, err)
	}

	base := BasePath()

	var (
		body   string
		picked string
		err    error
	)
	for _, cand := range fallbackChain(requested) {
		url := base + cand + ".json"
		body, err = fetchText(url)
		if err != nil {
			wings.G.Logf(3, "wi18n: %s not available (%v), trying next\n", url, err)
			continue
		}
		if sigErr := requireSig(url, body); sigErr != nil {
			return nil, sigErr
		}
		picked = cand
		break
	}
	if picked == "" {
		return nil, errors.New("wi18n: no catalog available for " + requested)
	}

	var entries []Entry
	if err := json.Unmarshal([]byte(body), &entries); err != nil {
		return nil, err
	}
	text := make([]string, len(entries))
	revised := make([]bool, len(entries))
	for i, e := range entries {
		text[i] = e.Content
		revised[i] = e.Revised
	}

	tag, err := language.Parse(picked)
	if err != nil {
		tag = language.English
	}

	bundle := &localeBundle{
		text:    text,
		revised: revised,
		tag:     tag,
		picked:  picked,
	}

	// Inflections are optional — apps without flex blocks publish no file (a
	// 404 here is fine). But once the file IS present and a public key is
	// configured, it must carry a valid .sig like the main catalog: silently
	// dropping a tampered inflections file would let an attacker degrade flex.
	flexURL := base + picked + ".inflections.json"
	if flexBody, err := fetchText(flexURL); err == nil {
		if sigErr := requireSig(flexURL, flexBody); sigErr != nil {
			return nil, sigErr
		}
		var flexEntries []FlexEntry
		if err := json.Unmarshal([]byte(flexBody), &flexEntries); err == nil {
			bundle.flex = flexEntries
		} else {
			wings.G.Logf(1, "wi18n: failed to parse inflections for %s: %v\n", picked, err)
		}
	}

	// Format config is optional — apps without measure types publish no file.
	// Same policy as inflections: absent is fine, present must verify.
	fmtURL := base + picked + ".fmt.json"
	if fmtBody, err := fetchText(fmtURL); err == nil {
		if sigErr := requireSig(fmtURL, fmtBody); sigErr != nil {
			return nil, sigErr
		}
		if cfg := parseFmtConfig(fmtBody); cfg != nil {
			bundle.fmtCfg = cfg
		}
	}

	return bundle, nil
}

// requireSig enforces the signature policy on one successfully fetched catalog
// file: when a public key is configured (SetCatalogPublicKey), the file's .sig
// sidecar must exist and verify — a missing sidecar or a bad signature is
// treated as tampering and reported as *CatalogSignatureError. Without a
// configured key it is a no-op and no .sig is fetched at all.
func requireSig(url, body string) error {
	if !signaturesRequired() {
		return nil
	}
	sigBody, err := fetchText(url + ".sig")
	if err != nil {
		return &CatalogSignatureError{URL: url, Err: fmt.Errorf("signature required but .sig unavailable: %w", err)}
	}
	if err := verifyCatalog(body, sigBody); err != nil {
		return &CatalogSignatureError{URL: url, Err: err}
	}
	return nil
}

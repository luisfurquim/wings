//go:build js && wasm

package wprana

import (
	"sync"

	"github.com/luisfurquim/wprana/expr"
)

// parseCache memoises expr.ParseText so the SetLang re-bind walker does
// not re-parse the same translated string for every instance that uses it
// (e.g. dozens of custom elements all carrying the same i18n index).
//
// Keys are the parsed input string itself. expr.ParseText is deterministic
// in its input, so cached entries are valid forever — the only "change"
// possible is a new translated string after SetLang, which produces a
// different key. Entries are never invalidated.
var (
	parseCacheMu sync.RWMutex
	parseCache   = map[string][]TextSegment{}
)

// cachedParseText returns the parsed segs for text, populating the cache
// on first request. The returned slice is shared with the cache and other
// callers; callers MUST treat it as read-only.
func cachedParseText(text string) ([]TextSegment, error) {
	parseCacheMu.RLock()
	segs, ok := parseCache[text]
	parseCacheMu.RUnlock()
	if ok {
		return segs, nil
	}
	segs, err := expr.ParseText(text)
	if err != nil {
		return nil, err
	}
	parseCacheMu.Lock()
	parseCache[text] = segs
	parseCacheMu.Unlock()
	return segs, nil
}

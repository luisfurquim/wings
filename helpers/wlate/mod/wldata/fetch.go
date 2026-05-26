//go:build js && wasm

package wldata

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/luisfurquim/wprana/wi18n"
)

// ── Fetch helpers ──────────────────────────────────────────────────────────

type fetchResult struct {
	body string
	err  error
}

// FetchText GETs a URL and returns its body as text. It blocks the calling
// goroutine on the fetch promise, so it must run off the main loop.
//
// Bypasses the HTTP cache entirely (cache: "no-store"): wlate edits these
// files in place, so any stale disk-cache hit would show pre-save content
// even though /save wrote the new bytes to disk.
func FetchText(url string) (string, error) {
	ch := make(chan fetchResult, 1)

	var thenFn, catchFn, textThen, textCatch js.Func

	textThen = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- fetchResult{body: args[0].String()}
		return nil
	})
	textCatch = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- fetchResult{err: fmt.Errorf("fetch text error: %s", jsErrMsg(args))}
		return nil
	})
	thenFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		resp := args[0]
		if !resp.Get("ok").Bool() {
			ch <- fetchResult{err: fmt.Errorf("fetch %s: status %d", url, resp.Get("status").Int())}
			return nil
		}
		resp.Call("text").Call("then", textThen).Call("catch", textCatch)
		return nil
	})
	catchFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- fetchResult{err: fmt.Errorf("fetch %s: %s", url, jsErrMsg(args))}
		return nil
	})

	opts := js.Global().Get("Object").New()
	opts.Set("cache", "no-store")
	js.Global().Call("fetch", url, opts).Call("then", thenFn).Call("catch", catchFn)
	r := <-ch
	thenFn.Release()
	catchFn.Release()
	textThen.Release()
	textCatch.Release()
	return r.body, r.err
}

// PostJSON POSTs raw bytes as application/json and blocks until the response
// settles. It must run off the main loop.
func PostJSON(url string, data []byte) error {
	ch := make(chan error, 1)

	headers := js.Global().Get("Object").New()
	headers.Set("Content-Type", "application/json")

	opts := js.Global().Get("Object").New()
	opts.Set("method", "POST")
	opts.Set("headers", headers)

	jsBody := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(jsBody, data)
	opts.Set("body", jsBody)

	var thenFn, catchFn js.Func
	thenFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		resp := args[0]
		if !resp.Get("ok").Bool() {
			ch <- fmt.Errorf("save failed: status %d", resp.Get("status").Int())
			return nil
		}
		ch <- nil
		return nil
	})
	catchFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- fmt.Errorf("save error: %s", jsErrMsg(args))
		return nil
	})

	js.Global().Call("fetch", url, opts).Call("then", thenFn).Call("catch", catchFn)
	err := <-ch
	thenFn.Release()
	catchFn.Release()
	return err
}

func jsErrMsg(args []js.Value) string {
	if len(args) == 0 {
		return "unknown error"
	}
	v := args[0]
	if v.IsUndefined() || v.IsNull() {
		return "unknown error"
	}
	msg := v.Get("message")
	if msg.IsUndefined() || msg.IsNull() {
		return v.String()
	}
	return msg.String()
}

// ── Catalog loaders ────────────────────────────────────────────────────────

// LoadText fetches a <lang>.json + <lang>.meta.json pair and merges them into
// the in-memory TextRecord shape. Either half may be absent — legacy combined
// files ship both fields in the data half, so those still decode correctly.
func LoadText(lang string) []TextRecord {
	var records []TextRecord

	body, err := FetchText("i18n/" + lang + ".json")
	if err == nil {
		json.Unmarshal([]byte(body), &records)
	}
	body, err = FetchText("i18n/" + lang + ".meta.json")
	if err == nil {
		var metas []wi18n.EntryMeta
		if json.Unmarshal([]byte(body), &metas) == nil {
			for i := range records {
				if i < len(metas) {
					records[i].EntryMeta = metas[i]
				}
			}
		}
	}
	return records
}

// LoadFlex fetches a <lang>.inflections.json + .inflections.meta.json pair and
// merges them into the in-memory InflectionRecord shape.
func LoadFlex(lang string) []InflectionRecord {
	var records []InflectionRecord

	body, err := FetchText("i18n/" + lang + ".inflections.json")
	if err == nil {
		json.Unmarshal([]byte(body), &records)
	}
	body, err = FetchText("i18n/" + lang + ".inflections.meta.json")
	if err == nil {
		var metas []wi18n.FlexEntryMeta
		if json.Unmarshal([]byte(body), &metas) == nil {
			for i := range records {
				if i < len(metas) {
					records[i].FlexEntryMeta = metas[i]
				}
			}
		}
	}
	return records
}

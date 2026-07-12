//go:build js && wasm

package wtextepub

import "syscall/js"

// download hands data to the browser as a named file download: a Blob
// URL on a transient <a download> — the only file-save path a web page
// gets without special permissions. The object URL is revoked right
// after the click; the browser keeps the blob alive for the in-flight
// download.
func download(name string, data []byte) {
	u8 := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(u8, data)
	blob := js.Global().Get("Blob").New(
		[]any{u8},
		map[string]any{"type": "application/epub+zip"},
	)
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	doc := js.Global().Get("document")
	a := doc.Call("createElement", "a")
	a.Set("href", url)
	a.Set("download", name)
	doc.Get("body").Call("appendChild", a)
	a.Call("click")
	a.Call("remove")
	js.Global().Get("URL").Call("revokeObjectURL", url)
}

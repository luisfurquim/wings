//go:build js && wasm

package flex

import (
	_ "embed"
	"encoding/json"

	"github.com/luisfurquim/wings"
)

//go:embed platformhelp.json
var platformHelpJSON []byte

// PlatformHelper is a SYNCHRONOUS wings.CustomFlex engine — no network, no
// build-time catalog. It selects a platform-specific string from an embedded
// table shaped like a Fluent selector
//
//	{ $platform -> [linux] … [macos] … *[other] … }
//
// expressed as plain JSON: message id → platform → form. Nothing binds WINGS to
// this shape; a custom engine stores its data however it likes. Mirroring Fluent
// just makes the mapping obvious to anyone arriving from there.
//
// It demonstrates ARBITRARY contextual selection: Flex branches on the
// $platform selector it receives — the same role Fluent gives an arbitrary
// selector — while the surrounding sentence stays plain, catalog-translated i18n.
type PlatformHelper struct {
	table map[string]map[string]string
}

// NewPlatformHelper parses the embedded selection table.
func NewPlatformHelper() *PlatformHelper {
	h := &PlatformHelper{}
	_ = json.Unmarshal(platformHelpJSON, &h.table)
	return h
}

// Priority elects this engine over the implicit catalog engine (priority 0).
func (h *PlatformHelper) Priority() uint { return 10 }

// String contributes no text of its own — the engine only resolves ~words.
func (h *PlatformHelper) String() string { return "" }

// Flex returns the form for word (a message id, e.g. "copy") under the current
// $platform selector, falling back to the table's "*" entry, then to the word
// verbatim — a missing form stays visible rather than blank.
func (h *PlatformHelper) Flex(word string, selectors ...wings.FlexSelector) (string, error) {
	platform := "*"
	for _, s := range selectors {
		if s.Sigil == '$' && s.Name == "platform" {
			if p, ok := s.Value.(string); ok && p != "" {
				platform = p
			}
		}
	}
	variants, ok := h.table[word]
	if !ok {
		return word, nil
	}
	if form, ok := variants[platform]; ok {
		return form, nil
	}
	if form, ok := variants["*"]; ok {
		return form, nil
	}
	return word, nil
}

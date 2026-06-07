package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// genI18nFile mirrors the gen_i18n.json shape that cmd/gen_i18n reads (see
// cmd/gen_i18n/config.go). Only the translator block is written; gen_i18n fills
// the rest from its own defaults.
type genI18nFile struct {
	Translator genI18nTranslator `json:"translator"`
}

type genI18nTranslator struct {
	Backend string `json:"backend"`
	URL     string `json:"url"`
	Model   string `json:"model,omitempty"`
	Key     string `json:"key,omitempty"`
	Timeout string `json:"timeout,omitempty"`
}

// ensureTranslatorConfig makes the WINGS_TR_* settings available to gen_i18n by
// writing a gen_i18n.json in the app root — but only when the file does not
// already exist. A user-authored gen_i18n.json always wins; in that case the
// WINGS_TR_* values are ignored (and we say so). With no backend configured and
// no file present there is nothing to write, so -auto-translate simply no-ops in
// gen_i18n (it warns there).
func ensureTranslatorConfig(cfg *devConfig) error {
	path := filepath.Join(cfg.AppRoot, "gen_i18n.json")
	if _, err := os.Stat(path); err == nil {
		if cfg.TRBackend != "" {
			devLogf("gen_i18n.json already exists — WINGS_TR_* ignored (file wins)")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if cfg.TRBackend == "" {
		return nil
	}
	f := genI18nFile{Translator: genI18nTranslator{
		Backend: cfg.TRBackend,
		URL:     cfg.TRURL,
		Model:   cfg.TRModel,
		Key:     cfg.TRKey,
		Timeout: cfg.TRTimeout,
	}}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return err
	}
	devLogf("wrote gen_i18n.json from WINGS_TR_* (backend=%s)", cfg.TRBackend)
	return nil
}

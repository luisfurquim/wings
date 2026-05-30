package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/luisfurquim/wings/cmd/gen_i18n/translator"
	"github.com/luisfurquim/wings/wi18n"
)

// autoTranslate is set by the -auto-translate flag. When true, gen_i18n
// calls the configured LLM/MT backend to pre-fill catalog entries that the
// dictionary pass could not fill. Entries filled by the translator are
// flagged Revised=false so the human translator sees them in wlate.
var autoTranslate bool

// autoTranslator is the active translation backend. Nil when -auto-translate
// is not set or when no backend is configured in gen_i18n.json.
var autoTranslator translator.Translator

// trBatchSize and trBatchChars control how entries are grouped per
// Translate call. Overridden from gen_i18n.json; defaults match config.go.
var trBatchSize = 20
var trBatchChars = 4000

// initTranslator reads gen_i18n.json from rootDir and instantiates the
// configured backend, storing it in autoTranslator. Returns an error only for
// hard configuration failures; an unconfigured or unknown backend is a warning.
func initTranslator(rootDir string) error {
	cfg, err := loadGenI18nConfig(rootDir)
	if err != nil {
		return fmt.Errorf("loading gen_i18n.json: %w", err)
	}
	tc := cfg.Translator
	switch tc.Backend {
	case "openai":
		if tc.URL == "" {
			return fmt.Errorf("translator.backend=openai requires translator.url in gen_i18n.json")
		}
		timeout, err := time.ParseDuration(tc.Timeout)
		if err != nil {
			timeout = 60 * time.Second
		}
		autoTranslator = translator.NewOpenAI(tc.URL, tc.Model, tc.Key, tc.SystemPrompt, timeout)
	case "libretranslate":
		if tc.URL == "" {
			return fmt.Errorf("translator.backend=libretranslate requires translator.url in gen_i18n.json")
		}
		timeout, err := time.ParseDuration(tc.Timeout)
		if err != nil {
			timeout = 60 * time.Second
		}
		autoTranslator = translator.NewLibreTranslate(tc.URL, tc.Key, timeout)
	case "":
		G.Logf(2, "warn: -auto-translate set but translator.backend not configured in gen_i18n.json; -auto-translate has no effect")
	default:
		G.Logf(2, "warn: unknown translator.backend %q in gen_i18n.json; -auto-translate has no effect", tc.Backend)
	}
	if autoTranslator != nil {
		trBatchSize = tc.BatchSize
		trBatchChars = tc.BatchChars
	}
	return nil
}

// applyTextTranslations fills empty Content fields in entries by sending them
// to the configured translator in batches. defEntries provides the source
// language strings (parallel-indexed). No-op when autoTranslator is nil.
func applyTextTranslations(entries []wi18n.Entry, defEntries []wi18n.Entry, defLang, lang string) {
	if autoTranslator == nil {
		return
	}
	var batch []translator.Entry
	labelToIdx := map[string]int{} // numeric label → index into entries

	for i := range entries {
		if entries[i].Content != "" {
			continue
		}
		if i >= len(defEntries) || defEntries[i].Content == "" {
			continue
		}
		label := strconv.Itoa(i)
		batch = append(batch, translator.Entry{
			Label:   label,
			Context: entries[i].Context,
			Cells:   map[string]string{translator.SimpleTextKey: defEntries[i].Content},
		})
		labelToIdx[label] = i
	}
	if len(batch) == 0 {
		return
	}

	tag := autoTranslator.SourceTag()
	ctx := context.Background()
	for _, b := range translator.Batch(batch, trBatchSize, trBatchChars) {
		resp, err := autoTranslator.Translate(ctx, defLang, lang, b)
		if err != nil {
			G.Logf(2, "warn: translator error (%s→%s): %v", defLang, lang, err)
			return
		}
		for _, e := range resp.Entries {
			text := e.Cells[translator.SimpleTextKey]
			if text == "" {
				continue
			}
			if idx, ok := labelToIdx[e.Label]; ok {
				entries[idx].Content = text
				entries[idx].Source = tag
			}
		}
		for key, reason := range resp.Failed {
			G.Logf(2, "warn: translator skip (%s→%s) %s: %s", defLang, lang, key, reason)
		}
	}
}

// applyFlexTranslations fills empty cells in out[] by translating the
// corresponding source cells from defOut[] (the deflang inflection catalog).
// Only cells that are empty in out and non-empty in defOut are submitted.
// No-op when autoTranslator is nil.
func applyFlexTranslations(out []wi18n.FlexEntry, defOut []wi18n.FlexEntry, defLang, lang string) {
	if autoTranslator == nil {
		return
	}
	var batch []translator.Entry
	labelToOutIdx := map[string]int{} // numeric label → index into out

	for i := range out {
		if i >= len(defOut) {
			continue
		}
		cells := map[string]string{}
		for k, v := range out[i].Cells {
			if v == "" && defOut[i].Cells[k] != "" {
				cells[k] = defOut[i].Cells[k]
			}
		}
		if len(cells) == 0 {
			continue
		}
		label := strconv.Itoa(i)
		batch = append(batch, translator.Entry{
			Label:   label,
			Context: out[i].Label + " | " + out[i].Context,
			Cells:   cells,
		})
		labelToOutIdx[label] = i
	}
	if len(batch) == 0 {
		return
	}

	tag := autoTranslator.SourceTag()
	ctx := context.Background()
	for _, b := range translator.Batch(batch, trBatchSize, trBatchChars) {
		resp, err := autoTranslator.Translate(ctx, defLang, lang, b)
		if err != nil {
			G.Logf(2, "warn: translator error (%s→%s) flex: %v", defLang, lang, err)
			return
		}
		for _, e := range resp.Entries {
			outIdx, ok := labelToOutIdx[e.Label]
			if !ok {
				continue
			}
			if out[outIdx].Sources == nil {
				out[outIdx].Sources = map[string]string{}
			}
			for k, v := range e.Cells {
				if v != "" {
					out[outIdx].Cells[k] = v
					out[outIdx].Sources[k] = tag
				}
			}
		}
		for key, reason := range resp.Failed {
			G.Logf(2, "warn: translator skip (%s→%s) flex %s: %s", defLang, lang, key, reason)
		}
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type translatorConfig struct {
	Backend      string `json:"backend"`       // "openai" | "libretranslate" | ""
	URL          string `json:"url"`
	Model        string `json:"model"`         // OpenAI backend only
	Key          string `json:"key"`           // optional API key
	BatchSize    int    `json:"batch_size"`    // max entries per API call
	BatchChars   int    `json:"batch_chars"`   // max source chars per API call
	Timeout      string `json:"timeout"`       // e.g. "60s"
	SystemPrompt string `json:"system_prompt"` // overrides the built-in LLM system prompt
}

type genI18nConfig struct {
	Translator translatorConfig `json:"translator"`
}

// loadGenI18nConfig reads gen_i18n.json from rootDir. Missing file is not an
// error — returns defaults so the tool works without any config file.
func loadGenI18nConfig(rootDir string) (genI18nConfig, error) {
	cfg := genI18nConfig{
		Translator: translatorConfig{
			BatchSize:  20,
			BatchChars: 4000,
			Timeout:    "60s",
		},
	}
	data, err := os.ReadFile(filepath.Join(rootDir, "gen_i18n.json"))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("gen_i18n.json: %w", err)
	}
	if cfg.Translator.BatchSize <= 0 {
		cfg.Translator.BatchSize = 20
	}
	if cfg.Translator.BatchChars <= 0 {
		cfg.Translator.BatchChars = 4000
	}
	if cfg.Translator.Timeout == "" {
		cfg.Translator.Timeout = "60s"
	}
	return cfg, nil
}

package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LibreTranslate implements Translator against a LibreTranslate instance.
// It uses array batch mode (POST /translate with "q": [...]); LibreTranslate
// v1.3+ is required.
//
// Template tokens ({{...}}) and numeric placeholders (%word) are replaced with
// numbered markers (WPTK_0, WPTK_1, …) before each cell is sent and restored
// afterward, because LibreTranslate has no system-prompt mechanism to enforce
// verbatim preservation.
type LibreTranslate struct {
	url        string
	apiKey     string
	httpClient *http.Client
}

// NewLibreTranslate constructs a LibreTranslate backend. apiKey is optional —
// pass "" for self-hosted instances that do not require authentication.
func NewLibreTranslate(url, apiKey string, timeout time.Duration) *LibreTranslate {
	return &LibreTranslate{
		url:        strings.TrimRight(url, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (l *LibreTranslate) SourceTag() string { return "libretranslate" }

// Translate flattens all entry cells, applies placeholder substitution, sends
// the whole batch in one POST /translate call, then restores tokens and runs
// ValidateCell. Cell-level failures go into Response.Failed.
func (l *LibreTranslate) Translate(ctx context.Context, srcLang, dstLang string, entries []Entry) (Response, error) {
	type cellRef struct {
		entryIdx int
		cellKey  string
		src      string
		tokens   []string
	}

	var refs []cellRef
	var texts []string

	for i, e := range entries {
		for k, text := range e.Cells {
			substituted, tokens := substituteTokens(text)
			refs = append(refs, cellRef{i, k, text, tokens})
			texts = append(texts, substituted)
		}
	}

	out := Response{Entries: make([]Entry, len(entries))}
	for i, e := range entries {
		out.Entries[i] = Entry{Label: e.Label, Context: e.Context, Cells: map[string]string{}}
	}
	if len(texts) == 0 {
		return out, nil
	}

	translated, err := l.translateBatch(ctx, langToISO(srcLang), langToISO(dstLang), texts)
	if err != nil {
		return Response{}, err
	}

	for j, ref := range refs {
		if j >= len(translated) {
			out = recordCellFailed(out, entries[ref.entryIdx].Label, ref.cellKey, "missing from response")
			continue
		}
		restored := restoreTokens(translated[j], ref.tokens)
		if err := ValidateCell(ref.cellKey, ref.src, restored); err != nil {
			out = recordCellFailed(out, entries[ref.entryIdx].Label, ref.cellKey, err.Error())
			continue
		}
		out.Entries[ref.entryIdx].Cells[ref.cellKey] = restored
	}
	if len(out.Failed) == 0 {
		out.Failed = nil
	}
	return out, nil
}

func (l *LibreTranslate) translateBatch(ctx context.Context, src, dst string, texts []string) ([]string, error) {
	reqBody, err := json.Marshal(ltRequest{
		Q:      texts,
		Source: src,
		Target: dst,
		APIKey: l.apiKey,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.url+"/translate", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("libretranslate: HTTP %d: %s", resp.StatusCode, body)
	}

	var ltResp ltResponse
	if err := json.Unmarshal(body, &ltResp); err != nil {
		return nil, fmt.Errorf("libretranslate: parse response: %w", err)
	}
	if ltResp.Error != "" {
		return nil, fmt.Errorf("libretranslate: %s", ltResp.Error)
	}
	return ltResp.TranslatedText, nil
}

// ── wire types ────────────────────────────────────────────────────────────────

type ltRequest struct {
	Q      []string `json:"q"`
	Source string   `json:"source"`
	Target string   `json:"target"`
	APIKey string   `json:"api_key,omitempty"`
}

type ltResponse struct {
	TranslatedText []string `json:"translatedText"`
	Error          string   `json:"error,omitempty"`
}

// ── placeholder substitution ──────────────────────────────────────────────────

// reWptkTokens matches the token types that must survive translation verbatim:
// {{...}} template references and %word numeric placeholders.
var reWptkTokens = regexp.MustCompile(`\{\{[^}]+\}\}|%[A-Za-z_]\w*`)

// reWptkMarker matches the WPTK_N markers inserted by substituteTokens.
var reWptkMarker = regexp.MustCompile(`WPTK_(\d+)`)

// substituteTokens replaces each {{...}} and %word token with a numbered
// marker WPTK_N. Returns the substituted string and the original token list
// (index N → original token) for use with restoreTokens.
func substituteTokens(text string) (substituted string, tokens []string) {
	substituted = reWptkTokens.ReplaceAllStringFunc(text, func(tok string) string {
		n := len(tokens)
		tokens = append(tokens, tok)
		return fmt.Sprintf("WPTK_%d", n)
	})
	return substituted, tokens
}

// restoreTokens replaces WPTK_N markers in text with the corresponding entry
// from tokens. Markers with out-of-range indices are left as-is.
func restoreTokens(text string, tokens []string) string {
	return reWptkMarker.ReplaceAllStringFunc(text, func(m string) string {
		n, err := strconv.Atoi(strings.TrimPrefix(m, "WPTK_"))
		if err != nil || n >= len(tokens) {
			return m
		}
		return tokens[n]
	})
}

// ── language code helpers ─────────────────────────────────────────────────────

// langToISO strips a BCP 47 tag to its ISO 639-1 base (e.g. "pt-BR" → "pt").
// LibreTranslate uses 2-letter ISO codes.
func langToISO(lang string) string {
	if i := strings.IndexByte(lang, '-'); i >= 0 {
		return lang[:i]
	}
	return lang
}

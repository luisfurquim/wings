package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAI implements Translator against any OpenAI-compatible chat completions
// endpoint (Ollama, LM Studio, OpenAI, etc.).
type OpenAI struct {
	url        string
	model      string
	key        string
	sysPrompt  string
	httpClient *http.Client
}

// NewOpenAI constructs an OpenAI backend. sysPrompt overrides
// DefaultSystemPrompt when non-empty.
func NewOpenAI(url, model, key, sysPrompt string, timeout time.Duration) *OpenAI {
	if sysPrompt == "" {
		sysPrompt = DefaultSystemPrompt
	}
	return &OpenAI{
		url:        strings.TrimRight(url, "/"),
		model:      model,
		key:        key,
		sysPrompt:  sysPrompt,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// SourceTag returns the provenance tag stamped on entries this backend translates.
func (o *OpenAI) SourceTag() string { return "llm:" + o.model }

// Translate sends entries as a single chat completions request and returns the
// translated result. ValidateCell is run on every translated cell; failures are
// recorded in Response.Failed rather than returned as errors.
func (o *OpenAI) Translate(ctx context.Context, srcLang, dstLang string, entries []Entry) (Response, error) {
	userMsg, err := json.Marshal(oaiBatchReq{
		SrcLang: srcLang,
		DstLang: dstLang,
		Entries: entriesToBatch(entries),
	})
	if err != nil {
		return Response{}, err
	}

	reqBody, err := json.Marshal(oaiChatReq{
		Model: o.model,
		Messages: []oaiMsg{
			{Role: "system", Content: o.sysPrompt},
			{Role: "user", Content: string(userMsg)},
		},
		Temperature: 0.1,
	})
	if err != nil {
		return Response{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.url+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.key)
	}

	httpResp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, err
	}
	if httpResp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("openai: HTTP %d: %s", httpResp.StatusCode, body)
	}

	var chatResp oaiChatResp
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return Response{}, fmt.Errorf("openai: parse response: %w", err)
	}
	if chatResp.Error != nil {
		return Response{}, fmt.Errorf("openai: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return Response{}, fmt.Errorf("openai: empty choices")
	}

	content := stripMarkdownFences(chatResp.Choices[0].Message.Content)

	var batchResp oaiBatchResp
	if err := json.Unmarshal([]byte(content), &batchResp); err != nil {
		return Response{}, fmt.Errorf("openai: parse model output: %w", err)
	}

	return assembleoaiResponse(entries, batchResp.Entries), nil
}

// ── wire types ───────────────────────────────────────────────────────────────

type oaiChatReq struct {
	Model       string   `json:"model"`
	Messages    []oaiMsg `json:"messages"`
	Temperature float32  `json:"temperature"`
}

type oaiMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiChatResp struct {
	Choices []struct {
		Message oaiMsg `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ── batch payload types (user message / model reply) ─────────────────────────

type oaiBatchReq struct {
	SrcLang string          `json:"source_lang"`
	DstLang string          `json:"target_lang"`
	Entries []oaiBatchEntry `json:"entries"`
}

type oaiBatchResp struct {
	Entries []oaiBatchEntry `json:"entries"`
}

type oaiBatchEntry struct {
	Label   string            `json:"label"`
	Context string            `json:"context,omitempty"`
	Cells   map[string]string `json:"cells"`
}

// ── helpers ──────────────────────────────────────────────────────────────────

func entriesToBatch(entries []Entry) []oaiBatchEntry {
	out := make([]oaiBatchEntry, len(entries))
	for i, e := range entries {
		out[i] = oaiBatchEntry{Label: e.Label, Context: e.Context, Cells: e.Cells}
	}
	return out
}

// assembleoaiResponse matches translated entries to source entries by label and
// validates each cell. Mismatched or invalid cells go into Response.Failed.
func assembleoaiResponse(src []Entry, translated []oaiBatchEntry) Response {
	byLabel := make(map[string]map[string]string, len(translated))
	for _, be := range translated {
		byLabel[be.Label] = be.Cells
	}

	out := Response{
		Entries: make([]Entry, len(src)),
	}
	for i, e := range src {
		out.Entries[i] = Entry{Label: e.Label, Context: e.Context, Cells: map[string]string{}}
		dstCells, ok := byLabel[e.Label]
		if !ok {
			out = recordFailed(out, e, "not returned by model")
			continue
		}
		for cellKey, srcText := range e.Cells {
			dstText, ok := dstCells[cellKey]
			if !ok {
				out = recordCellFailed(out, e.Label, cellKey, "cell not returned by model")
				continue
			}
			if err := ValidateCell(cellKey, srcText, dstText); err != nil {
				out = recordCellFailed(out, e.Label, cellKey, err.Error())
				continue
			}
			out.Entries[i].Cells[cellKey] = dstText
		}
	}
	if len(out.Failed) == 0 {
		out.Failed = nil
	}
	return out
}

func recordFailed(out Response, e Entry, reason string) Response {
	for cellKey := range e.Cells {
		out = recordCellFailed(out, e.Label, cellKey, reason)
	}
	return out
}

func recordCellFailed(out Response, label, cellKey, reason string) Response {
	if out.Failed == nil {
		out.Failed = map[string]string{}
	}
	out.Failed[label+":"+cellKey] = reason
	return out
}

// stripMarkdownFences removes ```json ... ``` or ``` ... ``` wrappers that
// some models add around their JSON output despite being told not to.
func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if idx := strings.Index(s, "\n"); idx >= 0 {
		s = s[idx+1:]
	}
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

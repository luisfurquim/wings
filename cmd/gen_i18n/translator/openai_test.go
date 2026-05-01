package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockOAIServer creates a test server that returns the given batchResp entries
// as a chat completions response.
func mockOAIServer(t *testing.T, entries []oaiBatchEntry) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Decode request to verify it is well-formed.
		var req oaiChatReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(req.Messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(req.Messages))
		}

		content, _ := json.Marshal(oaiBatchResp{Entries: entries})
		resp, _ := json.Marshal(oaiChatResp{
			Choices: []struct {
				Message oaiMsg `json:"message"`
			}{{Message: oaiMsg{Role: "assistant", Content: string(content)}}},
		})
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	}))
}

func newTestOpenAI(url string) *OpenAI {
	return NewOpenAI(url, "test-model", "", "", 5*time.Second)
}

func TestOpenAI_SimpleText_Success(t *testing.T) {
	ts := mockOAIServer(t, []oaiBatchEntry{
		{Label: "greeting", Cells: map[string]string{"": "Hola, {{nome}}"}},
	})
	defer ts.Close()

	entries := []Entry{
		{Label: "greeting", Cells: map[string]string{"": "Olá, {{nome}}"}},
	}
	resp, err := newTestOpenAI(ts.URL).Translate(context.Background(), "pt-BR", "es-AR", entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Failed) != 0 {
		t.Errorf("unexpected failures: %v", resp.Failed)
	}
	if got := resp.Entries[0].Cells[""]; got != "Hola, {{nome}}" {
		t.Errorf("got %q, want %q", got, "Hola, {{nome}}")
	}
}

func TestOpenAI_FlexEntry_Success(t *testing.T) {
	ts := mockOAIServer(t, []oaiBatchEntry{
		{Label: "student_count", Cells: map[string]string{
			"m.one":   "el %n alumno está aprobado",
			"m.other": "los %n alumnos están aprobados",
		}},
	})
	defer ts.Close()

	entries := []Entry{
		{Label: "student_count", Cells: map[string]string{
			"m.one":   "o %n aluno está aprovado",
			"m.other": "os %n alunos estão aprovados",
		}},
	}
	resp, err := newTestOpenAI(ts.URL).Translate(context.Background(), "pt-BR", "es-AR", entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Failed) != 0 {
		t.Errorf("unexpected failures: %v", resp.Failed)
	}
	if got := resp.Entries[0].Cells["m.one"]; got != "el %n alumno está aprobado" {
		t.Errorf("m.one: got %q", got)
	}
}

func TestOpenAI_MarkdownFences_Stripped(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner, _ := json.Marshal(oaiBatchResp{Entries: []oaiBatchEntry{
			{Label: "title", Cells: map[string]string{"": "Hola"}},
		}})
		// Wrap in markdown fences as some models do.
		fenced := "```json\n" + string(inner) + "\n```"
		resp, _ := json.Marshal(oaiChatResp{
			Choices: []struct {
				Message oaiMsg `json:"message"`
			}{{Message: oaiMsg{Role: "assistant", Content: fenced}}},
		})
		w.Write(resp)
	}))
	defer ts.Close()

	entries := []Entry{{Label: "title", Cells: map[string]string{"": "Olá"}}}
	resp, err := newTestOpenAI(ts.URL).Translate(context.Background(), "pt-BR", "es-AR", entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resp.Entries[0].Cells[""]; got != "Hola" {
		t.Errorf("got %q, want %q", got, "Hola")
	}
}

func TestOpenAI_PlaceholderDropped_RecordedInFailed(t *testing.T) {
	ts := mockOAIServer(t, []oaiBatchEntry{
		// Translator dropped %n from the cell.
		{Label: "count", Cells: map[string]string{"m.one": "el alumno está aprobado"}},
	})
	defer ts.Close()

	entries := []Entry{
		{Label: "count", Cells: map[string]string{"m.one": "o %n aluno está aprovado"}},
	}
	resp, err := newTestOpenAI(ts.URL).Translate(context.Background(), "pt-BR", "es-AR", entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, failed := resp.Failed["count:m.one"]; !failed {
		t.Errorf("expected count:m.one in Failed, got %v", resp.Failed)
	}
	if got := resp.Entries[0].Cells["m.one"]; got != "" {
		t.Errorf("failed cell should be empty, got %q", got)
	}
}

func TestOpenAI_LabelNotReturned_AllCellsFailed(t *testing.T) {
	ts := mockOAIServer(t, []oaiBatchEntry{
		// Model returned a different label.
		{Label: "other", Cells: map[string]string{"": "anything"}},
	})
	defer ts.Close()

	entries := []Entry{
		{Label: "missing", Cells: map[string]string{"": "texto"}},
	}
	resp, err := newTestOpenAI(ts.URL).Translate(context.Background(), "pt-BR", "es-AR", entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, failed := resp.Failed["missing:"]; !failed {
		t.Errorf("expected missing: in Failed, got %v", resp.Failed)
	}
}

func TestOpenAI_HTTPError_ReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	entries := []Entry{{Label: "x", Cells: map[string]string{"": "texto"}}}
	_, err := newTestOpenAI(ts.URL).Translate(context.Background(), "pt-BR", "es-AR", entries)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestOpenAI_InvalidJSON_ReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, _ := json.Marshal(oaiChatResp{
			Choices: []struct {
				Message oaiMsg `json:"message"`
			}{{Message: oaiMsg{Role: "assistant", Content: "not json at all"}}},
		})
		w.Write(resp)
	}))
	defer ts.Close()

	entries := []Entry{{Label: "x", Cells: map[string]string{"": "texto"}}}
	_, err := newTestOpenAI(ts.URL).Translate(context.Background(), "pt-BR", "es-AR", entries)
	if err == nil {
		t.Fatal("expected error for invalid model JSON output")
	}
}

func TestOpenAI_SourceTag(t *testing.T) {
	o := NewOpenAI("http://localhost:11434/v1", "gemma4", "", "", time.Second)
	if got := o.SourceTag(); got != "llm:gemma4" {
		t.Errorf("got %q, want %q", got, "llm:gemma4")
	}
}

func TestStripMarkdownFences(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`{"a":1}`, `{"a":1}`},
		{"```json\n{\"a\":1}\n```", `{"a":1}`},
		{"```\n{\"a\":1}\n```", `{"a":1}`},
		{"  ```json\n{}\n```  ", `{}`},
	}
	for _, tc := range cases {
		if got := stripMarkdownFences(tc.in); got != tc.want {
			t.Errorf("stripMarkdownFences(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

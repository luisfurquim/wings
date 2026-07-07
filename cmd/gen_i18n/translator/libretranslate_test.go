package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSubstituteTokens_NoTokens(t *testing.T) {
	sub, tokens := substituteTokens("hello world")
	if sub != "hello world" {
		t.Fatalf("got %q want %q", sub, "hello world")
	}
	if len(tokens) != 0 {
		t.Fatalf("expected no tokens, got %v", tokens)
	}
}

func TestSubstituteTokens_TemplateRef(t *testing.T) {
	sub, tokens := substituteTokens("click {{%count}} items")
	if sub != "click WPTK_0 items" {
		t.Fatalf("got %q", sub)
	}
	if len(tokens) != 1 || tokens[0] != "{{%count}}" {
		t.Fatalf("tokens = %v", tokens)
	}
}

func TestSubstituteTokens_PctWord(t *testing.T) {
	sub, tokens := substituteTokens("you have %n items")
	if sub != "you have WPTK_0 items" {
		t.Fatalf("got %q", sub)
	}
	if len(tokens) != 1 || tokens[0] != "%n" {
		t.Fatalf("tokens = %v", tokens)
	}
}

func TestSubstituteTokens_Multiple(t *testing.T) {
	sub, tokens := substituteTokens("{{@gender %qt #0}} and %n more")
	if sub != "WPTK_0 and WPTK_1 more" {
		t.Fatalf("got %q", sub)
	}
	if len(tokens) != 2 || tokens[0] != "{{@gender %qt #0}}" || tokens[1] != "%n" {
		t.Fatalf("tokens = %v", tokens)
	}
}

func TestRestoreTokens_RoundTrip(t *testing.T) {
	original := "{{@gender %qt #0}} and %n more"
	sub, tokens := substituteTokens(original)
	restored := restoreTokens(sub, tokens)
	if restored != original {
		t.Fatalf("got %q want %q", restored, original)
	}
}

func TestRestoreTokens_OutOfRange(t *testing.T) {
	// Marker index beyond token list — leave as-is.
	result := restoreTokens("WPTK_5 text", []string{"tok0"})
	if result != "WPTK_5 text" {
		t.Fatalf("got %q", result)
	}
}

func TestLangToISO(t *testing.T) {
	cases := [][2]string{
		{"pt-BR", "pt"},
		{"en-US", "en"},
		{"es-AR", "es"},
		{"fr", "fr"},
		{"zh-Hant-TW", "zh"},
	}
	for _, c := range cases {
		if got := langToISO(c[0]); got != c[1] {
			t.Errorf("langToISO(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestLibreTranslate_SourceTag(t *testing.T) {
	lt := NewLibreTranslate("http://localhost:5000", "", time.Second)
	if lt.SourceTag() != "libretranslate" {
		t.Fatalf("got %q", lt.SourceTag())
	}
}

func TestLibreTranslate_SimpleText_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ltRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		// echo each text uppercased to simulate translation
		out := make([]string, len(req.Q))
		for i, q := range req.Q {
			out[i] = "TRANSLATED:" + q
		}
		_ = json.NewEncoder(w).Encode(ltResponse{TranslatedText: out})
	}))
	defer srv.Close()

	lt := NewLibreTranslate(srv.URL, "", 5*time.Second)
	entries := []Entry{
		{Label: "greeting", Cells: map[string]string{"": "Hello world"}},
	}
	resp, err := lt.Translate(context.Background(), "en", "pt", entries)
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Entries[0].Cells[""]; got != "TRANSLATED:Hello world" {
		t.Fatalf("got %q", got)
	}
	if len(resp.Failed) != 0 {
		t.Fatalf("unexpected failures: %v", resp.Failed)
	}
}

func TestLibreTranslate_TokensRestored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ltRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		// simulate: MT translated text but left WPTK markers intact
		out := make([]string, len(req.Q))
		for i, q := range req.Q {
			out[i] = "Você tem WPTK_0 itens"
			_ = q
		}
		_ = json.NewEncoder(w).Encode(ltResponse{TranslatedText: out})
	}))
	defer srv.Close()

	lt := NewLibreTranslate(srv.URL, "", 5*time.Second)
	entries := []Entry{
		{Label: "count", Cells: map[string]string{"": "You have {{%count}} items"}},
	}
	resp, err := lt.Translate(context.Background(), "en", "pt", entries)
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Entries[0].Cells[""]; got != "Você tem {{%count}} itens" {
		t.Fatalf("got %q", got)
	}
}

func TestLibreTranslate_TokenDropped_RecordedInFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ltRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		// MT dropped the WPTK marker entirely
		out := make([]string, len(req.Q))
		for i := range req.Q {
			out[i] = "Você tem itens"
		}
		_ = json.NewEncoder(w).Encode(ltResponse{TranslatedText: out})
	}))
	defer srv.Close()

	lt := NewLibreTranslate(srv.URL, "", 5*time.Second)
	entries := []Entry{
		{Label: "count", Cells: map[string]string{"": "You have {{%count}} items"}},
	}
	resp, err := lt.Translate(context.Background(), "en", "pt", entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Failed) == 0 {
		t.Fatal("expected failure due to dropped placeholder")
	}
}

func TestLibreTranslate_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	lt := NewLibreTranslate(srv.URL, "", 5*time.Second)
	entries := []Entry{{Label: "x", Cells: map[string]string{"": "hello"}}}
	_, err := lt.Translate(context.Background(), "en", "pt", entries)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLibreTranslate_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ltResponse{Error: "language not supported"})
	}))
	defer srv.Close()

	lt := NewLibreTranslate(srv.URL, "", 5*time.Second)
	entries := []Entry{{Label: "x", Cells: map[string]string{"": "hello"}}}
	_, err := lt.Translate(context.Background(), "en", "xx", entries)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLibreTranslate_EmptyEntries(t *testing.T) {
	lt := NewLibreTranslate("http://localhost:5000", "", 5*time.Second)
	resp, err := lt.Translate(context.Background(), "en", "pt", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Entries) != 0 {
		t.Fatalf("expected empty entries")
	}
}

func TestLibreTranslate_APIKeyForwarded(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ltRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotKey = req.APIKey
		_ = json.NewEncoder(w).Encode(ltResponse{TranslatedText: []string{"hola"}})
	}))
	defer srv.Close()

	lt := NewLibreTranslate(srv.URL, "secret123", 5*time.Second)
	_, _ = lt.Translate(context.Background(), "en", "es", []Entry{{Label: "x", Cells: map[string]string{"": "hello"}}})
	if gotKey != "secret123" {
		t.Fatalf("api_key not forwarded, got %q", gotKey)
	}
}

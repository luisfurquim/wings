package wi18n

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestCatalogSignatureError: the typed error is reachable through a wrap chain
// via errors.As (how @error handlers branch on it), unwraps to its cause, and
// names the offending catalog in its message.
func TestCatalogSignatureError(t *testing.T) {
	cause := errors.New("signature verification failed")
	var err error = fmt.Errorf("loading bundle: %w",
		&CatalogSignatureError{URL: "i18n/pt-BR.inflections.json", Err: cause})

	var sigErr *CatalogSignatureError
	if !errors.As(err, &sigErr) {
		t.Fatal("errors.As did not find *CatalogSignatureError in the chain")
	}
	if sigErr.URL != "i18n/pt-BR.inflections.json" {
		t.Errorf("URL = %q, want i18n/pt-BR.inflections.json", sigErr.URL)
	}
	if !errors.Is(err, cause) {
		t.Error("Unwrap chain does not reach the underlying cause")
	}
	if !strings.Contains(sigErr.Error(), "pt-BR.inflections.json") {
		t.Errorf("message %q does not name the catalog", sigErr.Error())
	}
}

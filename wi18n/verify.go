package wi18n

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	catalogPubKey   ed25519.PublicKey
	catalogPubKeyMu sync.RWMutex
)

// SetCatalogPublicKey configures the ed25519 public key used to verify catalog
// signatures. pemBytes is the PEM block produced by gen_i18n -genkey (type
// "ED25519 PUBLIC KEY"). Call this from main() before wings.Main(), with the
// key embedded via //go:embed gen_i18n.ed25519.pub.
//
// If no public key is configured, catalog verification is skipped, no .sig
// sidecars are fetched, and all catalogs are accepted (backward-compatible
// behaviour for apps without signing). Once a key IS configured, verification
// becomes mandatory: every loaded catalog must carry a valid .sig — a missing
// sidecar (404) is treated as tampering and the catalog is rejected.
func SetCatalogPublicKey(pemBytes []byte) error {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "ED25519 PUBLIC KEY" {
		return errors.New("wi18n: invalid or missing ED25519 PUBLIC KEY PEM block")
	}
	if len(block.Bytes) != ed25519.PublicKeySize {
		return fmt.Errorf("wi18n: wrong public key size: got %d bytes, want %d",
			len(block.Bytes), ed25519.PublicKeySize)
	}
	catalogPubKeyMu.Lock()
	catalogPubKey = ed25519.PublicKey(block.Bytes)
	catalogPubKeyMu.Unlock()
	return nil
}

// signaturesRequired reports whether catalog signature verification is enabled,
// i.e. a public key was configured via SetCatalogPublicKey. When it returns
// false, callers MUST NOT fetch .sig sidecars at all (no signing in use). When
// true, every loaded catalog must carry a valid .sig or be rejected.
//
// Its only caller (setlang.go) carries //go:build js, so a native-only lint
// pass reports this as unused — a false positive of running one build target
// at a time, not dead code. Cross-check against GOOS=js GOARCH=wasm before
// trusting a native-only "unused" finding anywhere in this module.
func signaturesRequired() bool {
	catalogPubKeyMu.RLock()
	defer catalogPubKeyMu.RUnlock()
	return catalogPubKey != nil
}

// verifyCatalog checks the ed25519 signature of jsonBody against the base64
// signature in sigB64. Returns nil when the signature is valid or when no
// public key has been configured (opt-in verification).
func verifyCatalog(jsonBody, sigB64 string) error {
	catalogPubKeyMu.RLock()
	pub := catalogPubKey
	catalogPubKeyMu.RUnlock()

	if pub == nil {
		return nil // verification not configured
	}

	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
	if err != nil {
		return fmt.Errorf("wi18n: catalog signature decode: %w", err)
	}
	if !ed25519.Verify(pub, []byte(jsonBody), sig) {
		return errors.New("wi18n: catalog signature verification failed")
	}
	return nil
}

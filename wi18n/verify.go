//go:build js && wasm

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
// "ED25519 PUBLIC KEY"). Call this from main() before wprana.Main(), with the
// key embedded via //go:embed gen_i18n.ed25519.pub.
//
// If no public key is configured, catalog verification is skipped and all
// catalogs are accepted (backward-compatible behaviour for apps without signing).
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

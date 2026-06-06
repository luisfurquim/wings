package wsign

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
)

// SignCatalog computes the ed25519 signature of jsonContent and writes it
// (base64-encoded) to sigFile alongside the catalog JSON.
func SignCatalog(priv ed25519.PrivateKey, jsonContent []byte, sigFile string) error {
	sig := ed25519.Sign(priv, jsonContent)
	out := base64.StdEncoding.EncodeToString(sig) + "\n"
	return os.WriteFile(sigFile, []byte(out), 0644)
}

package wi18n

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"testing"
)

// FuzzVerifyCatalog asserts the fail-closed property of catalog signature
// verification: with a public key configured, ONLY the exact known-good
// (body, signature) pair may verify. Flipping any byte of either — or feeding
// arbitrary garbage — must yield an error, never acceptance and never a panic.
func FuzzVerifyCatalog(f *testing.F) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		f.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "ED25519 PUBLIC KEY", Bytes: pub})
	if err := SetCatalogPublicKey(pemBytes); err != nil {
		f.Fatal(err)
	}

	goodBody := `{"hello":"world","n":3}`
	goodSig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte(goodBody)))

	// Sanity: the legitimate pair must verify, or the test is meaningless.
	if err := verifyCatalog(goodBody, goodSig); err != nil {
		f.Fatalf("known-good pair failed to verify: %v", err)
	}

	f.Add(goodBody, goodSig)
	f.Add(goodBody, "")
	f.Add("", goodSig)
	f.Add(goodBody+" ", goodSig)
	f.Add(goodBody, goodSig+"=")

	f.Fuzz(func(t *testing.T, body, sig string) {
		err := verifyCatalog(body, sig)
		if err == nil && (body != goodBody || sig != goodSig) {
			t.Fatalf("fail-closed violated: accepted tampered body=%q sig=%q", body, sig)
		}
	})
}

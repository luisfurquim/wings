package wsign

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"

	"golang.org/x/crypto/argon2"
)

// GenerateSigningKey generates a fresh ed25519 keypair and writes:
//   - pubFile: PEM "ED25519 PUBLIC KEY" (raw 32-byte key, no cert needed)
//   - keyFile: PEM "ED25519 PRIVATE KEY" encrypted with Argon2id+AES-256-GCM
func GenerateSigningKey(keyFile, pubFile, password string) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("ed25519 key generation: %w", err)
	}

	// ── Save public key ──────────────────────────────────────────────────────
	pubBlock := &pem.Block{
		Type:  "ED25519 PUBLIC KEY",
		Bytes: pub,
	}
	if err := os.WriteFile(pubFile, pem.EncodeToMemory(pubBlock), 0644); err != nil {
		return fmt.Errorf("writing public key %s: %w", pubFile, err)
	}

	// ── Encrypt private key seed with Argon2id + AES-256-GCM ────────────────
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generating salt: %w", err)
	}
	dk := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	block, err := aes.NewCipher(dk)
	if err != nil {
		return fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generating nonce: %w", err)
	}

	// Encrypt the 64-byte private key seed.
	ciphertext := gcm.Seal(nonce, nonce, priv.Seed(), nil)

	// PEM headers carry the Argon2id parameters so future decryption is
	// self-describing (no magic constants in the decoder).
	headers := map[string]string{
		"Salt":    base64.StdEncoding.EncodeToString(salt),
		"ArgTime": fmt.Sprintf("%d", argonTime),
		"ArgMem":  fmt.Sprintf("%d", argonMemory),
		"ArgThr":  fmt.Sprintf("%d", argonThreads),
	}
	privBlock := &pem.Block{
		Type:    "ED25519 PRIVATE KEY",
		Headers: headers,
		Bytes:   ciphertext,
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(privBlock), 0600); err != nil {
		return fmt.Errorf("writing private key %s: %w", keyFile, err)
	}

	return nil
}

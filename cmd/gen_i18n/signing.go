package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/argon2"
)

const (
	defaultKeyFile = "gen_i18n.ed25519.key"
	defaultPubFile = "gen_i18n.ed25519.pub"

	// Argon2id parameters for private key encryption.
	// Time=3, Memory=64MiB, Threads=4 per OWASP recommendation.
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 32
	nonceLen     = 12
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

// LoadSigningKey reads and decrypts the private key from keyFile using password.
func LoadSigningKey(keyFile, password string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("reading key file %s: %w", keyFile, err)
	}
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "ED25519 PRIVATE KEY" {
		return nil, errors.New("invalid or missing ED25519 PRIVATE KEY PEM block")
	}

	saltB64, ok := block.Headers["Salt"]
	if !ok {
		return nil, errors.New("key file missing Salt header")
	}
	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return nil, fmt.Errorf("decoding salt: %w", err)
	}

	t := parsePEMUint32(block.Headers["ArgTime"], argonTime)
	m := parsePEMUint32(block.Headers["ArgMem"], argonMemory)
	th := parsePEMUint8(block.Headers["ArgThr"], argonThreads)

	dk := argon2.IDKey([]byte(password), salt, t, m, th, argonKeyLen)

	ciph, err := aes.NewCipher(dk)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(ciph)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}
	if len(block.Bytes) < nonceLen {
		return nil, errors.New("ciphertext too short")
	}
	nonce := block.Bytes[:nonceLen]
	seed, err := gcm.Open(nil, nonce, block.Bytes[nonceLen:], nil)
	if err != nil {
		return nil, errors.New("decryption failed — wrong password or corrupted key")
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("unexpected seed size %d", len(seed))
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// SignCatalog computes the ed25519 signature of jsonContent and writes it
// (base64-encoded) to sigFile alongside the catalog JSON.
func SignCatalog(priv ed25519.PrivateKey, jsonContent []byte, sigFile string) error {
	sig := ed25519.Sign(priv, jsonContent)
	out := base64.StdEncoding.EncodeToString(sig) + "\n"
	return os.WriteFile(sigFile, []byte(out), 0644)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func parsePEMUint32(s string, def uint32) uint32 {
	var v uint64
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil || v == 0 {
		return def
	}
	_ = binary.Size(uint32(0))
	return uint32(v)
}

func parsePEMUint8(s string, def uint8) uint8 {
	var v uint64
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil || v == 0 {
		return def
	}
	return uint8(v)
}

package wsign

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/argon2"
)

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

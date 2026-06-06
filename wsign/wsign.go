// Package wsign is the server-side counterpart to wi18n's catalog signature
// verification: it generates ed25519 signing keypairs, decrypts the private
// key, and signs catalog JSON. Both cmd/gen_i18n (build-time signing) and the
// wlate dev server (sign-on-save) share this one implementation so the crypto
// is never duplicated.
//
// The browser-side verification lives in wi18n (verify.go, js/wasm): it loads
// the public key and rejects any catalog whose .sig does not match. wsign
// produces exactly the signatures wi18n verifies — ed25519 over the catalog
// JSON bytes, base64-encoded into a <lang>.json.sig sidecar.
package wsign

const (
	// DefaultKeyFile and DefaultPubFile are the conventional file names for a
	// generated keypair (encrypted private key + PEM public key).
	DefaultKeyFile = "gen_i18n.ed25519.key"
	DefaultPubFile = "gen_i18n.ed25519.pub"

	// Argon2id parameters for private key encryption.
	// Time=3, Memory=64MiB, Threads=4 per OWASP recommendation.
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 32
	nonceLen     = 12
)

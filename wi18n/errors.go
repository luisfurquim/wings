package wi18n

import "fmt"

// CatalogSignatureError reports a catalog rejected by the signature policy:
// with a public key configured (SetCatalogPublicKey), a catalog that loaded
// successfully had no reachable .sig sidecar, or the signature did not verify.
// Handlers can branch on it with errors.As to tell tampering apart from
// ordinary load failures (missing catalog, parse error):
//
//	var sigErr *wi18n.CatalogSignatureError
//	if errors.As(err, &sigErr) { ... }
type CatalogSignatureError struct {
	URL string // the catalog file whose signature policy failed
	Err error  // underlying cause: .sig fetch failure or verification failure
}

func (e *CatalogSignatureError) Error() string {
	return fmt.Sprintf("wi18n: catalog %s rejected: %v", e.URL, e.Err)
}

func (e *CatalogSignatureError) Unwrap() error { return e.Err }

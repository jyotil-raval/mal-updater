package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// PKCEPair holds the verifier and challenge needed for the OAuth2 PKCE flow
type PKCEPair struct {
	Verifier  string
	Challenge string
}

// GeneratePKCE creates a cryptographically random verifier.
// MAL uses plain PKCE — challenge equals verifier directly.
func GeneratePKCE() (PKCEPair, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return PKCEPair{}, fmt.Errorf("generating verifier: %w", err)
	}

	verifier := base64.RawURLEncoding.EncodeToString(b)

	return PKCEPair{
		Verifier:  verifier,
		Challenge: verifier,
	}, nil
}
